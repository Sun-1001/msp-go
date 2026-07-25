package qathread

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"mathstudy/backend-go/internal/domain/user"
)

var (
	ErrForbidden     = errors.New("qathread forbidden")
	ErrNotFound      = errors.New("qathread not found")
	ErrInvalidStatus = errors.New("qathread invalid status")
	ErrInvalidInput  = errors.New("qathread invalid input")
)

const (
	maxIdentifierRunes = 36
	maxSearchRunes     = 200
	maxClassNameRunes  = 200
	maxSourceRunes     = 50
	maxContentRunes    = 50000
	maxMessageRunes    = 10000
	maxPageNumber      = 10000
	maxPageSize        = 100
)

// Repository is the persistence surface required by Q&A thread use cases.
type Repository interface {
	ListThreads(ctx context.Context, userID string, role user.Role, search string, status string, className string, teacherID string, page, pageSize int) ([]any, int, error)
	GetThread(ctx context.Context, threadID string, userID string, role user.Role, page, pageSize int) (any, bool, error)
	AcknowledgeThreadRead(ctx context.Context, threadID string, studentID string, throughMessageID string) (bool, error)
	CreateThread(ctx context.Context, studentID string, teacherID string, content string, source string, now time.Time) (ThreadDetail, error)
	CreateThreadMessage(ctx context.Context, threadID string, senderID string, senderRole string, text string, now time.Time) (Message, error)
	UpdateThreadStatus(ctx context.Context, threadID string, teacherID string, status string) (bool, error)
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
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	TeacherID      string    `json:"teacher_id"`
	TeacherName    string    `json:"teacher_name"`
	Source         string    `json:"source"`
	ContextPreview string    `json:"context_preview"`
	Status         string    `json:"status"`
	ClassID        string    `json:"class_id,omitempty"`
	ClassName      string    `json:"class_name,omitempty"`
	Unread         bool      `json:"unread"`
	LastUpdate     time.Time `json:"last_update"`
}

// TeacherThreadItem is the teacher view of a question thread.
type TeacherThreadItem struct {
	ID             string    `json:"id"`
	StudentName    string    `json:"student_name"`
	ClassID        string    `json:"class_id,omitempty"`
	ClassName      string    `json:"class_name"`
	Title          string    `json:"title"`
	Source         string    `json:"source"`
	KnowledgePoint string    `json:"knowledge_point"`
	ResourceName   string    `json:"resource_name"`
	Status         string    `json:"status"`
	ContextPreview string    `json:"context_preview"`
	LastUpdate     time.Time `json:"last_update"`
}

