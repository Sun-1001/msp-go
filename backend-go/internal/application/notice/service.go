package notice

import (
	"context"
	"errors"
	"time"

	"mathstudy/backend-go/internal/domain/user"
)

var (
	ErrForbidden = errors.New("notice forbidden")
	ErrNotFound  = errors.New("notice not found")
)

// Repository is the persistence surface required by notice use cases.
type Repository interface {
	ListNotices(ctx context.Context, userID string, role user.Role, search string, status string, className string, page, pageSize int) ([]any, int, error)
	GetNotice(ctx context.Context, noticeID string, userID string, role user.Role) (any, bool, error)
	CreateNotice(ctx context.Context, teacherID string, classID string, title string, body string, now time.Time) (TeacherNoticeItem, error)
	ConfirmNotice(ctx context.Context, noticeID string, studentID string) (bool, error)
	RemindUnconfirmed(ctx context.Context, noticeID string, teacherID string) ([]string, bool, error)
}

// StudentNoticeItem is the student view of a notice.
type StudentNoticeItem struct {
	ID          string    `json:"id"`
	ClassName   string    `json:"class_name"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"published_at"`
	Confirmed   bool      `json:"confirmed"`
	Attachments []string  `json:"attachments"`
}

// TeacherNoticeItem is the teacher view of a notice.
type TeacherNoticeItem struct {
	ID                  string    `json:"id"`
	ClassName           string    `json:"class_name"`
	Title               string    `json:"title"`
	Body                string    `json:"body"`
	PublishedAt         time.Time `json:"published_at"`
	ConfirmedCount      int       `json:"confirmed_count"`
	TotalCount          int       `json:"total_count"`
	UnconfirmedStudents []string  `json:"unconfirmed_students"`
}

// ListResponse is the paginated list response.
type ListResponse struct {
	Items    []any `json:"items"`
	Total    int   `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// Service implements notice business logic.
type Service struct {
	repo Repository
}

// NewService creates a notice service.
func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, errors.New("notice repository is nil")
	}
	return &Service{repo: repo}, nil
}

// ListNotices returns paginated notices for the user.
func (s *Service) ListNotices(ctx context.Context, userID string, role user.Role, search string, status string, className string, page int, pageSize int) (ListResponse, error) {
	items, total, err := s.repo.ListNotices(ctx, userID, role, search, status, className, page, pageSize)
	if err != nil {
		return ListResponse{}, err
	}
	return ListResponse{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// GetNotice returns a single notice.
func (s *Service) GetNotice(ctx context.Context, userID string, noticeID string, role user.Role) (any, error) {
	item, found, err := s.repo.GetNotice(ctx, noticeID, userID, role)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrNotFound
	}
	return item, nil
}

// CreateNotice publishes a new notice.
func (s *Service) CreateNotice(ctx context.Context, teacherID string, classID string, title string, body string) (TeacherNoticeItem, error) {
	return s.repo.CreateNotice(ctx, teacherID, classID, title, body, time.Now())
}

// ConfirmNotice marks a notice as confirmed by a student.
func (s *Service) ConfirmNotice(ctx context.Context, noticeID string, studentID string) error {
	ok, err := s.repo.ConfirmNotice(ctx, noticeID, studentID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

// RemindUnconfirmed returns the names of students who haven't confirmed.
func (s *Service) RemindUnconfirmed(ctx context.Context, noticeID string, teacherID string) ([]string, error) {
	names, found, err := s.repo.RemindUnconfirmed(ctx, noticeID, teacherID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrNotFound
	}
	return names, nil
}
