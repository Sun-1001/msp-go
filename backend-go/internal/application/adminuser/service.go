package adminuser

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	authapp "mathstudy/backend-go/internal/application/auth"
	"mathstudy/backend-go/internal/domain/user"
	"mathstudy/backend-go/internal/platform/numutil"
	"mathstudy/backend-go/internal/platform/redact"
)

var (
	// ErrNotFound is returned when an account cannot be found.
	ErrNotFound = errors.New("admin user not found")
	// ErrBadRequest is returned when input cannot be applied.
	ErrBadRequest = errors.New("bad admin user request")
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

// Repository is the persistence surface required by admin user management.
type Repository interface {
	AccountStats(context.Context) (AccountStats, error)
	ListUsers(context.Context, ListFilter) ([]UserItem, int, error)
	GetByUsername(context.Context, string) (user.User, bool, error)
	GetByEmail(context.Context, string) (user.User, bool, error)
	Create(context.Context, user.CreateUser) (user.User, error)
	UpdateUser(context.Context, string, Update, *string, time.Time) (user.User, bool, error)
	UpdateUserStatus(context.Context, string, user.Status, time.Time) (user.User, bool, error)
	DeleteUser(context.Context, string) (bool, error)
	ExportUsers(context.Context, ListFilter) ([]ExportUser, error)
}

// AccountStats mirrors /admin/users/stats.
type AccountStats struct {
	Total     int `json:"total"`
	Active    int `json:"active"`
	Suspended int `json:"suspended"`
}

// UserItem is the Python-compatible user list item.
type UserItem struct {
	ID          string      `json:"id"`
	Username    string      `json:"username"`
	Email       string      `json:"email"`
	DisplayName *string     `json:"display_name"`
	Role        user.Role   `json:"role"`
	Status      user.Status `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
}

// ListFilter stores admin user list and export filters.
type ListFilter struct {
	Page     int
	PageSize int
	Search   string
	Role     string
	Status   string
}

// ListResponse wraps paginated user rows.
type ListResponse struct {
	Items      []UserItem `json:"items"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	TotalPages int        `json:"total_pages"`
}

// Create stores admin-created user fields.
type Create struct {
	Username    string
	Email       string
	Password    string
	Role        string
	DisplayName *string
}

// CreateResponse mirrors the Python response that returns 200 for duplicates.
type CreateResponse struct {
	Success bool      `json:"success"`
	Message string    `json:"message"`
	User    *UserItem `json:"user"`
}

// Update stores optional admin user updates.
type Update struct {
	DisplayName *string
	Password    *string
}

// UpdateResponse mirrors update/status responses.
type UpdateResponse struct {
	Success bool     `json:"success"`
	Message string   `json:"message"`
	User    UserItem `json:"user"`
}

// DeleteResponse mirrors delete responses.
type DeleteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ExportUser stores one CSV export row.
type ExportUser struct {
	Username    string
	Email       string
	DisplayName string
	Role        string
	Status      string
	CreatedAt   time.Time
}

// ImportUser stores one parsed CSV import row.
type ImportUser struct {
	Username    string
	Email       string
	Password    string
	Role        string
	DisplayName *string
}

// ImportResult stores one row result.
type ImportResult struct {
	Row      int    `json:"row"`
	Username string `json:"username"`
	Success  bool   `json:"success"`
	Message  string `json:"message"`
}

// ImportResponse mirrors the Python import response.
type ImportResponse struct {
	Success bool           `json:"success"`
	Total   int            `json:"total"`
	Created int            `json:"created"`
	Failed  int            `json:"failed"`
	Skipped int            `json:"skipped"`
	Details []ImportResult `json:"details"`
}

// Service implements admin user management use cases.
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService creates an admin user service.
func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, errors.New("admin user repository is nil")
	}
	return &Service{repo: repo, now: func() time.Time { return time.Now().UTC() }}, nil
}

// AccountStats returns current user account counters.
func (s *Service) AccountStats(ctx context.Context) (AccountStats, error) {
	return s.repo.AccountStats(ctx)
}

