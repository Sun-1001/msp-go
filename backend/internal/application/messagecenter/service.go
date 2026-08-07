package messagecenter

import (
	"context"
	"errors"
	"time"

	"mathstudy/backend/internal/domain/user"
)

var ErrForbidden = errors.New("message center forbidden")

// Repository is the persistence surface required by message center summaries.
type Repository interface {
	Summary(context.Context, string, user.Role) (Summary, error)
}

// PreviewItem is a compact item rendered in the message preview bell.
type PreviewItem struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Title      string    `json:"title"`
	Summary    string    `json:"summary"`
	OccurredAt time.Time `json:"occurred_at"`
	Pending    bool      `json:"pending"`
}

// Summary aggregates pending counts and the five most recent message-center items.
type Summary struct {
	// ConversationCount is the total number of unread private messages.
	ConversationCount int           `json:"conversation_count"`
	NoticeCount       int           `json:"notice_count"`
	ThreadCount       int           `json:"thread_count"`
	ForumCount        int           `json:"forum_count"`
	Items             []PreviewItem `json:"items"`
}

// Service implements message center summary use cases.
type Service struct {
	repo Repository
}

// NewService creates a message center summary service.
func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, errors.New("message center repository is nil")
	}
	return &Service{repo: repo}, nil
}

// Summary returns the compact preview data for a student or teacher.
func (s *Service) Summary(ctx context.Context, userID string, role user.Role) (Summary, error) {
	if role != user.RoleStudent && role != user.RoleTeacher {
		return Summary{}, ErrForbidden
	}
	return s.repo.Summary(ctx, userID, role)
}
