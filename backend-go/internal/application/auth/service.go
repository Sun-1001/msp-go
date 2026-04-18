package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"mathstudy/backend-go/internal/domain/user"
)

const maxPasswordResetRequestsPerDay = 3

// UserRepository is the user persistence surface needed by auth use cases.
type UserRepository interface {
	GetByUsername(context.Context, string) (user.User, bool, error)
	GetByEmail(context.Context, string) (user.User, bool, error)
	GetByID(context.Context, string) (user.User, bool, error)
	Create(context.Context, user.CreateUser) (user.User, error)
	UpdatePassword(context.Context, string, string) error
}

// SettingsRepository reads public auth-related system settings.
type SettingsRepository interface {
	RegistrationSettings(context.Context) (RegistrationSettings, error)
}

// PasswordResetRepository stores public password reset requests.
type PasswordResetRepository interface {
	CountPasswordResetRequestsSince(context.Context, string, time.Time) (int, error)
	HasPendingPasswordReset(context.Context, string) (bool, error)
	CreatePasswordResetRequest(context.Context, PasswordResetRequest) (string, error)
	LatestPasswordResetRequestStatus(context.Context, string, string) (PasswordResetStatus, bool, error)
}

// RegistrationSettings mirrors /auth/registration-status.
type RegistrationSettings struct {
	AllowStudent bool
	AllowTeacher bool
}

// AuthResult mirrors the Python AuthResult used by login and register.
type AuthResult struct {
	Success      bool
	AccessToken  string
	RefreshToken string
	User         user.User
	Error        string
}

// Principal is the authenticated user context extracted from an access token.
type Principal struct {
	UserID string
	Role   user.Role
}

// PasswordResetRequest contains data for a new password reset request.
type PasswordResetRequest struct {
	UserID    string
	Username  string
	Email     string
	Reason    string
	CreatedAt time.Time
}

// PasswordResetResult mirrors the public forgot-password response.
type PasswordResetResult struct {
	Success   bool
	Message   string
	RequestID *string
}

// PasswordResetStatus mirrors the public forgot-password/status response.
type PasswordResetStatus struct {
	HasPending bool
	Status     *string
	CreatedAt  *time.Time
}

// Service implements auth and user-domain use cases.
type Service struct {
	users    UserRepository
	settings SettingsRepository
	resets   PasswordResetRepository
	tokens   TokenService
	limiter  *LoginLimiter
	logger   *slog.Logger
	now      func() time.Time
}

// NewService creates an auth service with explicit dependencies.
func NewService(
	users UserRepository,
	settings SettingsRepository,
	resets PasswordResetRepository,
	tokens TokenService,
	limiter *LoginLimiter,
	logger *slog.Logger,
) (*Service, error) {
	if users == nil {
		return nil, errors.New("auth user repository is nil")
	}
	if settings == nil {
		return nil, errors.New("auth settings repository is nil")
	}
	if resets == nil {
		return nil, errors.New("auth password reset repository is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		users:    users,
		settings: settings,
		resets:   resets,
		tokens:   tokens,
		limiter:  limiter,
		logger:   logger,
		now:      func() time.Time { return time.Now().UTC() },
	}, nil
}

// Authenticate verifies credentials and returns access/refresh tokens.
func (s *Service) Authenticate(ctx context.Context, username, password string) (AuthResult, error) {
	if s.limiter != nil && s.limiter.IsLocked(ctx, username) {
		return AuthResult{
			Success: false,
			Error:   fmt.Sprintf("账户已被临时锁定，请 %d 分钟后重试", s.limiter.LockoutMinutes()),
		}, nil
	}

	account, ok, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		return AuthResult{}, fmt.Errorf("get user by username: %w", err)
	}
	if !ok || !VerifyPassword(password, account.HashedPassword) {
		if s.limiter != nil {
			s.limiter.RecordFailure(ctx, username)
		}
		return AuthResult{Success: false, Error: "用户名或密码错误"}, nil
	}
	if !account.IsActive {
		return AuthResult{Success: false, Error: "账户已被禁用"}, nil
	}
	if s.limiter != nil {
		s.limiter.Clear(ctx, username)
	}
	return s.tokensForUser(account)
}

