package classroom

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"time"

	"mathstudy/backend-go/internal/domain/user"
)

var (
	// ErrForbidden is returned when the current principal cannot perform an action.
	ErrForbidden = errors.New("classroom forbidden")
	// ErrNotFound is returned when a class or enrollment cannot be found.
	ErrNotFound = errors.New("classroom not found")
	// ErrConflict is returned when the requested state already conflicts with existing data.
	ErrConflict = errors.New("classroom conflict")
)

// Repository is the persistence surface required by class management use cases.
type Repository interface {
	GetUser(context.Context, string) (UserRef, bool, error)
	CreateClass(context.Context, ClassCreate, time.Time) (ClassInfo, error)
	ListTeacherClasses(context.Context, string) ([]ClassInfo, error)
	GetTeacherClassDetail(context.Context, string, string) (ClassInfo, []StudentItem, bool, error)
	RemoveStudent(context.Context, string, string, string) (bool, error)
	DisbandClass(context.Context, string, string) (bool, error)
	LookupClassByCode(context.Context, string) (ClassInfo, *UserRef, bool, error)
	StudentHasEnrollment(context.Context, string) (bool, error)
	CreateEnrollment(context.Context, string, string, time.Time) error
	LeaveClass(context.Context, string) (bool, error)
	GetStudentClass(context.Context, string) (ClassInfo, bool, error)
}

// UserRef contains the user fields class management needs.
type UserRef struct {
	ID          string
	Username    string
	Email       string
	DisplayName *string
	AvatarURL   *string
	Role        user.Role
}

// ClassInfo is the Python-compatible class response shape.
type ClassInfo struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Code             string     `json:"code"`
	TeacherID        string     `json:"teacher_id"`
	Description      *string    `json:"description"`
	CreatedAt        time.Time  `json:"created_at"`
	TeacherName      *string    `json:"teacher_name"`
	TeacherEmail     *string    `json:"teacher_email"`
	TeacherAvatarURL *string    `json:"teacher_avatar_url"`
	StudentCount     *int       `json:"student_count"`
	JoinedAt         *time.Time `json:"joined_at"`
}

// StudentItem is the Python-compatible class student response shape.
type StudentItem struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	Email       string  `json:"email"`
	DisplayName *string `json:"display_name"`
}

// ClassCreate stores fields required to persist a class.
type ClassCreate struct {
	ID          string
	Name        string
	Code        string
	TeacherID   string
	Description *string
}

// ClassCreateResponse is returned after creating a class.
type ClassCreateResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	ClassInfo ClassInfo `json:"class_info"`
}

// ClassListResponse wraps teacher class list items.
type ClassListResponse struct {
	Items []ClassInfo `json:"items"`
}

// ClassDetailResponse contains class info and its students.
type ClassDetailResponse struct {
	ClassInfo ClassInfo     `json:"class_info"`
	Students  []StudentItem `json:"students"`
}

// ClassLookupResponse is returned when looking up a class by code.
type ClassLookupResponse struct {
	Found       bool       `json:"found"`
	ClassInfo   *ClassInfo `json:"class_info"`
	TeacherName *string    `json:"teacher_name"`
}

// JoinClassResponse is returned after a student joins a class.
type JoinClassResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	ClassInfo ClassInfo `json:"class_info"`
}

// ActionResponse is used for simple class actions.
type ActionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// StudentClassResponse returns the current student's class if any.
type StudentClassResponse struct {
	ClassInfo *ClassInfo `json:"class_info"`
}

// Service implements class management use cases.
type Service struct {
	repo        Repository
	now         func() time.Time
	codeFactory func() (string, error)
}

// NewService creates a class management service.
func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, errors.New("classroom repository is nil")
	}
	return &Service{
		repo:        repo,
		now:         func() time.Time { return time.Now().UTC() },
		codeFactory: generateClassCode,
	}, nil
}

// CreateClass creates a teacher-owned class with a unique class code.
func (s *Service) CreateClass(ctx context.Context, teacherID string, name string, description *string) (ClassCreateResponse, error) {
	teacher, ok, err := s.repo.GetUser(ctx, teacherID)
	if err != nil {
		return ClassCreateResponse{}, err
	}
	if !ok || (teacher.Role != user.RoleTeacher && teacher.Role != user.RoleAdmin) {
		return ClassCreateResponse{}, ErrForbidden
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return ClassCreateResponse{}, ErrConflict
	}
	if description != nil {
		trimmed := strings.TrimSpace(*description)
		description = &trimmed
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		code, err := s.codeFactory()
		if err != nil {
			return ClassCreateResponse{}, err
		}
		classInfo, err := s.repo.CreateClass(ctx, ClassCreate{
			Name:        name,
			Code:        strings.ToUpper(strings.TrimSpace(code)),
			TeacherID:   teacherID,
			Description: description,
		}, s.now())
		if err == nil {
			return ClassCreateResponse{Success: true, Message: "班级创建成功", ClassInfo: classInfo}, nil
		}
		if !errors.Is(err, ErrConflict) {
			return ClassCreateResponse{}, err
		}
		lastErr = err
	}
	if lastErr != nil {
		return ClassCreateResponse{}, lastErr
	}
	return ClassCreateResponse{}, ErrConflict
}

