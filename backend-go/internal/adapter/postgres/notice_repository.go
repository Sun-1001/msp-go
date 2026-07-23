package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	noticeapp "mathstudy/backend-go/internal/application/notice"
	"mathstudy/backend-go/internal/domain/user"
)

// NoticeRepository persists notice data in PostgreSQL.
type NoticeRepository struct {
	Repository
}

// NewNoticeRepository creates a PostgreSQL-backed notice repository.
func NewNoticeRepository(db Querier) (NoticeRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return NoticeRepository{}, err
	}
	return NoticeRepository{Repository: base}, nil
}

// ListNotices returns paginated notices for a user.
func (r NoticeRepository) ListNotices(ctx context.Context, userID string, role user.Role, search string, status string, className string, page int, pageSize int) ([]any, int, error) {
	pgPage, err := NewPage((page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, err
	}

	if role == user.RoleStudent {
		return r.listStudentNotices(ctx, userID, search, status, pgPage)
	}
	return r.listTeacherNotices(ctx, userID, search, status, className, pgPage)
}

func (r NoticeRepository) listStudentNotices(ctx context.Context, studentID string, search string, status string, page Page) ([]any, int, error) {
	args := []any{studentID}
	idx := 2
	where := " WHERE EXISTS (SELECT 1 FROM public.class_enrollments e WHERE e.class_id = n.class_id AND e.student_id = $1)"

	if strings.TrimSpace(search) != "" {
		where += ` AND (n.title ILIKE $` + idxStr(idx) + ` OR n.body ILIKE $` + idxStr(idx) + `)`
		args = append(args, "%"+search+"%")
		idx++
	}
	switch status {
	case "待确认":
		where += ` AND nc.notice_id IS NULL`
	case "已确认":
		where += ` AND nc.notice_id IS NOT NULL`
	}

	studentParamIdx := idx

	var total int
	if err := r.DB().QueryRow(ctx, `
		SELECT COUNT(*)
		FROM public.notices n
		LEFT JOIN public.notice_confirmations nc ON nc.notice_id = n.id AND nc.student_id = $`+idxStr(idx)+`
		`+where, append(args, studentID)...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, studentID, page.Limit, page.Offset)

	rows2, err := r.DB().Query(ctx, `
		SELECT n.id, c.name, n.title, n.body, n.created_at,
			n.attachments,
			nc.notice_id IS NOT NULL AS confirmed
		FROM public.notices n
		JOIN public.classes c ON c.id = n.class_id
		LEFT JOIN public.notice_confirmations nc ON nc.notice_id = n.id AND nc.student_id = $`+idxStr(studentParamIdx)+`
		`+where+`
		ORDER BY n.created_at DESC
		LIMIT $`+idxStr(studentParamIdx+1)+` OFFSET $`+idxStr(studentParamIdx+2),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows2.Close()

	items := make([]any, 0)
	for rows2.Next() {
		var item noticeapp.StudentNoticeItem
		var attachmentsJSON []byte
		if err := rows2.Scan(&item.ID, &item.ClassName, &item.Title, &item.Body, &item.PublishedAt, &attachmentsJSON, &item.Confirmed); err != nil {
			return nil, 0, err
		}
		if len(attachmentsJSON) > 0 {
			_ = json.Unmarshal(attachmentsJSON, &item.Attachments)
		}
		if item.Attachments == nil {
			item.Attachments = []string{}
		}
		items = append(items, item)
	}
	return items, total, rows2.Err()
}

func (r NoticeRepository) listTeacherNotices(ctx context.Context, teacherID string, search string, status string, className string, page Page) ([]any, int, error) {
	where := " WHERE n.teacher_id = $1"
	args := []any{teacherID}
	idx := 2

	if strings.TrimSpace(search) != "" {
		where += ` AND (n.title ILIKE $` + idxStr(idx) + ` OR n.body ILIKE $` + idxStr(idx) + `)`
		args = append(args, "%"+search+"%")
		idx++
	}
	if strings.TrimSpace(className) != "" {
		where += ` AND c.name ILIKE $` + idxStr(idx)
		args = append(args, "%"+className+"%")
		idx++
	}

	var total int
	if err := r.DB().QueryRow(ctx, `SELECT COUNT(*) FROM public.notices n JOIN public.classes c ON c.id = n.class_id`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	countArgs := len(args)
	args = append(args, page.Limit, page.Offset)

	rows, err := r.DB().Query(ctx, `
		SELECT n.id, c.name, n.title, n.body, n.created_at,
			COALESCE(conf.confirmed_count, 0),
			COALESCE(class_counts.total_count, 0)
		FROM public.notices n
		JOIN public.classes c ON c.id = n.class_id
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS confirmed_count
			FROM public.notice_confirmations nc
			WHERE nc.notice_id = n.id
		) conf ON true
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS total_count
			FROM public.class_enrollments e
			WHERE e.class_id = n.class_id
		) class_counts ON true
		`+where+`
		ORDER BY n.created_at DESC
		LIMIT $`+idxStr(countArgs+1)+` OFFSET $`+idxStr(countArgs+2),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]any, 0)
	for rows.Next() {
		var item noticeapp.TeacherNoticeItem
		if err := rows.Scan(&item.ID, &item.ClassName, &item.Title, &item.Body, &item.PublishedAt,
			&item.ConfirmedCount, &item.TotalCount); err != nil {
			return nil, 0, err
		}
		// Load unconfirmed student names
		studentRows, err := r.DB().Query(ctx, `
			SELECT COALESCE(u.display_name, u.username)
			FROM public.users u
			JOIN public.class_enrollments e ON e.student_id = u.id
			WHERE e.class_id = (SELECT class_id FROM public.notices WHERE id = $1) AND u.id NOT IN (
				SELECT nc.student_id FROM public.notice_confirmations nc WHERE nc.notice_id = $1
			)`, item.ID)
		if err != nil {
			return nil, 0, err
		}
		names := make([]string, 0)
		for studentRows.Next() {
			var name string
			if err := studentRows.Scan(&name); err != nil {
				studentRows.Close()
				return nil, 0, err
			}
			names = append(names, name)
		}
		if err := studentRows.Err(); err != nil {
			studentRows.Close()
			return nil, 0, err
		}
		studentRows.Close()
		item.UnconfirmedStudents = names
		items = append(items, item)
	}
	return items, total, rows.Err()
}

// GetNotice returns a single notice.
func (r NoticeRepository) GetNotice(ctx context.Context, noticeID string, userID string, role user.Role) (any, bool, error) {
	if role == user.RoleStudent {
		return r.getStudentNotice(ctx, noticeID, userID)
	}
	return r.getTeacherNotice(ctx, noticeID, userID)
}

func (r NoticeRepository) getStudentNotice(ctx context.Context, noticeID string, studentID string) (any, bool, error) {
	var item noticeapp.StudentNoticeItem
	var attachmentsJSON []byte
	err := r.DB().QueryRow(ctx, `
		SELECT n.id, c.name, n.title, n.body, n.created_at, n.attachments,
			nc.notice_id IS NOT NULL AS confirmed
		FROM public.notices n
		JOIN public.classes c ON c.id = n.class_id
		LEFT JOIN public.notice_confirmations nc ON nc.notice_id = n.id AND nc.student_id = $2
		WHERE n.id = $1 AND EXISTS (SELECT 1 FROM public.class_enrollments e WHERE e.class_id = n.class_id AND e.student_id = $2)`,
		noticeID, studentID,
	).Scan(&item.ID, &item.ClassName, &item.Title, &item.Body, &item.PublishedAt, &attachmentsJSON, &item.Confirmed)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	if len(attachmentsJSON) > 0 {
		_ = json.Unmarshal(attachmentsJSON, &item.Attachments)
	}
	if item.Attachments == nil {
		item.Attachments = []string{}
	}
	return item, true, nil
}

func (r NoticeRepository) getTeacherNotice(ctx context.Context, noticeID string, teacherID string) (any, bool, error) {
	var item noticeapp.TeacherNoticeItem
	err := r.DB().QueryRow(ctx, `
		SELECT n.id, c.name, n.title, n.body, n.created_at,
			COALESCE(conf.confirmed_count, 0),
			COALESCE(class_counts.total_count, 0)
		FROM public.notices n
		JOIN public.classes c ON c.id = n.class_id
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS confirmed_count FROM public.notice_confirmations nc WHERE nc.notice_id = n.id
		) conf ON true
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS total_count FROM public.class_enrollments e
			WHERE e.class_id = n.class_id
		) class_counts ON true
		WHERE n.id = $1 AND n.teacher_id = $2`,
		noticeID, teacherID,
	).Scan(&item.ID, &item.ClassName, &item.Title, &item.Body, &item.PublishedAt, &item.ConfirmedCount, &item.TotalCount)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	// Load unconfirmed names
	studentRows, err := r.DB().Query(ctx, `
		SELECT COALESCE(u.display_name, u.username)
		FROM public.users u
		JOIN public.class_enrollments e ON e.student_id = u.id
		WHERE e.class_id = (SELECT class_id FROM public.notices WHERE id = $1) AND u.id NOT IN (
			SELECT nc.student_id FROM public.notice_confirmations nc WHERE nc.notice_id = $1
		)`, item.ID)
	if err != nil {
		return nil, false, err
	}
	names := make([]string, 0)
	for studentRows.Next() {
		var name string
		if err := studentRows.Scan(&name); err != nil {
			studentRows.Close()
			return nil, false, err
		}
		names = append(names, name)
	}
	if err := studentRows.Err(); err != nil {
		studentRows.Close()
		return nil, false, err
	}
	studentRows.Close()
	item.UnconfirmedStudents = names
	return item, true, nil
}

// CreateNotice publishes a new notice.
func (r NoticeRepository) CreateNotice(ctx context.Context, teacherID string, classID string, title string, body string, now time.Time) (noticeapp.TeacherNoticeItem, error) {
	var className string
	if err := r.DB().QueryRow(ctx, `SELECT name FROM public.classes WHERE id = $1 AND teacher_id = $2`, classID, teacherID).Scan(&className); err != nil {
		if err == pgx.ErrNoRows {
			return noticeapp.TeacherNoticeItem{}, noticeapp.ErrForbidden
		}
		return noticeapp.TeacherNoticeItem{}, err
	}
	id, err := newUUID()
	if err != nil {
		return noticeapp.TeacherNoticeItem{}, err
	}
	tag, err := r.DB().Exec(ctx, `
		INSERT INTO public.notices (id, teacher_id, class_id, title, body, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		id, teacherID, classID, title, body, now,
	)
	if err != nil {
		return noticeapp.TeacherNoticeItem{}, err
	}
	if tag.RowsAffected() == 0 {
		return noticeapp.TeacherNoticeItem{}, noticeapp.ErrForbidden
	}
	return noticeapp.TeacherNoticeItem{
		ID:                  id,
		ClassName:           className,
		Title:               title,
		Body:                body,
		PublishedAt:         now,
		ConfirmedCount:      0,
		TotalCount:          0,
		UnconfirmedStudents: []string{},
	}, nil
}

// ConfirmNotice marks a notice as confirmed by a student.
func (r NoticeRepository) ConfirmNotice(ctx context.Context, noticeID string, studentID string) (bool, error) {
	// Verify the notice exists and targets the student's class
	var exists bool
	if err := r.DB().QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM public.notices n
		JOIN public.class_enrollments e ON e.class_id = n.class_id
			WHERE n.id = $1 AND e.student_id = $2
		)`, noticeID, studentID,
	).Scan(&exists); err != nil || !exists {
		return false, err
	}

	id, err := newUUID()
	if err != nil {
		return false, err
	}
	_, err = r.DB().Exec(ctx, `
		INSERT INTO public.notice_confirmations (id, notice_id, student_id, confirmed_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (notice_id, student_id) DO NOTHING`,
		id, noticeID, studentID,
	)
	if err != nil {
		return false, err
	}
	return true, nil
}

// RemindUnconfirmed returns students who haven't confirmed a notice.
func (r NoticeRepository) RemindUnconfirmed(ctx context.Context, noticeID string, teacherID string) ([]string, bool, error) {
	// Verify the notice belongs to teacher
	var owner string
	err := r.DB().QueryRow(ctx, `SELECT teacher_id FROM public.notices WHERE id = $1`, noticeID).Scan(&owner)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	if owner != teacherID {
		return nil, false, nil
	}

	rows, err := r.DB().Query(ctx, `
		SELECT COALESCE(u.display_name, u.username)
		FROM public.users u
		JOIN public.class_enrollments e ON e.student_id = u.id
		JOIN public.classes c ON c.id = e.class_id
		JOIN public.notices n ON n.class_id = c.id
		WHERE n.id = $1 AND u.id NOT IN (
			SELECT nc.student_id FROM public.notice_confirmations nc WHERE nc.notice_id = $1
		)`, noticeID)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, false, err
		}
		names = append(names, name)
	}
	return names, true, rows.Err()
}
