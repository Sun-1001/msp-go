package announcement

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"mathstudy/backend-go/internal/domain/user"
	"mathstudy/backend-go/internal/platform/identifier"
)

const (
	maxTitleRunes   = 120
	maxContentRunes = 50000
)

var (
	// ErrBadRequest is returned when announcement input is invalid.
	ErrBadRequest = errors.New("bad announcement request")
	// ErrNotFound is returned when an announcement cannot be found or accessed.
	ErrNotFound = errors.New("announcement not found")
)

// Error wraps an application error with a user-safe message.
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

// Audience identifies which user roles receive an announcement.
type Audience string

const (
	AudienceStudent Audience = "student"
	AudienceTeacher Audience = "teacher"
	AudienceAll     Audience = "all"
)

// ContentFormat identifies the announcement renderer.
type ContentFormat string

const (
	ContentFormatMarkdown ContentFormat = "markdown"
	ContentFormatHTML     ContentFormat = "html"
)

// Announcement is the persisted announcement response shape.
type Announcement struct {
	ID            string        `json:"id"`
	Title         string        `json:"title"`
	Content       string        `json:"content"`
	ContentFormat ContentFormat `json:"content_format"`
	Audience      Audience      `json:"audience"`
	Append        bool          `json:"append"`
	Persistent    bool          `json:"persistent"`
	IsActive      bool          `json:"is_active"`
	Revision      int           `json:"revision"`
	PublishedAt   time.Time     `json:"published_at"`
	CreatedBy     *string       `json:"created_by"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// SaveRequest stores administrator-editable announcement fields.
type SaveRequest struct {
	Title         string `json:"title"`
	Content       string `json:"content"`
	ContentFormat string `json:"content_format"`
	Audience      string `json:"audience"`
	Append        bool   `json:"append"`
	Persistent    bool   `json:"persistent"`
	IsActive      *bool  `json:"is_active"`
}

// CreateParams stores normalized fields required for insertion.
type CreateParams struct {
	ID            string
	Title         string
	Content       string
	ContentFormat ContentFormat
	Audience      Audience
	Append        bool
	Persistent    bool
	IsActive      bool
	CreatedBy     string
	Now           time.Time
}

// UpdateParams stores normalized fields required for an update.
type UpdateParams struct {
	ID            string
	Title         string
	Content       string
	ContentFormat ContentFormat
	Audience      Audience
	Append        bool
	Persistent    bool
	IsActive      *bool
	Now           time.Time
}

// DismissResult reports whether the target can be dismissed permanently.
type DismissResult struct {
	Found      bool
	Persistent bool
}

// Repository is the persistence surface required by announcement use cases.
type Repository interface {
	ListAnnouncementsForAdmin(context.Context) ([]Announcement, error)
	ListAnnouncementsForUser(context.Context, string, user.Role) ([]Announcement, error)
	CreateAnnouncement(context.Context, CreateParams) (Announcement, error)
	UpdateAnnouncement(context.Context, UpdateParams) (Announcement, bool, error)
	DeleteAnnouncement(context.Context, string) (bool, error)
	DismissAnnouncement(context.Context, string, string, user.Role, time.Time) (DismissResult, error)
}

// ListResponse wraps an announcement list.
type ListResponse struct {
	Items []Announcement `json:"items"`
}

// DeleteResponse confirms an administrator delete action.
type DeleteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// DismissResponse confirms a user's permanent dismissal choice.
type DismissResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Service implements administrator and recipient announcement use cases.
type Service struct {
	repo  Repository
	now   func() time.Time
	newID func() (string, error)
}

// NewService creates an announcement service.
func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, errors.New("announcement repository is nil")
	}
	return &Service{
		repo:  repo,
		now:   func() time.Time { return time.Now().UTC() },
		newID: identifier.NewUUID,
	}, nil
}

// ListForAdmin returns active and inactive announcements for management.
func (s *Service) ListForAdmin(ctx context.Context) (ListResponse, error) {
	items, err := s.repo.ListAnnouncementsForAdmin(ctx)
	if err != nil {
		return ListResponse{}, err
	}
	return ListResponse{Items: items}, nil
}

// ListForUser returns active, role-matched announcements not dismissed at the current revision.
func (s *Service) ListForUser(ctx context.Context, userID string, role user.Role) (ListResponse, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ListResponse{}, badRequest("用户 ID 不能为空")
	}
	if !isRecipientRole(role) {
		return ListResponse{}, badRequest("仅学生或教师可以接收系统公告")
	}
	items, err := s.repo.ListAnnouncementsForUser(ctx, userID, role)
	if err != nil {
		return ListResponse{}, err
	}
	return ListResponse{Items: items}, nil
}

// Create publishes a new announcement.
func (s *Service) Create(ctx context.Context, adminID string, request SaveRequest) (Announcement, error) {
	adminID = strings.TrimSpace(adminID)
	if adminID == "" {
		return Announcement{}, badRequest("管理员 ID 不能为空")
	}
	if request.IsActive == nil {
		active := true
		request.IsActive = &active
	}
	normalized, err := normalizeSaveRequest(request)
	if err != nil {
		return Announcement{}, err
	}
	id, err := s.newID()
	if err != nil {
		return Announcement{}, fmt.Errorf("generate announcement id: %w", err)
	}
	now := s.now()
	return s.repo.CreateAnnouncement(ctx, CreateParams{
		ID:            id,
		Title:         normalized.Title,
		Content:       normalized.Content,
		ContentFormat: ContentFormat(normalized.ContentFormat),
		Audience:      Audience(normalized.Audience),
		Append:        normalized.Append,
		Persistent:    normalized.Persistent,
		IsActive:      *normalized.IsActive,
		CreatedBy:     adminID,
		Now:           now,
	})
}

// Update edits and republishes an existing announcement with a new revision.
func (s *Service) Update(ctx context.Context, announcementID string, request SaveRequest) (Announcement, error) {
	announcementID = strings.TrimSpace(announcementID)
	if announcementID == "" {
		return Announcement{}, badRequest("公告 ID 不能为空")
	}
	normalized, err := normalizeSaveRequest(request)
	if err != nil {
		return Announcement{}, err
	}
	item, found, err := s.repo.UpdateAnnouncement(ctx, UpdateParams{
		ID:            announcementID,
		Title:         normalized.Title,
		Content:       normalized.Content,
		ContentFormat: ContentFormat(normalized.ContentFormat),
		Audience:      Audience(normalized.Audience),
		Append:        normalized.Append,
		Persistent:    normalized.Persistent,
		IsActive:      normalized.IsActive,
		Now:           s.now(),
	})
	if err != nil {
		return Announcement{}, err
	}
	if !found {
		return Announcement{}, notFound("公告不存在")
	}
	return item, nil
}

// Delete permanently removes one announcement and its dismissal rows.
func (s *Service) Delete(ctx context.Context, announcementID string) (DeleteResponse, error) {
	announcementID = strings.TrimSpace(announcementID)
	if announcementID == "" {
		return DeleteResponse{}, badRequest("公告 ID 不能为空")
	}
	found, err := s.repo.DeleteAnnouncement(ctx, announcementID)
	if err != nil {
		return DeleteResponse{}, err
	}
	if !found {
		return DeleteResponse{}, notFound("公告不存在")
	}
	return DeleteResponse{Success: true, Message: "公告已删除"}, nil
}

// Dismiss records an account-level "do not show again" choice for a non-persistent announcement.
func (s *Service) Dismiss(ctx context.Context, announcementID string, userID string, role user.Role) (DismissResponse, error) {
	announcementID = strings.TrimSpace(announcementID)
	userID = strings.TrimSpace(userID)
	if announcementID == "" || userID == "" {
		return DismissResponse{}, badRequest("公告 ID 和用户 ID 不能为空")
	}
	if !isRecipientRole(role) {
		return DismissResponse{}, badRequest("仅学生或教师可以关闭系统公告")
	}
	result, err := s.repo.DismissAnnouncement(ctx, announcementID, userID, role, s.now())
	if err != nil {
		return DismissResponse{}, err
	}
	if !result.Found {
		return DismissResponse{}, notFound("公告不存在或不可访问")
	}
	if result.Persistent {
		return DismissResponse{}, badRequest("常驻公告不能设置为不再弹出")
	}
	return DismissResponse{Success: true, Message: "该公告将不再弹出"}, nil
}

func normalizeSaveRequest(request SaveRequest) (SaveRequest, error) {
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" {
		return SaveRequest{}, badRequest("公告标题不能为空")
	}
	if utf8.RuneCountInString(request.Title) > maxTitleRunes {
		return SaveRequest{}, badRequest("公告标题长度不能超过 120 个字符")
	}
	if strings.ContainsAny(request.Title, "\r\n\x00") {
		return SaveRequest{}, badRequest("公告标题不能包含换行或空字符")
	}
	if strings.TrimSpace(request.Content) == "" {
		return SaveRequest{}, badRequest("公告正文不能为空")
	}
	if utf8.RuneCountInString(request.Content) > maxContentRunes {
		return SaveRequest{}, badRequest("公告正文长度不能超过 50000 个字符")
	}
	if strings.ContainsRune(request.Content, '\x00') {
		return SaveRequest{}, badRequest("公告正文不能包含空字符")
	}
	request.ContentFormat = strings.ToLower(strings.TrimSpace(request.ContentFormat))
	switch ContentFormat(request.ContentFormat) {
	case ContentFormatMarkdown, ContentFormatHTML:
	default:
		return SaveRequest{}, badRequest("content_format 必须是 markdown 或 html")
	}
	request.Audience = strings.ToLower(strings.TrimSpace(request.Audience))
	switch Audience(request.Audience) {
	case AudienceStudent, AudienceTeacher, AudienceAll:
	default:
		return SaveRequest{}, badRequest("audience 必须是 student、teacher 或 all")
	}
	if request.IsActive != nil {
		active := *request.IsActive
		request.IsActive = &active
	}
	return request, nil
}

func isRecipientRole(role user.Role) bool {
	return role == user.RoleStudent || role == user.RoleTeacher
}

func badRequest(message string) error {
	return Error{Kind: ErrBadRequest, Message: message}
}

func notFound(message string) error {
	return Error{Kind: ErrNotFound, Message: message}
}