// ListTeacherClasses returns classes created by one teacher.
func (s *Service) ListTeacherClasses(ctx context.Context, teacherID string) (ClassListResponse, error) {
	items, err := s.repo.ListTeacherClasses(ctx, teacherID)
	if err != nil {
		return ClassListResponse{}, err
	}
	return ClassListResponse{Items: items}, nil
}

// GetTeacherClassDetail returns an owned class and its students.
func (s *Service) GetTeacherClassDetail(ctx context.Context, teacherID string, classID string) (ClassDetailResponse, error) {
	classInfo, students, ok, err := s.repo.GetTeacherClassDetail(ctx, teacherID, classID)
	if err != nil {
		return ClassDetailResponse{}, err
	}
	if !ok {
		return ClassDetailResponse{}, ErrNotFound
	}
	count := len(students)
	classInfo.StudentCount = &count
	return ClassDetailResponse{ClassInfo: classInfo, Students: students}, nil
}

// RemoveStudent removes a student from a teacher-owned class.
func (s *Service) RemoveStudent(ctx context.Context, teacherID string, classID string, studentID string) (ActionResponse, error) {
	removed, err := s.repo.RemoveStudent(ctx, teacherID, classID, studentID)
	if err != nil {
		return ActionResponse{}, err
	}
	if !removed {
		return ActionResponse{}, ErrNotFound
	}
	return ActionResponse{Success: true, Message: "学生已移除"}, nil
}

// DisbandClass deletes a teacher-owned class and its enrollments.
func (s *Service) DisbandClass(ctx context.Context, teacherID string, classID string) (ActionResponse, error) {
	disbanded, err := s.repo.DisbandClass(ctx, teacherID, classID)
	if err != nil {
		return ActionResponse{}, err
	}
	if !disbanded {
		return ActionResponse{}, ErrNotFound
	}
	return ActionResponse{Success: true, Message: "班级已解散"}, nil
}

// LookupClass returns a class by a public class code.
func (s *Service) LookupClass(ctx context.Context, code string) (ClassLookupResponse, error) {
	classInfo, teacher, ok, err := s.repo.LookupClassByCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return ClassLookupResponse{}, err
	}
	if !ok {
		return ClassLookupResponse{Found: false, ClassInfo: nil, TeacherName: nil}, nil
	}
	return ClassLookupResponse{
		Found:       true,
		ClassInfo:   &classInfo,
		TeacherName: displayName(teacher),
	}, nil
}

// JoinClass enrolls a student in a class by code.
func (s *Service) JoinClass(ctx context.Context, studentID string, code string) (JoinClassResponse, error) {
	student, ok, err := s.repo.GetUser(ctx, studentID)
	if err != nil {
		return JoinClassResponse{}, err
	}
	if !ok || student.Role != user.RoleStudent {
		return JoinClassResponse{}, ErrForbidden
	}

	enrolled, err := s.repo.StudentHasEnrollment(ctx, studentID)
	if err != nil {
		return JoinClassResponse{}, err
	}
	if enrolled {
		return JoinClassResponse{}, ErrConflict
	}

	classInfo, _, found, err := s.repo.LookupClassByCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return JoinClassResponse{}, err
	}
	if !found {
		return JoinClassResponse{}, ErrNotFound
	}
	if err := s.repo.CreateEnrollment(ctx, classInfo.ID, studentID, s.now()); err != nil {
		if errors.Is(err, ErrConflict) {
			return JoinClassResponse{}, ErrConflict
		}
		return JoinClassResponse{}, err
	}
	return JoinClassResponse{Success: true, Message: "已加入班级", ClassInfo: classInfo}, nil
}

// LeaveClass removes the student's current class enrollment.
func (s *Service) LeaveClass(ctx context.Context, studentID string) (ActionResponse, error) {
	left, err := s.repo.LeaveClass(ctx, studentID)
	if err != nil {
		return ActionResponse{}, err
	}
	if !left {
		return ActionResponse{}, ErrNotFound
	}
	return ActionResponse{Success: true, Message: "已退出班级"}, nil
}

// GetStudentClass returns the student's current class.
func (s *Service) GetStudentClass(ctx context.Context, studentID string) (StudentClassResponse, error) {
	classInfo, ok, err := s.repo.GetStudentClass(ctx, studentID)
	if err != nil {
		return StudentClassResponse{}, err
	}
	if !ok {
		return StudentClassResponse{ClassInfo: nil}, nil
	}
	return StudentClassResponse{ClassInfo: &classInfo}, nil
}

func displayName(userRef *UserRef) *string {
	if userRef == nil {
		return nil
	}
	if userRef.DisplayName != nil && strings.TrimSpace(*userRef.DisplayName) != "" {
		return userRef.DisplayName
	}
	if userRef.Username == "" {
		return nil
	}
	return &userRef.Username
}

func generateClassCode() (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var builder strings.Builder
	builder.Grow(6)
	for i := 0; i < 6; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		builder.WriteByte(alphabet[n.Int64()])
	}
	return builder.String(), nil
}
