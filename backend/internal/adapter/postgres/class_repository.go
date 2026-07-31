package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	classroomapp "mathstudy/backend/internal/application/classroom"
	"mathstudy/backend/internal/domain/user"
)

// ClassRepository persists class management data in PostgreSQL.
type ClassRepository struct {
	Repository
}

// NewClassRepository creates a PostgreSQL-backed class repository.
func NewClassRepository(db Querier) (ClassRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return ClassRepository{}, err
	}
	return ClassRepository{Repository: base}, nil
}

// GetUser returns one user by ID for class role checks and response decoration.
func (r ClassRepository) GetUser(ctx context.Context, userID string) (classroomapp.UserRef, bool, error) {
	row := r.DB().QueryRow(ctx, `
		SELECT id, username, email, display_name, avatar_url, role::text
		FROM public.users
		WHERE id = $1`,
		userID,
	)
	return scanOptionalClassUser(row)
}

// CreateClass inserts a class and returns the persisted row.
func (r ClassRepository) CreateClass(ctx context.Context, input classroomapp.ClassCreate, now time.Time) (classroomapp.ClassInfo, error) {
	if input.ID == "" {
		id, err := newUUID()
		if err != nil {
			return classroomapp.ClassInfo{}, err
		}
		input.ID = id
	}
	row := r.DB().QueryRow(ctx, `
		INSERT INTO public.classes (
			id,
			name,
			code,
			teacher_id,
			description,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		RETURNING id, name, code, teacher_id, description, created_at`,
		input.ID,
		input.Name,
		input.Code,
		input.TeacherID,
		input.Description,
		now,
	)
	classInfo, err := scanClassBasic(row)
	if err != nil {
		if isUniqueViolation(err) {
			return classroomapp.ClassInfo{}, classroomapp.ErrConflict
		}
		return classroomapp.ClassInfo{}, err
	}
	return classInfo, nil
}

