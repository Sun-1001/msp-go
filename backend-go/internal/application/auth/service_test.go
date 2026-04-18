package auth

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"mathstudy/backend-go/internal/domain/user"
)

func TestServiceAuthenticateReturnsTokensAndPrincipal(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository(t)
	service := newTestService(t, repo)

	result, err := service.Authenticate(ctx, "teacher", "Strong1!")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if !result.Success || result.AccessToken == "" || result.RefreshToken == "" {
		t.Fatalf("Authenticate() = %#v", result)
	}
	principal, ok := service.DecodeAccessToken(result.AccessToken)
	if !ok {
		t.Fatal("DecodeAccessToken() ok = false")
	}
	if principal.UserID != "teacher-id" || principal.Role != user.RoleTeacher {
		t.Fatalf("principal = %#v", principal)
	}
}

func TestServiceRegisterChecksSettingsPasswordAndDuplicates(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository(t)
	service := newTestService(t, repo)

	repo.settings.AllowTeacher = false
	result, err := service.Register(ctx, "newteacher", "newteacher@example.com", "Strong1!", "teacher")
	if err != nil {
		t.Fatalf("Register(disallowed) error = %v", err)
	}
	if result.Success || result.Error != "当前不允许教师注册" {
		t.Fatalf("Register(disallowed) = %#v", result)
	}

	repo.settings.AllowTeacher = true
	result, err = service.Register(ctx, "teacher", "other@example.com", "Strong1!", "teacher")
	if err != nil {
		t.Fatalf("Register(duplicate) error = %v", err)
	}
	if result.Success || result.Error != "用户名已存在" {
		t.Fatalf("Register(duplicate) = %#v", result)
	}

	result, err = service.Register(ctx, "student2", "student2@example.com", "weak", "student")
	if err != nil {
		t.Fatalf("Register(weak) error = %v", err)
	}
	if result.Success || result.Error == "" {
		t.Fatalf("Register(weak) = %#v", result)
	}
}

func TestServiceChangePasswordUpdatesHashAndClearsFailures(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository(t)
	service := newTestService(t, repo)

	ok, message, err := service.ChangePassword(ctx, "teacher-id", "bad", "Better1!")
	if err != nil {
		t.Fatalf("ChangePassword(wrong old) error = %v", err)
	}
	if ok || message != "原密码错误" {
		t.Fatalf("ChangePassword(wrong old) = %t/%q", ok, message)
	}

	ok, message, err = service.ChangePassword(ctx, "teacher-id", "Strong1!", "Better1!")
	if err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if !ok || message != "密码修改成功" {
		t.Fatalf("ChangePassword() = %t/%q", ok, message)
	}
	account := repo.usersByID["teacher-id"]
	if !VerifyPassword("Better1!", account.HashedPassword) {
		t.Fatal("password hash was not updated")
	}
}

func TestServiceInitAdminCreatesOnlyWhenMissing(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository(t)
	service := newTestService(t, repo)

	created, err := service.InitAdmin(ctx, "admin", "admin@example.com", "Admin1!")
	if err != nil {
		t.Fatalf("InitAdmin() error = %v", err)
	}
	if !created {
		t.Fatal("InitAdmin() created = false, want true")
	}
	created, err = service.InitAdmin(ctx, "admin", "admin@example.com", "Admin1!")
	if err != nil {
		t.Fatalf("InitAdmin(second) error = %v", err)
	}
	if created {
		t.Fatal("InitAdmin(second) created = true, want false")
	}
	if repo.usersByUsername["admin"].Role != user.RoleAdmin {
		t.Fatalf("admin role = %q", repo.usersByUsername["admin"].Role)
	}
}

func TestServicePasswordResetDoesNotLeakMissingUserAndCreatesRequest(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository(t)
	service := newTestService(t, repo)

	result, err := service.SubmitPasswordReset(ctx, "missing", "missing@example.com", "")
	if err != nil {
		t.Fatalf("SubmitPasswordReset(missing) error = %v", err)
	}
	if !result.Success || result.RequestID != nil {
		t.Fatalf("SubmitPasswordReset(missing) = %#v", result)
	}

	result, err = service.SubmitPasswordReset(ctx, "teacher", "teacher@example.com", "forgot")
	if err != nil {
		t.Fatalf("SubmitPasswordReset() error = %v", err)
	}
	if !result.Success || result.RequestID == nil {
		t.Fatalf("SubmitPasswordReset() = %#v", result)
	}
	status, err := service.PasswordResetStatus(ctx, "teacher", "teacher@example.com")
	if err != nil {
		t.Fatalf("PasswordResetStatus() error = %v", err)
	}
	if !status.HasPending || status.Status == nil || *status.Status != "pending" {
		t.Fatalf("PasswordResetStatus() = %#v", status)
	}
}

