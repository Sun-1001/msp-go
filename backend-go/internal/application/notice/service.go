package notice

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"mathstudy/backend-go/internal/application/messageattachment"
	"mathstudy/backend-go/internal/domain/user"
)

var (
	ErrForbidden           = errors.New("notice forbidden")
	ErrNotFound            = errors.New("notice not found")
	ErrInvalidInput        = errors.New("notice invalid input")
	ErrReminderUnavailable = errors.New("notice reminder unavailable")
)

const (
	maxIdentifierRunes = 36
	maxSearchRunes     = 200
	maxClassNameRunes  = 200
	maxTitleRunes      = 500
	maxBodyRunes       = 50000
	maxPageNumber      = 10000
)

// Repository is the persistence surface required by notice use cases.
type Repository interface {
	ListNotices(ctx context.Context, userID string, role user.Role, search string, status string, className string, page, pageSize int) ([]any, int, error)
	GetNotice(ctx context.Context, noticeID string, userID string, role user.Role) (any, bool, error)
	CreateNotice(ctx context.Context, teacherID string, classID string, title string, body string, attachments []messageattachment.Attachment, now time.Time) (TeacherNoticeItem, error)
	ConfirmNotice(ctx context.Context, noticeID string, studentID string) (bool, error)
	RemindUnconfirmed(ctx context.Context, noticeID string, teacherID string) (ReminderResult, error)
}

// StudentNoticeListItem is the compact student view returned from notice lists.
type StudentNoticeListItem struct {
	ID          string    `json:"id"`
	ClassName   string    `json:"class_name"`
	Title       string    `json:"title"`
	PublishedAt time.Time `json:"published_at"`
	Confirmed   bool      `json:"confirmed"`
}

// StudentNoticeItem is the student detail view of a notice.
type StudentNoticeItem struct {
	StudentNoticeListItem
	Body        string                         `json:"body"`
	Attachments []messageattachment.Attachment `json:"attachments"`
}

// TeacherNoticeListItem is the compact teacher view returned from notice lists.
type TeacherNoticeListItem struct {
	ID             string    `json:"id"`
	ClassName      string    `json:"class_name"`
	Title          string    `json:"title"`
	PublishedAt    time.Time `json:"published_at"`
	ConfirmedCount int       `json:"confirmed_count"`
	TotalCount     int       `json:"total_count"`
}

// TeacherNoticeItem is the teacher detail view of a notice.
type TeacherNoticeItem struct {
	ID                  string                         `json:"id"`
	ClassName           string                         `json:"class_name"`
	Title               string                         `json:"title"`
	Body                string                         `json:"body"`
	PublishedAt         time.Time                      `json:"published_at"`
	ConfirmedCount      int                            `json:"confirmed_count"`
	TotalCount          int                            `json:"total_count"`
	UnconfirmedStudents []string                       `json:"unconfirmed_students"`
	Attachments         []messageattachment.Attachment `json:"attachments"`
}

// ReminderResult reports the current target set and how many jobs were newly queued.
type ReminderResult struct {
	UnconfirmedStudents []string `json:"unconfirmed_students"`
	Count               int      `json:"count"`
	QueuedCount         int      `json:"queued_count"`
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
	if role != user.RoleStudent && role != user.RoleTeacher {
		return ListResponse{}, ErrForbidden
	}
	search = strings.TrimSpace(search)
	status = strings.TrimSpace(status)
	className = strings.TrimSpace(className)
	if page < 1 || page > maxPageNumber || pageSize < 1 || pageSize > 100 ||
		utf8.RuneCountInString(search) > maxSearchRunes ||
		utf8.RuneCountInString(className) > maxClassNameRunes ||
		containsInvalidText(search, status, className) || !validListFilters(role, status, className) {
		return ListResponse{}, ErrInvalidInput
	}
	items, total, err := s.repo.ListNotices(ctx, userID, role, search, status, className, page, pageSize)
	if err != nil {
		return ListResponse{}, err
	}
	return ListResponse{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// GetNotice returns a single notice.
func (s *Service) GetNotice(ctx context.Context, userID string, noticeID string, role user.Role) (any, error) {
	noticeID = strings.TrimSpace(noticeID)
	if (role != user.RoleStudent && role != user.RoleTeacher) || !validIdentifier(noticeID) {
		return nil, ErrInvalidInput
	}
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
func (s *Service) CreateNotice(ctx context.Context, teacherID string, classID string, title string, body string, attachments []messageattachment.Attachment) (TeacherNoticeItem, error) {
	classID = strings.TrimSpace(classID)
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	normalizedAttachments, err := messageattachment.Normalize(attachments)
	if err != nil || !validIdentifier(classID) || title == "" ||
		utf8.RuneCountInString(title) > maxTitleRunes ||
		utf8.RuneCountInString(body) > maxBodyRunes || containsInvalidText(title, body) {
		return TeacherNoticeItem{}, ErrInvalidInput
	}
	return s.repo.CreateNotice(ctx, teacherID, classID, title, body, normalizedAttachments, time.Now())
}

// ConfirmNotice marks a notice as confirmed by a student.
func (s *Service) ConfirmNotice(ctx context.Context, noticeID string, studentID string) error {
	noticeID = strings.TrimSpace(noticeID)
	if !validIdentifier(noticeID) {
		return ErrInvalidInput
	}
	ok, err := s.repo.ConfirmNotice(ctx, noticeID, studentID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

// RemindUnconfirmed requeues reminders for the teacher's currently unconfirmed recipients.
func (s *Service) RemindUnconfirmed(ctx context.Context, noticeID string, teacherID string) (ReminderResult, error) {
	noticeID = strings.TrimSpace(noticeID)
	if !validIdentifier(noticeID) || !validIdentifier(teacherID) {
		return ReminderResult{}, ErrInvalidInput
	}
	return s.repo.RemindUnconfirmed(ctx, noticeID, teacherID)
}

func validIdentifier(value string) bool {
	return value != "" && utf8.RuneCountInString(value) <= maxIdentifierRunes &&
		utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

func validStatusFilter(role user.Role, status string) bool {
	if status == "" || status == "全部" {
		return true
	}
	if role == user.RoleStudent {
		return status == "待确认" || status == "已确认"
	}
	return status == "有未确认" || status == "全部确认"
}

func validListFilters(role user.Role, status string, className string) bool {
	if !validStatusFilter(role, status) {
		return false
	}
	return role != user.RoleStudent || className == ""
}

func containsInvalidText(values ...string) bool {
	for _, value := range values {
		if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
			return true
		}
	}
	return false
}