// ListTeacherClasses returns classes created by a teacher with enrollment counts.
func (r ClassRepository) ListTeacherClasses(ctx context.Context, teacherID string) ([]classroomapp.ClassInfo, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT
			c.id,
			c.name,
			c.code,
			c.teacher_id,
			c.description,
			c.created_at,
			count(ce.id)::int
		FROM public.classes c
		LEFT JOIN public.class_enrollments ce ON ce.class_id = c.id
		WHERE c.teacher_id = $1
		GROUP BY c.id
		ORDER BY c.created_at DESC, c.id DESC`,
		teacherID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	classes := []classroomapp.ClassInfo{}
	for rows.Next() {
		var classInfo classroomapp.ClassInfo
		var description pgtype.Text
		var count int
		if err := rows.Scan(
			&classInfo.ID,
			&classInfo.Name,
			&classInfo.Code,
			&classInfo.TeacherID,
			&description,
			&classInfo.CreatedAt,
			&count,
		); err != nil {
			return nil, err
		}
		classInfo.Description = textPtr(description)
		classInfo.StudentCount = &count
		classes = append(classes, classInfo)
	}
	return classes, rows.Err()
}

// GetTeacherClassDetail returns one teacher-owned class and its students.
func (r ClassRepository) GetTeacherClassDetail(ctx context.Context, teacherID string, classID string) (classroomapp.ClassInfo, []classroomapp.StudentItem, bool, error) {
	row := r.DB().QueryRow(ctx, `
		SELECT
			c.id,
			c.name,
			c.code,
			c.teacher_id,
			c.description,
			c.created_at,
			u.username,
			u.email,
			u.display_name,
			u.avatar_url
		FROM public.classes c
		LEFT JOIN public.users u ON u.id = c.teacher_id
		WHERE c.id = $1 AND c.teacher_id = $2`,
		classID,
		teacherID,
	)
	classInfo, ok, err := scanOptionalClassWithTeacher(row)
	if err != nil || !ok {
		return classroomapp.ClassInfo{}, nil, ok, err
	}

	students, err := r.listClassStudents(ctx, classID)
	if err != nil {
		return classroomapp.ClassInfo{}, nil, false, err
	}
	return classInfo, students, true, nil
}

// RemoveStudent removes one student enrollment from a teacher-owned class.
func (r ClassRepository) RemoveStudent(ctx context.Context, teacherID string, classID string, studentID string) (bool, error) {
	removed := false
	err := r.withTx(ctx, func(tx ClassRepository) error {
		var enrollmentID string
		var enrolledClassID string
		var enrolledStudentID string
		var joinedAt time.Time
		err := tx.DB().QueryRow(ctx, `
			DELETE FROM public.class_enrollments enrollment
			USING public.classes class_info
			WHERE enrollment.class_id = class_info.id
				AND class_info.id = $1
				AND class_info.teacher_id = $2
				AND enrollment.student_id = $3
			RETURNING enrollment.id, enrollment.class_id, enrollment.student_id, enrollment.joined_at`,
			classID,
			teacherID,
			studentID,
		).Scan(&enrollmentID, &enrolledClassID, &enrolledStudentID, &joinedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}

		_, err = tx.DB().Exec(ctx, `
			INSERT INTO public.class_enrollment_history (
				enrollment_id, class_id, student_id, joined_at, left_at
			)
			VALUES ($1, $2, $3, $4, clock_timestamp() AT TIME ZONE 'UTC')
			ON CONFLICT (enrollment_id) DO UPDATE SET
				left_at = EXCLUDED.left_at
			WHERE public.class_enrollment_history.left_at IS NULL`,
			enrollmentID,
			enrolledClassID,
			enrolledStudentID,
			joinedAt,
		)
		if err != nil {
			return err
		}
		removed = true
		return nil
	})
	return removed, err
}

// DisbandClass deletes a teacher-owned class and its enrollments.
func (r ClassRepository) DisbandClass(ctx context.Context, teacherID string, classID string) (bool, error) {
	disbanded := false
	err := r.withTx(ctx, func(tx ClassRepository) error {
		if _, err := tx.DB().Exec(ctx, `
			DELETE FROM public.class_enrollments
			WHERE class_id IN (
				SELECT id
				FROM public.classes
				WHERE id = $1 AND teacher_id = $2
			)`,
			classID,
			teacherID,
		); err != nil {
			return err
		}
		tag, err := tx.DB().Exec(ctx, `
			DELETE FROM public.classes
			WHERE id = $1 AND teacher_id = $2`,
			classID,
			teacherID,
		)
		if err != nil {
			return err
		}
		disbanded = tag.RowsAffected() > 0
		return nil
	})
	return disbanded, err
}

// LookupClassByCode returns a class and its teacher by public class code.
func (r ClassRepository) LookupClassByCode(ctx context.Context, code string) (classroomapp.ClassInfo, *classroomapp.UserRef, bool, error) {
	row := r.DB().QueryRow(ctx, `
		SELECT
			c.id,
			c.name,
			c.code,
			c.teacher_id,
			c.description,
			c.created_at,
			u.id,
			u.username,
			u.email,
			u.display_name,
			u.avatar_url,
			u.role::text
		FROM public.classes c
		LEFT JOIN public.users u ON u.id = c.teacher_id
		WHERE c.code = $1`,
		strings.ToUpper(strings.TrimSpace(code)),
	)
	classInfo, teacher, ok, err := scanOptionalClassLookup(row)
	if err != nil || !ok {
		return classroomapp.ClassInfo{}, nil, ok, err
	}
	return classInfo, teacher, true, nil
}

// StudentHasEnrollment reports whether a student is already enrolled in a class.
func (r ClassRepository) StudentHasEnrollment(ctx context.Context, studentID string) (bool, error) {
	return r.Exists(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM public.class_enrollments
			WHERE student_id = $1
		)`,
		studentID,
	)
}

// CreateEnrollment inserts a class enrollment for one student.
func (r ClassRepository) CreateEnrollment(ctx context.Context, classID string, studentID string, joinedAt time.Time) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	err = r.withTx(ctx, func(tx ClassRepository) error {
		if _, err := tx.DB().Exec(ctx, `
			INSERT INTO public.class_enrollments (id, class_id, student_id, joined_at)
			VALUES ($1, $2, $3, $4)`,
			id,
			classID,
			studentID,
			joinedAt,
		); err != nil {
			return err
		}
		_, err := tx.DB().Exec(ctx, `
			INSERT INTO public.class_enrollment_history (
				enrollment_id,
				class_id,
				student_id,
				joined_at
			)
			VALUES ($1, $2, $3, $4)`,
			id,
			classID,
			studentID,
			joinedAt,
		)
		return err
	})
	if err != nil {
		if isUniqueViolation(err) {
			return classroomapp.ErrConflict
		}
		return err
	}
	return nil
}

