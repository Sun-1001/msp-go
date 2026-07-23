package qathread

import (
	"context"
	"errors"
	"time"

	"mathstudy/backend-go/internal/domain/user"
)

var (
	ErrForbidden     = errors.New("qathread forbidden")
	ErrNotFound      = errors.New("qathread not found")
	ErrInvalidStatus = errors.New("qathread invalid status")
)

// Repository is the persistence surface required by Q&A thread use cases.
type Repository interface {
	ListThreads(ctx context.Context, userID string, role user.Role, search string, status string, className string, teacherID string, page, pageSize int) ([]any, int, error)
	GetThread(ctx context.Context, threadID string, userID string, role user.Role, page, pageSize int) (any, bool, error)
	CreateThread(ctx context.Context, studentID string, teacherID string, content string, source string, now time.Time) (ThreadDetail, error)
	CreateThreadMessage(ctx context.Context, threadID string, senderID string, senderRole string, text string, now time.Time) (Message, error)
	UpdateThreadStatus(ctx context.Context, threadID string, teacherID string, status string) (bool, error)
	DeleteThread(ctx context.Context, threadID string, studentID string) (bool, error)
}

// Message is a single message in a thread.
type Message struct {
	ID   string    `json:"id"`
	From string    `json:"from"`
	Text string    `json:"text"`
	Time time.Time `json:"time"`
}

// StudentThreadItem is the student view of a question thread.
type StudentThreadItem struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	TeacherID   string    `json:"teacher_id"`
	TeacherName string    `json:"teacher_name"`
	Source      string    `json:"source"`
	Context     string    `json:"context"`
	Status      string    `json:"status"`
	LastUpdate  time.Time `json:"last_update"`
}

// TeacherThreadItem is the teacher view of a question thread.
type TeacherThreadItem struct {
	ID             string    `json:"id"`
	StudentName    string    `json:"student_name"`
	ClassName      string    `json:"class_name"`
	Title          string    `json:"title"`
	Source         string    `json:"source"`
	KnowledgePoint string    `json:"knowledge_point"`
	ResourceName   string    `json:"resource_name"`
	Status         string    `json:"status"`
	Context        string    `json:"context"`
	LastUpdate     time.Time `json:"last_update"`
}

// ThreadDetail is a thread with full messages.
type ThreadDetail struct {
	ID             string    `json:"id"`
	StudentName    string    `json:"student_name,omitempty"`
	TeacherName    string    `json:"teacher_name,omitempty"`
	ClassName      string    `json:"class_name,omitempty"`
	Title          string    `json:"title"`
	TeacherID      string    `json:"teacher_id,omitempty"`
	Source         string    `json:"source"`
	KnowledgePoint string    `json:"knowledge_point,omitempty"`
	ResourceName   string    `json:"resource_name,omitempty"`
	Status         string    `json:"status"`
	Context        string    `json:"context"`
	Messages       []Message `json:"messages"`
	MessagesTotal  int       `json:"messages_total"`
	MessagesPage   int       `json:"messages_page"`
	MessagesSize   int       `json:"messages_page_size"`
}

// ListResponse is the paginated list response.
type ListResponse struct {
	Items    []any `json:"items"`
	Total    int   `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// Service implements Q&A thread business logic.
type Service struct {
	repo Repository
}

// NewService creates a Q&A thread service.
func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, errors.New("qathread repository is nil")
	}
	return &Service{repo: repo}, nil
}

// ListThreads returns paginated threads for the user.
func (s *Service) ListThreads(ctx context.Context, userID string, role user.Role, search string, status string, className string, teacherID string, page int, pageSize int) (ListResponse, error) {
	items, total, err := s.repo.ListThreads(ctx, userID, role, search, status, className, teacherID, page, pageSize)
	if err != nil {
		return ListResponse{}, err
	}
	return ListResponse{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// GetThread returns a single thread with messages.
func (s *Service) GetThread(ctx context.Context, userID string, threadID string, role user.Role, page, pageSize int) (any, error) {
	item, found, err := s.repo.GetThread(ctx, threadID, userID, role, page, pageSize)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrNotFound
	}
	return item, nil
}

// CreateThread creates a new question thread.
func (s *Service) CreateThread(ctx context.Context, studentID string, teacherID string, content string, source string) (ThreadDetail, error) {
	return s.repo.CreateThread(ctx, studentID, teacherID, content, source, time.Now())
}

// CreateThreadMessage adds a message to a thread.
func (s *Service) CreateThreadMessage(ctx context.Context, threadID string, senderID string, senderRole string, text string) (Message, error) {
	return s.repo.CreateThreadMessage(ctx, threadID, senderID, senderRole, text, time.Now())
}

// UpdateThreadStatus updates a thread's status.
func (s *Service) UpdateThreadStatus(ctx context.Context, threadID string, teacherID string, status string) error {
	switch status {
	case "待回复", "已回复", "已解决", "需跟进":
	default:
		return ErrInvalidStatus
	}
	ok, err := s.repo.UpdateThreadStatus(ctx, threadID, teacherID, status)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// DeleteThread deletes a thread (student only).
func (s *Service) DeleteThread(ctx context.Context, threadID string, studentID string) error {
	ok, err := s.repo.DeleteThread(ctx, threadID, studentID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}