// ListUsers returns a filtered user page.
func (s *Service) ListUsers(ctx context.Context, filter ListFilter) (ListResponse, error) {
	normalized, err := normalizeListFilter(filter)
	if err != nil {
		return ListResponse{}, err
	}
	items, total, err := s.repo.ListUsers(ctx, normalized)
	if err != nil {
		return ListResponse{}, err
	}
	return ListResponse{
		Items:      items,
		Total:      total,
		Page:       normalized.Page,
		PageSize:   normalized.PageSize,
		TotalPages: numutil.TotalPages(total, normalized.PageSize),
	}, nil
}

// CreateUser creates an account or returns a Python-compatible duplicate response.
func (s *Service) CreateUser(ctx context.Context, input Create) (CreateResponse, error) {
	normalized, role, err := normalizeCreate(input)
	if err != nil {
		return CreateResponse{}, err
	}
	if existing, ok, err := s.repo.GetByUsername(ctx, normalized.Username); err != nil {
		return CreateResponse{}, fmt.Errorf("get user by username: %w", err)
	} else if ok {
		return CreateResponse{Success: false, Message: "用户名 '" + existing.Username + "' 已存在"}, nil
	}
	if existing, ok, err := s.repo.GetByEmail(ctx, normalized.Email); err != nil {
		return CreateResponse{}, fmt.Errorf("get user by email: %w", err)
	} else if ok {
		return CreateResponse{Success: false, Message: "邮箱 '" + existing.Email + "' 已被使用"}, nil
	}
	hash, err := authapp.HashPassword(normalized.Password)
	if err != nil {
		return CreateResponse{}, fmt.Errorf("hash password: %w", err)
	}
	now := s.now()
	account, err := s.repo.Create(ctx, user.CreateUser{
		Username:       normalized.Username,
		Email:          normalized.Email,
		HashedPassword: hash,
		Role:           role,
		DisplayName:    normalized.DisplayName,
		IsActive:       true,
		Status:         user.StatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		return CreateResponse{}, fmt.Errorf("create user: %w", err)
	}
	item := toUserItem(account)
	return CreateResponse{Success: true, Message: "用户创建成功", User: &item}, nil
}

// UpdateUser updates display name and optionally resets password.
func (s *Service) UpdateUser(ctx context.Context, userID string, update Update) (UpdateResponse, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return UpdateResponse{}, badRequest("用户 ID 不能为空")
	}
	normalized, err := normalizeUpdate(update)
	if err != nil {
		return UpdateResponse{}, err
	}
	var passwordHash *string
	if normalized.Password != nil {
		hash, err := authapp.HashPassword(*normalized.Password)
		if err != nil {
			return UpdateResponse{}, fmt.Errorf("hash password: %w", err)
		}
		passwordHash = &hash
	}
	account, ok, err := s.repo.UpdateUser(ctx, userID, normalized, passwordHash, s.now())
	if err != nil {
		return UpdateResponse{}, fmt.Errorf("update user: %w", err)
	}
	if !ok {
		return UpdateResponse{}, notFound("用户不存在")
	}
	return UpdateResponse{Success: true, Message: "用户信息更新成功", User: toUserItem(account)}, nil
}

// UpdateUserStatus toggles account active/suspended state.
func (s *Service) UpdateUserStatus(ctx context.Context, userID string, statusValue string) (UpdateResponse, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return UpdateResponse{}, badRequest("用户 ID 不能为空")
	}
	status, err := parseManagedStatus(statusValue)
	if err != nil {
		return UpdateResponse{}, err
	}
	account, ok, err := s.repo.UpdateUserStatus(ctx, userID, status, s.now())
	if err != nil {
		return UpdateResponse{}, fmt.Errorf("update user status: %w", err)
	}
	if !ok {
		return UpdateResponse{}, notFound("用户不存在")
	}
	message := "用户状态已更新"
	if status == user.StatusActive {
		message = "用户已解锁"
	}
	if status == user.StatusSuspended {
		message = "用户已停用"
	}
	return UpdateResponse{Success: true, Message: message, User: toUserItem(account)}, nil
}

