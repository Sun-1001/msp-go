package conversation

import (
	"context"
	"errors"
	"time"

	"mathstudy/backend-go/internal/domain/user"
)

var (
	ErrForbidden = errors.New("conversation forbidden")
	ErrNotFound  = errors.New("conversation not found")
	ErrConflict  = errors.New("conversation conflict")
)

// Repository is the persistence surface required by conversation use cases.
type Repository interface {
	// ListConversations returns paginated conversations for a user.
	// Role-based: students see their conversations with teachers, teachers see theirs with students.
	ListConversations(ctx context.Context, userID string, role user.Role, search string, status string, className string, page, pageSize int) ([]ConversationItem, int, error)
	// GetConversation returns full conversation detail with messages.
	GetConversation(ctx context.Context, conversationID string, userID string, page, pageSize int) (ConversationDetail, bool, error)
	// MarkConversationRead marks all messages in a conversation as read for the given user.
	MarkConversationRead(ctx context.Context, conversationID string, userID string) error
	// CreateConversation creates a new conversation between a student and teacher.
	CreateConversation(ctx context.Context, creatorID string, creatorRole user.Role, targetID string, subject string, initialMessage string, now time.Time) (ConversationDetail, error)
	// SendMessage adds a message to a conversation.
	SendMessage(ctx context.Context, conversationID string, senderID string, senderRole string, text string, now time.Time) (Message, error)
	// ArchiveConversation archives a conversation for the student.
	ArchiveConversation(ctx context.Context, conversationID string, studentID string) (bool, error)
	// DeleteConversation deletes a conversation (student only).
	DeleteConversation(ctx context.Context, conversationID string, studentID string) (bool, error)
	// ListTeacherContacts returns teachers the student can message.
	ListTeacherContacts(ctx context.Context, studentID string) ([]Contact, error)
	// ListStudentContacts returns students the teacher can message.
	ListStudentContacts(ctx context.Context, teacherID string) ([]Contact, error)
	// SearchContacts searches all users by ID or display name, filtered by role.
	SearchContacts(ctx context.Context, query string, role user.Role) ([]Contact, error)
}

// Message is a single message in a conversation.
type Message struct {
	ID              string    `json:"id"`
	From            string    `json:"from"`
	Text            string    `json:"text"`
	Time            time.Time `json:"time"`
	ReadByRecipient *bool     `json:"read_by_recipient,omitempty"`
}

// Contact is a teacher the student can start a conversation with.
type Contact struct {
	ID          string `json:"id"`
	TeacherName string `json:"teacher_name"`
	Scope       string `json:"scope"`
}

// ConversationItem is a list-level view of a conversation.
type ConversationItem struct {
	ID           string    `json:"id"`
	StudentID    string    `json:"student_id,omitempty"`
	TeacherID    string    `json:"teacher_id,omitempty"`
	StudentName  string    `json:"student_name,omitempty"`
	TeacherName  string    `json:"teacher_name,omitempty"`
	ClassName    string    `json:"class_name,omitempty"`
	Scope        string    `json:"scope,omitempty"`
	LastMessage  string    `json:"last_message"`
	LastTime     time.Time `json:"last_time"`
	Unread       int       `json:"unread"`
	PendingReply bool      `json:"pending_reply,omitempty"`
	Archived     bool      `json:"archived"`
}

// ConversationDetail includes full message history.
type ConversationDetail struct {
	ConversationItem
	Messages      []Message `json:"messages"`
	MessagesTotal int       `json:"messages_total"`
	MessagesPage  int       `json:"messages_page"`
	MessagesSize  int       `json:"messages_page_size"`
}

// ListResponse is the paginated list response.
type ListResponse struct {
	Items    []ConversationItem `json:"items"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

// Service implements conversation business logic.
type Service struct {
	repo Repository
}

// NewService creates a conversation service.
func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, errors.New("conversation repository is nil")
	}
	return &Service{repo: repo}, nil
}

// ListConversations returns the user's conversation list.
func (s *Service) ListConversations(ctx context.Context, userID string, role user.Role, search string, status string, className string, page int, pageSize int) (ListResponse, error) {
	items, total, err := s.repo.ListConversations(ctx, userID, role, search, status, className, page, pageSize)
	if err != nil {
		return ListResponse{}, err
	}
	return ListResponse{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// GetConversation returns a single conversation with messages.
func (s *Service) GetConversation(ctx context.Context, userID string, conversationID string, page, pageSize int) (ConversationDetail, error) {
	if err := s.repo.MarkConversationRead(ctx, conversationID, userID); err != nil {
		return ConversationDetail{}, err
	}
	detail, found, err := s.repo.GetConversation(ctx, conversationID, userID, page, pageSize)
	if err != nil {
		return ConversationDetail{}, err
	}
	if !found {
		return ConversationDetail{}, ErrNotFound
	}
	return detail, nil
}

// CreateConversation creates a new student-teacher conversation.
func (s *Service) CreateConversation(ctx context.Context, creatorID string, creatorRole user.Role, targetID string, subject string, initialMessage string) (ConversationDetail, error) {
	return s.repo.CreateConversation(ctx, creatorID, creatorRole, targetID, subject, initialMessage, time.Now())
}

// SendMessage sends a message in an existing conversation.
func (s *Service) SendMessage(ctx context.Context, conversationID string, senderID string, senderRole string, text string) (Message, error) {
	return s.repo.SendMessage(ctx, conversationID, senderID, senderRole, text, time.Now())
}

// ArchiveConversation archives a conversation.
func (s *Service) ArchiveConversation(ctx context.Context, conversationID string, studentID string) error {
	ok, err := s.repo.ArchiveConversation(ctx, conversationID, studentID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// DeleteConversation deletes a conversation.
func (s *Service) DeleteConversation(ctx context.Context, conversationID string, studentID string) error {
	ok, err := s.repo.DeleteConversation(ctx, conversationID, studentID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// ListTeacherContacts returns teachers available for messaging.
func (s *Service) ListTeacherContacts(ctx context.Context, studentID string) ([]Contact, error) {
	return s.repo.ListTeacherContacts(ctx, studentID)
}

// ListStudentContacts returns students available for messaging.
func (s *Service) ListStudentContacts(ctx context.Context, teacherID string) ([]Contact, error) {
	return s.repo.ListStudentContacts(ctx, teacherID)
}

// SearchContacts searches all users by ID or display name, filtered by role.
func (s *Service) SearchContacts(ctx context.Context, query string, role user.Role) ([]Contact, error) {
	return s.repo.SearchContacts(ctx, query, role)
}