// ThreadDetail is a thread with full messages.
type ThreadDetail struct {
	ID                   string    `json:"id"`
	StudentName          string    `json:"student_name,omitempty"`
	TeacherName          string    `json:"teacher_name,omitempty"`
	ClassID              string    `json:"class_id,omitempty"`
	ClassName            string    `json:"class_name,omitempty"`
	Title                string    `json:"title"`
	TeacherID            string    `json:"teacher_id,omitempty"`
	Source               string    `json:"source"`
	KnowledgePoint       string    `json:"knowledge_point,omitempty"`
	ResourceName         string    `json:"resource_name,omitempty"`
	Status               string    `json:"status"`
	Context              string    `json:"context"`
	Messages             []Message `json:"messages"`
	MessagesTotal        int       `json:"messages_total"`
	MessagesPage         int       `json:"messages_page"`
	MessagesSize         int       `json:"messages_page_size"`
	ReadThroughMessageID string    `json:"read_through_message_id,omitempty"`
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
	if role != user.RoleStudent && role != user.RoleTeacher {
		return ListResponse{}, ErrForbidden
	}
	search = strings.TrimSpace(search)
	status = strings.TrimSpace(status)
	className = strings.TrimSpace(className)
	teacherID = strings.TrimSpace(teacherID)
	if page < 1 || page > maxPageNumber || pageSize < 1 || pageSize > maxPageSize ||
		utf8.RuneCountInString(search) > maxSearchRunes || utf8.RuneCountInString(className) > maxClassNameRunes ||
		!validText(search) || !validText(className) || (teacherID != "" && !validIdentifier(teacherID)) ||
		!validListFilters(role, status, className, teacherID) {
		return ListResponse{}, ErrInvalidInput
	}
	items, total, err := s.repo.ListThreads(ctx, userID, role, search, status, className, teacherID, page, pageSize)
	if err != nil {
		return ListResponse{}, err
	}
	return ListResponse{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// GetThread returns a single thread with messages.
func (s *Service) GetThread(ctx context.Context, userID string, threadID string, role user.Role, page, pageSize int) (any, error) {
	threadID = strings.TrimSpace(threadID)
	if (role != user.RoleStudent && role != user.RoleTeacher) || !validIdentifier(threadID) ||
		page < 1 || page > maxPageNumber || pageSize < 1 || pageSize > maxPageSize {
		return nil, ErrInvalidInput
	}
	item, found, err := s.repo.GetThread(ctx, threadID, userID, role, page, pageSize)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrNotFound
	}
	return item, nil
}

// AcknowledgeThreadRead marks only teacher replies at or before a cutoff returned by GetThread.
func (s *Service) AcknowledgeThreadRead(ctx context.Context, studentID string, threadID string, throughMessageID string) error {
	threadID = strings.TrimSpace(threadID)
	throughMessageID = strings.TrimSpace(throughMessageID)
	if !validIdentifier(threadID) || !validIdentifier(throughMessageID) {
		return ErrInvalidInput
	}
	ok, err := s.repo.AcknowledgeThreadRead(ctx, threadID, studentID, throughMessageID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// CreateThread creates a new question thread.
func (s *Service) CreateThread(ctx context.Context, studentID string, teacherID string, content string, source string) (ThreadDetail, error) {
	teacherID = strings.TrimSpace(teacherID)
	content = strings.TrimSpace(content)
	source = strings.TrimSpace(source)
	if source == "" {
		source = "消息中心"
	}
	if !validIdentifier(teacherID) || content == "" || utf8.RuneCountInString(content) > maxContentRunes ||
		utf8.RuneCountInString(source) > maxSourceRunes || !validText(content) || !validText(source) {
		return ThreadDetail{}, ErrInvalidInput
	}
	return s.repo.CreateThread(ctx, studentID, teacherID, content, source, time.Now())
}

// CreateThreadMessage adds a message to a thread.
func (s *Service) CreateThreadMessage(ctx context.Context, threadID string, senderID string, senderRole string, text string) (Message, error) {
	threadID = strings.TrimSpace(threadID)
	text = strings.TrimSpace(text)
	if !validIdentifier(threadID) || (senderRole != string(user.RoleStudent) && senderRole != string(user.RoleTeacher)) ||
		text == "" || utf8.RuneCountInString(text) > maxMessageRunes || !validText(text) {
		return Message{}, ErrInvalidInput
	}
	return s.repo.CreateThreadMessage(ctx, threadID, senderID, senderRole, text, time.Now())
}

// UpdateThreadStatus updates a thread's status.
func (s *Service) UpdateThreadStatus(ctx context.Context, threadID string, teacherID string, status string) error {
	threadID = strings.TrimSpace(threadID)
	status = strings.TrimSpace(status)
	if !validIdentifier(threadID) {
		return ErrInvalidInput
	}
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

func validIdentifier(value string) bool {
	return value != "" && utf8.RuneCountInString(value) <= maxIdentifierRunes && validText(value)
}

func validText(value string) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

func validStatusFilter(status string) bool {
	switch status {
	case "", "全部", "待回复", "已回复", "已解决", "需跟进":
		return true
	default:
		return false
	}
}

func validListFilters(role user.Role, status string, className string, teacherID string) bool {
	if !validStatusFilter(status) {
		return false
	}
	if role == user.RoleStudent {
		return (status == "" || status == "全部") && className == ""
	}
	return teacherID == ""
}