// DeleteUser physically deletes the user and dependent records.
func (s *Service) DeleteUser(ctx context.Context, userID string) (DeleteResponse, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return DeleteResponse{}, badRequest("用户 ID 不能为空")
	}
	deleted, err := s.repo.DeleteUser(ctx, userID)
	if err != nil {
		return DeleteResponse{}, fmt.Errorf("delete user: %w", err)
	}
	if !deleted {
		return DeleteResponse{}, notFound("用户不存在")
	}
	return DeleteResponse{Success: true, Message: "用户已删除"}, nil
}

// ExportUsers returns non-admin users matching filters.
func (s *Service) ExportUsers(ctx context.Context, filter ListFilter) ([]ExportUser, error) {
	normalized, err := normalizeExportFilter(filter)
	if err != nil {
		return nil, err
	}
	return s.repo.ExportUsers(ctx, normalized)
}

// ImportUsers creates users from parsed CSV rows.
func (s *Service) ImportUsers(ctx context.Context, rows []ImportUser) (ImportResponse, error) {
	response := ImportResponse{Total: len(rows), Details: []ImportResult{}}
	for index, row := range rows {
		rowNumber := index + 1
		normalized := normalizeImportUser(row)
		if normalized.Username == "" || normalized.Email == "" || normalized.Password == "" {
			response.Failed++
			response.Details = append(response.Details, ImportResult{Row: rowNumber, Username: emptyUsername(normalized.Username), Message: "用户名、邮箱和密码为必填项"})
			continue
		}
		roleValue := normalized.Role
		if roleValue == "" {
			roleValue = string(user.RoleStudent)
		}
		role, err := user.ParseRole(roleValue)
		if err != nil {
			response.Failed++
			response.Details = append(response.Details, ImportResult{Row: rowNumber, Username: normalized.Username, Message: "无效的角色: " + normalized.Role})
			continue
		}
		if validationErrors := authapp.ValidatePasswordStrength(normalized.Password); len(validationErrors) > 0 {
			response.Failed++
			response.Details = append(response.Details, ImportResult{Row: rowNumber, Username: normalized.Username, Message: "密码不符合安全策略: " + strings.Join(validationErrors, "；")})
			continue
		}
		if existing, ok, err := s.repo.GetByUsername(ctx, normalized.Username); err != nil {
			return ImportResponse{}, fmt.Errorf("get user by username: %w", err)
		} else if ok {
			response.Skipped++
			response.Details = append(response.Details, ImportResult{Row: rowNumber, Username: normalized.Username, Message: "用户名 '" + existing.Username + "' 已存在"})
			continue
		}
		if existing, ok, err := s.repo.GetByEmail(ctx, normalized.Email); err != nil {
			return ImportResponse{}, fmt.Errorf("get user by email: %w", err)
		} else if ok {
			response.Skipped++
			response.Details = append(response.Details, ImportResult{Row: rowNumber, Username: normalized.Username, Message: "邮箱 '" + existing.Email + "' 已被使用"})
			continue
		}
		hash, err := authapp.HashPassword(normalized.Password)
		if err != nil {
			response.Failed++
			response.Details = append(response.Details, ImportResult{Row: rowNumber, Username: normalized.Username, Message: "创建失败: " + redact.String(err.Error())})
			continue
		}
		now := s.now()
		if _, err := s.repo.Create(ctx, user.CreateUser{
			Username:       normalized.Username,
			Email:          normalized.Email,
			HashedPassword: hash,
			Role:           role,
			DisplayName:    normalized.DisplayName,
			IsActive:       true,
			Status:         user.StatusActive,
			CreatedAt:      now,
			UpdatedAt:      now,
		}); err != nil {
			response.Failed++
			response.Details = append(response.Details, ImportResult{Row: rowNumber, Username: normalized.Username, Message: "创建失败: " + redact.String(err.Error())})
			continue
		}
		response.Created++
		response.Details = append(response.Details, ImportResult{Row: rowNumber, Username: normalized.Username, Success: true, Message: "创建成功"})
	}
	response.Success = response.Failed == 0
	return response, nil
}