// Register validates settings and password policy, creates a user, and returns login state.
func (s *Service) Register(ctx context.Context, username, email, password, roleValue string) (AuthResult, error) {
	role, err := user.ParseRole(roleValue)
	if err != nil || role == user.RoleAdmin {
		return AuthResult{Success: false, Error: "仅支持学生或教师角色注册"}, nil
	}

	settings, err := s.settings.RegistrationSettings(ctx)
	if err != nil {
		return AuthResult{}, fmt.Errorf("get registration settings: %w", err)
	}
	if role == user.RoleStudent && !settings.AllowStudent {
		return AuthResult{Success: false, Error: "当前不允许学生注册"}, nil
	}
	if role == user.RoleTeacher && !settings.AllowTeacher {
		return AuthResult{Success: false, Error: "当前不允许教师注册"}, nil
	}

	if validationErrors := ValidatePasswordStrength(password); len(validationErrors) > 0 {
		return AuthResult{Success: false, Error: strings.Join(validationErrors, "；")}, nil
	}
	if _, ok, err := s.users.GetByUsername(ctx, username); err != nil {
		return AuthResult{}, fmt.Errorf("get user by username: %w", err)
	} else if ok {
		return AuthResult{Success: false, Error: "用户名已存在"}, nil
	}
	if _, ok, err := s.users.GetByEmail(ctx, email); err != nil {
		return AuthResult{}, fmt.Errorf("get user by email: %w", err)
	} else if ok {
		return AuthResult{Success: false, Error: "邮箱已被注册"}, nil
	}

	hash, err := HashPassword(password)
	if err != nil {
		return AuthResult{}, fmt.Errorf("hash password: %w", err)
	}
	now := s.now()
	account, err := s.users.Create(ctx, user.CreateUser{
		Username:       username,
		Email:          email,
		HashedPassword: hash,
		Role:           role,
		IsActive:       true,
		Status:         user.StatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		return AuthResult{}, fmt.Errorf("create user: %w", err)
	}

	s.logger.Info("new user registered", "username", username, "role", role)
	return s.tokensForUser(account)
}

// ChangePassword verifies the old password, enforces strength, and persists a new hash.
func (s *Service) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) (bool, string, error) {
	account, ok, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return false, "", fmt.Errorf("get user by id: %w", err)
	}
	if !ok {
		return false, "用户不存在", nil
	}
	if !VerifyPassword(oldPassword, account.HashedPassword) {
		return false, "原密码错误", nil
	}
	if validationErrors := ValidatePasswordStrength(newPassword); len(validationErrors) > 0 {
		return false, strings.Join(validationErrors, "；"), nil
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return false, "", fmt.Errorf("hash password: %w", err)
	}
	if err := s.users.UpdatePassword(ctx, userID, hash); err != nil {
		return false, "", fmt.Errorf("update password: %w", err)
	}
	if s.limiter != nil {
		s.limiter.Clear(ctx, account.Username)
	}
	return true, "密码修改成功", nil
}

// GetUserByID returns one user by ID.
func (s *Service) GetUserByID(ctx context.Context, userID string) (user.User, bool, error) {
	return s.users.GetByID(ctx, userID)
}

// RefreshTokens verifies the user still exists and is active, then rotates both token types.
func (s *Service) RefreshTokens(ctx context.Context, userID string) (string, string, bool, error) {
	account, ok, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return "", "", false, fmt.Errorf("get user by id: %w", err)
	}
	if !ok || !account.IsActive {
		return "", "", false, nil
	}
	result, err := s.tokensForUser(account)
	if err != nil {
		return "", "", false, err
	}
	return result.AccessToken, result.RefreshToken, true, nil
}

// DecodeAccessToken returns a principal for a valid access token.
func (s *Service) DecodeAccessToken(token string) (Principal, bool) {
	claims, err := s.tokens.Decode(token)
	if err != nil || claims.Type != "access" || claims.Subject == "" {
		return Principal{}, false
	}
	return Principal{UserID: claims.Subject, Role: claims.Role}, true
}

