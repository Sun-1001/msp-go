package admininbox

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	authapp "mathstudy/backend/internal/application/auth"
	emailapp "mathstudy/backend/internal/application/email"
	"mathstudy/backend/internal/platform/securerand"
)

const (
	tempPasswordLength  = 12
	tempPasswordLower   = "abcdefghijklmnopqrstuvwxyz"
	tempPasswordUpper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	tempPasswordDigits  = "0123456789"
	tempPasswordSpecial = "!@#$%^&*()-_=+[]{}"
	tempPasswordChars   = tempPasswordLower + tempPasswordUpper + tempPasswordDigits + tempPasswordSpecial
)

var (
	// ErrBadRequest is returned when input cannot be applied.
	ErrBadRequest = errors.New("bad admin inbox request")
)

// Error wraps a domain error with a Python-compatible message.
type Error struct {
	Kind    error
	Message string
}

func (e Error) Error() string {
	return e.Message
}

func (e Error) Unwrap() error {
	return e.Kind
}

// Repository is the persistence surface required by admin password reset inbox.
type Repository interface {
	ListPasswordResetRequests(context.Context, ListFilter) ([]RequestItem, int, int, error)
	CountPendingPasswordResetRequests(context.Context) (int, error)
	ReviewPasswordResetRequest(context.Context, ReviewUpdate) (ReviewResult, error)
}

// LoginFailureClearer clears failed login counters after a password reset.
type LoginFailureClearer interface {
	Clear(context.Context, string)
}

// Status is the password reset request state.
type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
)

// RequestItem mirrors the admin password reset request list item.
type RequestItem struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Username   string     `json:"username"`
	Email      string     `json:"email"`
	Reason     string     `json:"reason"`
	Status     Status     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	ReviewedAt *time.Time `json:"reviewed_at"`
}

// ListFilter stores admin inbox list filters.
type ListFilter struct {
	Status   string
	Page     int
	PageSize int
}

// ListResponse wraps password reset request rows.
type ListResponse struct {
	Items        []RequestItem `json:"items"`
	Total        int           `json:"total"`
	PendingCount int           `json:"pending_count"`
}

// ReviewUpdate stores fields needed to apply an admin review.
type ReviewUpdate struct {
	RequestID    string
	AdminID      string
	Action       string
	RejectReason *string
	PasswordHash *string
	ReviewedAt   time.Time
}

// ReviewResult reports repository-side review conditions.
type ReviewResult struct {
	Found            bool
	AlreadyProcessed bool
	UserFound        bool
	Username         string
	Email            string
}

// ReviewResponse mirrors the Python review response.
type ReviewResponse struct {
	Success           bool                         `json:"success"`
	Message           string                       `json:"message"`
	TempPassword      *string                      `json:"temp_password"`
	EmailNotification *emailapp.NotificationResult `json:"email_notification,omitempty"`
}

// Service implements admin password reset inbox use cases.
type Service struct {
	repo     Repository
	clearer  LoginFailureClearer
	notifier emailapp.EventSender
	now      func() time.Time
}

// SetEventSender wires the optional post-commit password notification sender.
func (s *Service) SetEventSender(sender emailapp.EventSender) {
	s.notifier = sender
}

// NewService creates an admin inbox service.
func NewService(repo Repository, clearers ...LoginFailureClearer) (*Service, error) {
	if repo == nil {
		return nil, errors.New("admin inbox repository is nil")
	}
	var clearer LoginFailureClearer
	if len(clearers) > 0 {
		clearer = clearers[0]
	}
	return &Service{
		repo:    repo,
		clearer: clearer,
		now:     func() time.Time { return time.Now().UTC() },
	}, nil
}

// ListRequests returns a filtered password reset request page.
func (s *Service) ListRequests(ctx context.Context, filter ListFilter) (ListResponse, error) {
	normalized, err := normalizeListFilter(filter)
	if err != nil {
		return ListResponse{}, err
	}
	items, total, pendingCount, err := s.repo.ListPasswordResetRequests(ctx, normalized)
	if err != nil {
		return ListResponse{}, err
	}
	return ListResponse{Items: items, Total: total, PendingCount: pendingCount}, nil
}

// PendingCount returns the number of pending password reset requests.
func (s *Service) PendingCount(ctx context.Context) (int, error) {
	return s.repo.CountPendingPasswordResetRequests(ctx)
}