func normalizeListFilter(filter ListFilter) (ListFilter, error) {
	if filter.Page == 0 {
		filter.Page = 1
	}
	if filter.PageSize == 0 {
		filter.PageSize = 10
	}
	if filter.Page < 1 {
		return ListFilter{}, badRequest("page 必须大于等于 1")
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		return ListFilter{}, badRequest("page_size 必须在 1 到 100 之间")
	}
	return normalizeExportFilter(filter)
}

func normalizeExportFilter(filter ListFilter) (ListFilter, error) {
	filter.Search = strings.TrimSpace(filter.Search)
	filter.Role = normalizeAllFilter(filter.Role)
	filter.Status = normalizeAllFilter(filter.Status)
	if filter.Role != "" {
		if _, err := user.ParseRole(filter.Role); err != nil {
			return ListFilter{}, badRequest("role 角色无效")
		}
	}
	if filter.Status != "" {
		if _, err := parseManagedStatus(filter.Status); err != nil {
			return ListFilter{}, err
		}
	}
	return filter, nil
}

func normalizeCreate(input Create) (Create, user.Role, error) {
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.TrimSpace(input.Email)
	input.Password = strings.TrimSpace(input.Password)
	input.Role = strings.TrimSpace(input.Role)
	if input.Role == "" {
		input.Role = string(user.RoleStudent)
	}
	if input.DisplayName != nil {
		trimmed := strings.TrimSpace(*input.DisplayName)
		input.DisplayName = &trimmed
	}
	if len(input.Username) < 3 || len(input.Username) > 50 {
		return Create{}, "", badRequest("username 长度必须在 3 到 50 之间")
	}
	if input.Email == "" {
		return Create{}, "", badRequest("email 不能为空")
	}
	if validationErrors := authapp.ValidatePasswordStrength(input.Password); len(validationErrors) > 0 {
		return Create{}, "", badRequest(strings.Join(validationErrors, "；"))
	}
	if err := validateOptionalDisplayName(input.DisplayName); err != nil {
		return Create{}, "", err
	}
	role, err := user.ParseRole(input.Role)
	if err != nil {
		return Create{}, "", badRequest("role 角色无效")
	}
	return input, role, nil
}

func normalizeUpdate(update Update) (Update, error) {
	if update.DisplayName != nil {
		trimmed := strings.TrimSpace(*update.DisplayName)
		update.DisplayName = &trimmed
	}
	if err := validateOptionalDisplayName(update.DisplayName); err != nil {
		return Update{}, err
	}
	if update.Password != nil {
		password := strings.TrimSpace(*update.Password)
		if validationErrors := authapp.ValidatePasswordStrength(password); len(validationErrors) > 0 {
			return Update{}, badRequest(strings.Join(validationErrors, "；"))
		}
		update.Password = &password
	}
	return update, nil
}

func validateOptionalDisplayName(value *string) error {
	if value != nil && len(*value) > 100 {
		return badRequest("display_name 长度超出限制")
	}
	return nil
}

func parseManagedStatus(value string) (user.Status, error) {
	status, err := user.ParseStatus(value)
	if err != nil || (status != user.StatusActive && status != user.StatusSuspended) {
		return "", badRequest("status 必须是 active 或 suspended")
	}
	return status, nil
}

func normalizeImportUser(input ImportUser) ImportUser {
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.TrimSpace(input.Email)
	input.Password = strings.TrimSpace(input.Password)
	input.Role = strings.TrimSpace(input.Role)
	if input.DisplayName != nil {
		value := strings.TrimSpace(*input.DisplayName)
		if value == "" {
			input.DisplayName = nil
		} else {
			input.DisplayName = &value
		}
	}
	return input
}

func toUserItem(account user.User) UserItem {
	return UserItem{
		ID:          account.ID,
		Username:    account.Username,
		Email:       account.Email,
		DisplayName: account.DisplayName,
		Role:        account.Role,
		Status:      account.Status,
		CreatedAt:   account.CreatedAt,
	}
}

func normalizeAllFilter(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "all" {
		return ""
	}
	return value
}

func emptyUsername(value string) string {
	if value == "" {
		return "(空)"
	}
	return value
}

func badRequest(message string) error {
	return Error{Kind: ErrBadRequest, Message: message}
}

func notFound(message string) error {
	return Error{Kind: ErrNotFound, Message: message}
}