// DecodeRefreshToken returns a subject for a valid refresh token.
func (s *Service) DecodeRefreshToken(token string) (string, bool) {
	claims, err := s.tokens.Decode(token)
	if err != nil || claims.Type != "refresh" || claims.Subject == "" {
		return "", false
	}
	return claims.Subject, true
}

// RegistrationSettings returns the public registration toggles.
func (s *Service) RegistrationSettings(ctx context.Context) (RegistrationSettings, error) {
	return s.settings.RegistrationSettings(ctx)
}

// InitAdmin creates the configured admin account if it does not already exist.
func (s *Service) InitAdmin(ctx context.Context, username, email, password string) (bool, error) {
	if _, ok, err := s.users.GetByUsername(ctx, username); err != nil {
		return false, fmt.Errorf("get admin by username: %w", err)
	} else if ok {
		s.logger.Info("admin account already exists", "username", username)
		return false, nil
	}
	hash, err := HashPassword(password)
	if err != nil {
		return false, fmt.Errorf("hash admin password: %w", err)
	}
	displayName := "系统管理员"
	now := s.now()
	if _, err := s.users.Create(ctx, user.CreateUser{
		Username:       username,
		Email:          email,
		HashedPassword: hash,
		Role:           user.RoleAdmin,
		DisplayName:    &displayName,
		IsActive:       true,
		Status:         user.StatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		return false, fmt.Errorf("create admin user: %w", err)
	}
	s.logger.Info("admin account created", "username", username)
	return true, nil
}

// SubmitPasswordReset creates a password reset request without revealing whether the account exists.
func (s *Service) SubmitPasswordReset(ctx context.Context, username, email, reason string) (PasswordResetResult, error) {
	account, ok, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		return PasswordResetResult{}, fmt.Errorf("get user by username: %w", err)
	}
	if !ok || account.Email != email {
		return PasswordResetResult{
			Success: true,
			Message: "如果该账号存在，您的申请已提交，请等待管理员审批",
		}, nil
	}

	since := s.now().Add(-24 * time.Hour)
	count, err := s.resets.CountPasswordResetRequestsSince(ctx, account.ID, since)
	if err != nil {
		return PasswordResetResult{}, fmt.Errorf("count password reset requests: %w", err)
	}
	if count >= maxPasswordResetRequestsPerDay {
		return PasswordResetResult{Success: false, Message: "申请过于频繁，请 24 小时后再试"}, nil
	}
	pending, err := s.resets.HasPendingPasswordReset(ctx, account.ID)
	if err != nil {
		return PasswordResetResult{}, fmt.Errorf("check pending password reset: %w", err)
	}
	if pending {
		return PasswordResetResult{Success: true, Message: "您已有待处理申请，请耐心等待管理员审批"}, nil
	}

	requestID, err := s.resets.CreatePasswordResetRequest(ctx, PasswordResetRequest{
		UserID:    account.ID,
		Username:  account.Username,
		Email:     account.Email,
		Reason:    reason,
		CreatedAt: s.now(),
	})
	if err != nil {
		return PasswordResetResult{}, fmt.Errorf("create password reset request: %w", err)
	}
	return PasswordResetResult{
		Success:   true,
		Message:   "申请已提交，请等待管理员审批",
		RequestID: &requestID,
	}, nil
}

// PasswordResetStatus returns the latest matching reset request for public polling.
func (s *Service) PasswordResetStatus(ctx context.Context, username, email string) (PasswordResetStatus, error) {
	status, ok, err := s.resets.LatestPasswordResetRequestStatus(ctx, username, email)
	if err != nil {
		return PasswordResetStatus{}, err
	}
	if !ok {
		return PasswordResetStatus{HasPending: false}, nil
	}
	return status, nil
}

func (s *Service) tokensForUser(account user.User) (AuthResult, error) {
	accessToken, err := s.tokens.CreateAccessToken(account.ID, account.Role)
	if err != nil {
		return AuthResult{}, fmt.Errorf("create access token: %w", err)
	}
	refreshToken, err := s.tokens.CreateRefreshToken(account.ID)
	if err != nil {
		return AuthResult{}, fmt.Errorf("create refresh token: %w", err)
	}
	return AuthResult{
		Success:      true,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         account,
	}, nil
}