// LeaveClass deletes the current class enrollment for one student.
func (r ClassRepository) LeaveClass(ctx context.Context, studentID string) (bool, error) {
	left := false
	err := r.withTx(ctx, func(tx ClassRepository) error {
		var enrollmentID string
		var classID string
		var enrolledStudentID string
		var joinedAt time.Time
		err := tx.DB().QueryRow(ctx, `
			DELETE FROM public.class_enrollments
			WHERE student_id = $1
			RETURNING id, class_id, student_id, joined_at`,
			studentID,
		).Scan(&enrollmentID, &classID, &enrolledStudentID, &joinedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}

		_, err = tx.DB().Exec(ctx, `
			INSERT INTO public.class_enrollment_history (
				enrollment_id, class_id, student_id, joined_at, left_at
			)
			VALUES ($1, $2, $3, $4, clock_timestamp() AT TIME ZONE 'UTC')
			ON CONFLICT (enrollment_id) DO UPDATE SET
				left_at = EXCLUDED.left_at
			WHERE public.class_enrollment_history.left_at IS NULL`,
			enrollmentID,
			classID,
			enrolledStudentID,
			joinedAt,
		)
		if err != nil {
			return err
		}
		left = true
		return nil
	})
	return left, err
}

// GetStudentClass returns the student's current class with teacher and enrollment details.
func (r ClassRepository) GetStudentClass(ctx context.Context, studentID string) (classroomapp.ClassInfo, bool, error) {
	row := r.DB().QueryRow(ctx, `
		SELECT
			c.id,
			c.name,
			c.code,
			c.teacher_id,
			c.description,
			c.created_at,
			t.username,
			t.email,
			t.display_name,
			t.avatar_url,
			ce.joined_at,
			(
				SELECT count(id)::int
				FROM public.class_enrollments
				WHERE class_id = c.id
			) AS student_count
		FROM public.class_enrollments ce
		JOIN public.classes c ON c.id = ce.class_id
		LEFT JOIN public.users t ON t.id = c.teacher_id
		WHERE ce.student_id = $1`,
		studentID,
	)
	return scanOptionalStudentClass(row)
}