type fakeRepository struct {
	usersByID       map[string]user.User
	usersByUsername map[string]user.User
	settings        RegistrationSettings
	resets          []fakeReset
}

type fakeReset struct {
	ID        string
	UserID    string
	Username  string
	Email     string
	Status    string
	CreatedAt time.Time
}

func newFakeRepository(t *testing.T) *fakeRepository {
	t.Helper()
	hash, err := HashPassword("Strong1!")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	repo := &fakeRepository{
		usersByID:       map[string]user.User{},
		usersByUsername: map[string]user.User{},
		settings:        RegistrationSettings{AllowStudent: true, AllowTeacher: true},
	}
	repo.store(user.User{
		ID:             "teacher-id",
		Username:       "teacher",
		Email:          "teacher@example.com",
		HashedPassword: hash,
		Role:           user.RoleTeacher,
		IsActive:       true,
		Status:         user.StatusActive,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	})
	return repo
}

func newTestService(t *testing.T, repo *fakeRepository) *Service {
	t.Helper()
	now := time.Unix(1_700_000_000, 0).UTC()
	tokens, err := newTokenServiceWithClock("secret", "HS256", 30*time.Minute, 7*24*time.Hour, func() time.Time {
		return now
	})
	if err != nil {
		t.Fatalf("newTokenServiceWithClock() error = %v", err)
	}
	service, err := NewService(repo, repo, repo, tokens, nil, slog.Default())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	service.now = func() time.Time { return now }
	return service
}

func (r *fakeRepository) GetByUsername(_ context.Context, username string) (user.User, bool, error) {
	account, ok := r.usersByUsername[username]
	return account, ok, nil
}

func (r *fakeRepository) GetByEmail(_ context.Context, email string) (user.User, bool, error) {
	for _, account := range r.usersByID {
		if account.Email == email {
			return account, true, nil
		}
	}
	return user.User{}, false, nil
}

func (r *fakeRepository) GetByID(_ context.Context, id string) (user.User, bool, error) {
	account, ok := r.usersByID[id]
	return account, ok, nil
}

func (r *fakeRepository) Create(_ context.Context, input user.CreateUser) (user.User, error) {
	id := input.ID
	if id == "" {
		id = input.Username + "-id"
	}
	account := user.User{
		ID:             id,
		Username:       input.Username,
		Email:          input.Email,
		HashedPassword: input.HashedPassword,
		Role:           input.Role,
		DisplayName:    input.DisplayName,
		IsActive:       input.IsActive,
		Status:         input.Status,
		CreatedAt:      input.CreatedAt,
		UpdatedAt:      input.UpdatedAt,
	}
	r.store(account)
	return account, nil
}

func (r *fakeRepository) UpdatePassword(_ context.Context, id string, hash string) error {
	account := r.usersByID[id]
	account.HashedPassword = hash
	r.store(account)
	return nil
}

func (r *fakeRepository) RegistrationSettings(context.Context) (RegistrationSettings, error) {
	return r.settings, nil
}

func (r *fakeRepository) CountPasswordResetRequestsSince(_ context.Context, userID string, since time.Time) (int, error) {
	count := 0
	for _, reset := range r.resets {
		if reset.UserID == userID && !reset.CreatedAt.Before(since) {
			count++
		}
	}
	return count, nil
}

func (r *fakeRepository) HasPendingPasswordReset(_ context.Context, userID string) (bool, error) {
	for _, reset := range r.resets {
		if reset.UserID == userID && reset.Status == "pending" {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeRepository) CreatePasswordResetRequest(_ context.Context, request PasswordResetRequest) (string, error) {
	id := "reset-1"
	r.resets = append(r.resets, fakeReset{
		ID:        id,
		UserID:    request.UserID,
		Username:  request.Username,
		Email:     request.Email,
		Status:    "pending",
		CreatedAt: request.CreatedAt,
	})
	return id, nil
}

func (r *fakeRepository) LatestPasswordResetRequestStatus(_ context.Context, username, email string) (PasswordResetStatus, bool, error) {
	for i := len(r.resets) - 1; i >= 0; i-- {
		reset := r.resets[i]
		if reset.Username == username && reset.Email == email {
			status := reset.Status
			createdAt := reset.CreatedAt
			return PasswordResetStatus{
				HasPending: status == "pending",
				Status:     &status,
				CreatedAt:  &createdAt,
			}, true, nil
		}
	}
	return PasswordResetStatus{}, false, nil
}

func (r *fakeRepository) store(account user.User) {
	r.usersByID[account.ID] = account
	r.usersByUsername[account.Username] = account
}