// ReviewRequest approves or rejects one password reset request.
func (s *Service) ReviewRequest(ctx context.Context, requestID string, adminID string, action string, rejectReason *string) (ReviewResponse, error) {
	requestID = strings.TrimSpace(requestID)
	adminID = strings.TrimSpace(adminID)
	action = strings.ToLower(strings.TrimSpace(action))
	if requestID == "" {
		return ReviewResponse{}, badRequest("申请 ID 不能为空")
	}
	if adminID == "" {
		return ReviewResponse{}, badRequest("管理员 ID 不能为空")
	}
	if rejectReason != nil {
		value := strings.TrimSpace(*rejectReason)
		if len(value) > 500 {
			return ReviewResponse{}, badRequest("reject_reason 长度不能超过 500")
		}
		rejectReason = &value
	}

	var passwordHash *string
	var tempPassword *string
	switch action {
	case "approve":
		password, err := generateTempPassword()
		if err != nil {
			return ReviewResponse{}, fmt.Errorf("generate temporary password: %w", err)
		}
		hash, err := authapp.HashPassword(password)
		if err != nil {
			return ReviewResponse{}, fmt.Errorf("hash temporary password: %w", err)
		}
		passwordHash = &hash
		tempPassword = &password
	case "reject":
	default:
		return ReviewResponse{Success: false, Message: "无效的操作"}, nil
	}

	result, err := s.repo.ReviewPasswordResetRequest(ctx, ReviewUpdate{
		RequestID:    requestID,
		AdminID:      adminID,
		Action:       action,
		RejectReason: rejectReason,
		PasswordHash: passwordHash,
		ReviewedAt:   s.now(),
	})
	if err != nil {
		return ReviewResponse{}, err
	}
	if !result.Found {
		return ReviewResponse{Success: false, Message: "申请不存在"}, nil
	}
	if result.AlreadyProcessed {
		return ReviewResponse{Success: false, Message: "该申请已处理"}, nil
	}
	if action == "approve" && !result.UserFound {
		return ReviewResponse{Success: false, Message: "用户不存在"}, nil
	}
	if action == "approve" {
		if s.clearer != nil {
			s.clearer.Clear(ctx, result.Username)
		}
		message := "已通过审批，请线下安全告知用户临时密码"
		var notification *emailapp.NotificationResult
		if s.notifier != nil && tempPassword != nil {
			deliveryErr := s.notifier.SendEvent(ctx, emailapp.EventRequest{
				Event:     emailapp.EventPasswordReset,
				Locale:    emailapp.LocaleZhCN,
				Recipient: result.Email,
				Variables: map[string]string{
					"display_name":  result.Username,
					"username":      result.Username,
					"temp_password": *tempPassword,
				},
			})
			report := emailapp.NotificationResultFromError(deliveryErr)
			notification = &report
			if report.Status == emailapp.NotificationSent {
				message = "已通过审批，临时密码邮件已发送"
			} else {
				message = "已通过审批，但" + report.Message + "，请线下安全告知用户临时密码"
			}
		}
		return ReviewResponse{
			Success:           true,
			Message:           message,
			TempPassword:      tempPassword,
			EmailNotification: notification,
		}, nil
	}
	return ReviewResponse{Success: true, Message: "已拒绝该申请"}, nil
}

func normalizeListFilter(filter ListFilter) (ListFilter, error) {
	if filter.Page == 0 {
		filter.Page = 1
	}
	if filter.PageSize == 0 {
		filter.PageSize = 20
	}
	if filter.Page < 1 {
		return ListFilter{}, badRequest("page 必须大于等于 1")
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		return ListFilter{}, badRequest("page_size 必须在 1 到 100 之间")
	}
	status, err := normalizeStatusFilter(filter.Status)
	if err != nil {
		return ListFilter{}, err
	}
	filter.Status = status
	return filter, nil
}

func normalizeStatusFilter(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || value == "all" {
		return "", nil
	}
	switch Status(value) {
	case StatusPending, StatusApproved, StatusRejected:
		return value, nil
	default:
		return "", badRequest("status 必须是 pending、approved 或 rejected")
	}
}

func generateTempPassword() (string, error) {
	var builder strings.Builder
	builder.Grow(tempPasswordLength)
	for _, pool := range []string{tempPasswordLower, tempPasswordUpper, tempPasswordDigits, tempPasswordSpecial} {
		char, err := securerand.Byte(pool)
		if err != nil {
			return "", err
		}
		builder.WriteByte(char)
	}
	for builder.Len() < tempPasswordLength {
		char, err := securerand.Byte(tempPasswordChars)
		if err != nil {
			return "", err
		}
		builder.WriteByte(char)
	}
	return securerand.ShuffleString(builder.String())
}

func badRequest(message string) error {
	return Error{Kind: ErrBadRequest, Message: message}
}