func (r ClassRepository) listClassStudents(ctx context.Context, classID string) ([]classroomapp.StudentItem, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT u.id, u.username, u.email, u.display_name
		FROM public.users u
		JOIN public.class_enrollments ce ON ce.student_id = u.id
		WHERE ce.class_id = $1
		ORDER BY u.created_at DESC, u.id DESC`,
		classID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	students := []classroomapp.StudentItem{}
	for rows.Next() {
		var student classroomapp.StudentItem
		var displayName pgtype.Text
		if err := rows.Scan(&student.ID, &student.Username, &student.Email, &displayName); err != nil {
			return nil, err
		}
		student.DisplayName = textPtr(displayName)
		students = append(students, student)
	}
	return students, rows.Err()
}

func (r ClassRepository) withTx(ctx context.Context, fn func(ClassRepository) error) error {
	return withRepositoryTx(ctx, "classroom", r.Repository, func(base Repository) ClassRepository {
		return ClassRepository{Repository: base}
	}, fn)
}

func scanOptionalClassUser(row pgx.Row) (classroomapp.UserRef, bool, error) {
	userRef, err := scanClassUser(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return classroomapp.UserRef{}, false, nil
		}
		return classroomapp.UserRef{}, false, err
	}
	return userRef, true, nil
}

func scanClassUser(scanner rowScanner) (classroomapp.UserRef, error) {
	var userRef classroomapp.UserRef
	var displayName pgtype.Text
	var avatarURL pgtype.Text
	var roleValue string
	if err := scanner.Scan(
		&userRef.ID,
		&userRef.Username,
		&userRef.Email,
		&displayName,
		&avatarURL,
		&roleValue,
	); err != nil {
		return classroomapp.UserRef{}, err
	}
	role, err := user.ParseRole(roleValue)
	if err != nil {
		return classroomapp.UserRef{}, err
	}
	userRef.Role = role
	userRef.DisplayName = textPtr(displayName)
	userRef.AvatarURL = textPtr(avatarURL)
	return userRef, nil
}

func scanClassBasic(scanner rowScanner) (classroomapp.ClassInfo, error) {
	var classInfo classroomapp.ClassInfo
	var description pgtype.Text
	if err := scanner.Scan(
		&classInfo.ID,
		&classInfo.Name,
		&classInfo.Code,
		&classInfo.TeacherID,
		&description,
		&classInfo.CreatedAt,
	); err != nil {
		return classroomapp.ClassInfo{}, err
	}
	classInfo.Description = textPtr(description)
	return classInfo, nil
}

func scanOptionalClassWithTeacher(row pgx.Row) (classroomapp.ClassInfo, bool, error) {
	classInfo, err := scanClassWithTeacher(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return classroomapp.ClassInfo{}, false, nil
		}
		return classroomapp.ClassInfo{}, false, err
	}
	return classInfo, true, nil
}

func scanClassWithTeacher(scanner rowScanner) (classroomapp.ClassInfo, error) {
	var classInfo classroomapp.ClassInfo
	var description pgtype.Text
	var username pgtype.Text
	var email pgtype.Text
	var displayName pgtype.Text
	var avatarURL pgtype.Text
	if err := scanner.Scan(
		&classInfo.ID,
		&classInfo.Name,
		&classInfo.Code,
		&classInfo.TeacherID,
		&description,
		&classInfo.CreatedAt,
		&username,
		&email,
		&displayName,
		&avatarURL,
	); err != nil {
		return classroomapp.ClassInfo{}, err
	}
	classInfo.Description = textPtr(description)
	if displayName.Valid && strings.TrimSpace(displayName.String) != "" {
		value := displayName.String
		classInfo.TeacherName = &value
	} else if username.Valid {
		value := username.String
		classInfo.TeacherName = &value
	}
	classInfo.TeacherEmail = textPtr(email)
	classInfo.TeacherAvatarURL = textPtr(avatarURL)
	return classInfo, nil
}

func scanOptionalClassLookup(row pgx.Row) (classroomapp.ClassInfo, *classroomapp.UserRef, bool, error) {
	classInfo, teacher, err := scanClassLookup(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return classroomapp.ClassInfo{}, nil, false, nil
		}
		return classroomapp.ClassInfo{}, nil, false, err
	}
	return classInfo, teacher, true, nil
}

func scanClassLookup(scanner rowScanner) (classroomapp.ClassInfo, *classroomapp.UserRef, error) {
	var classInfo classroomapp.ClassInfo
	var description pgtype.Text
	var teacher classroomapp.UserRef
	var teacherID pgtype.Text
	var username pgtype.Text
	var email pgtype.Text
	var displayName pgtype.Text
	var avatarURL pgtype.Text
	var roleValue pgtype.Text
	if err := scanner.Scan(
		&classInfo.ID,
		&classInfo.Name,
		&classInfo.Code,
		&classInfo.TeacherID,
		&description,
		&classInfo.CreatedAt,
		&teacherID,
		&username,
		&email,
		&displayName,
		&avatarURL,
		&roleValue,
	); err != nil {
		return classroomapp.ClassInfo{}, nil, err
	}
	classInfo.Description = textPtr(description)
	if !teacherID.Valid {
		return classInfo, nil, nil
	}
	teacher.ID = teacherID.String
	if username.Valid {
		teacher.Username = username.String
	}
	if email.Valid {
		teacher.Email = email.String
	}
	teacher.DisplayName = textPtr(displayName)
	teacher.AvatarURL = textPtr(avatarURL)
	if roleValue.Valid {
		role, err := user.ParseRole(roleValue.String)
		if err != nil {
			return classroomapp.ClassInfo{}, nil, err
		}
		teacher.Role = role
	}
	return classInfo, &teacher, nil
}

func scanOptionalStudentClass(row pgx.Row) (classroomapp.ClassInfo, bool, error) {
	classInfo, err := scanStudentClass(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return classroomapp.ClassInfo{}, false, nil
		}
		return classroomapp.ClassInfo{}, false, err
	}
	return classInfo, true, nil
}

func scanStudentClass(scanner rowScanner) (classroomapp.ClassInfo, error) {
	classInfo, err := scanClassWithTeacherAndJoined(scanner)
	if err != nil {
		return classroomapp.ClassInfo{}, err
	}
	return classInfo, nil
}

func scanClassWithTeacherAndJoined(scanner rowScanner) (classroomapp.ClassInfo, error) {
	var classInfo classroomapp.ClassInfo
	var description pgtype.Text
	var username pgtype.Text
	var email pgtype.Text
	var displayName pgtype.Text
	var avatarURL pgtype.Text
	var joinedAt time.Time
	var studentCount int
	if err := scanner.Scan(
		&classInfo.ID,
		&classInfo.Name,
		&classInfo.Code,
		&classInfo.TeacherID,
		&description,
		&classInfo.CreatedAt,
		&username,
		&email,
		&displayName,
		&avatarURL,
		&joinedAt,
		&studentCount,
	); err != nil {
		return classroomapp.ClassInfo{}, err
	}
	classInfo.Description = textPtr(description)
	if displayName.Valid && strings.TrimSpace(displayName.String) != "" {
		value := displayName.String
		classInfo.TeacherName = &value
	} else if username.Valid {
		value := username.String
		classInfo.TeacherName = &value
	}
	classInfo.TeacherEmail = textPtr(email)
	classInfo.TeacherAvatarURL = textPtr(avatarURL)
	classInfo.JoinedAt = &joinedAt
	classInfo.StudentCount = &studentCount
	return classInfo, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
