package adminuser

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	authapp "mathstudy/backend-go/internal/application/auth"
	"mathstudy/backend-go/internal/domain/user"
)

func TestNewServiceRejectsNilRepository(t *testing.T) {
	if _, err := NewService(nil); err == nil {
		t.Fatal("NewService(nil) error = nil, want error")
	}
}

func TestListUsersNormalizesPaginationAndFilters(t *testing.T) {
	repo := &fakeRepository{
		listItems: []UserItem{{ID: "user-1", Username: "student", Role: user.RoleStudent, Status: user.StatusActive}},
		listTotal: 21,
	}
	service := newTestService(t, repo)

	response, err := service.ListUsers(context.Background(), ListFilter{Page: 2, PageSize: 10, Search: " alice ", Role: "student", Status: "active"})
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if response.TotalPages != 3 || response.Page != 2 || response.PageSize != 10 {
		t.Fatalf("response pagination = %#v", response)
	}
	if repo.lastFilter.Search != "alice" || repo.lastFilter.Role != "student" || repo.lastFilter.Status != "active" {
		t.Fatalf("filter = %#v", repo.lastFilter)
	}

	_, err = service.ListUsers(context.Background(), ListFilter{Page: 0, PageSize: 101})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("ListUsers(invalid) error = %v, want ErrBadRequest", err)
	}
}

func TestCreateUserHandlesDuplicateAndSuccess(t *testing.T) {
	existing := user.User{ID: "existing-1", Username: "taken", Email: "taken@example.com", Role: user.RoleStudent, Status: user.StatusActive, CreatedAt: time.Now()}
	repo := &fakeRepository{usersByUsername: map[string]user.User{"taken": existing}}
	service := newTestService(t, repo)

	response, err := service.CreateUser(context.Background(), Create{Username: "taken", Email: "new@example.com", Password: "Strong1!", Role: "student"})
	if err != nil {
		t.Fatalf("CreateUser(duplicate) error = %v", err)
	}
	if response.Success || response.Message != "用户名 'taken' 已存在" || response.User != nil {
		t.Fatalf("duplicate response = %#v", response)
	}

	displayName := "Teacher One"
	response, err = service.CreateUser(context.Background(), Create{
		Username:    "teacher",
		Email:       "teacher@example.com",
		Password:    "Strong1!",
		Role:        "teacher",
		DisplayName: &displayName,
	})
	if err != nil {
		t.Fatalf("CreateUser(success) error = %v", err)
	}
	if !response.Success || response.User == nil || response.User.Role != user.RoleTeacher {
		t.Fatalf("success response = %#v", response)
	}
	if len(repo.created) != 1 || !authapp.VerifyPassword("Strong1!", repo.created[0].HashedPassword) {
		t.Fatalf("created input = %#v", repo.created)
	}

	response, err = service.CreateUser(context.Background(), Create{Username: "default-role", Email: "default-role@example.com", Password: "Strong1!", Role: "   "})
	if err != nil {
		t.Fatalf("CreateUser(default role) error = %v", err)
	}
	if !response.Success || response.User == nil || response.User.Role != user.RoleStudent || len(repo.created) != 2 || repo.created[1].Role != user.RoleStudent {
		t.Fatalf("default role response=%#v created=%#v", response, repo.created)
	}
}

func TestUpdateUserStatusMapsMessages(t *testing.T) {
	repo := &fakeRepository{updateStatusUser: user.User{ID: "user-1", Username: "student", Role: user.RoleStudent, Status: user.StatusSuspended}}
	service := newTestService(t, repo)

	response, err := service.UpdateUserStatus(context.Background(), "user-1", "suspended")
	if err != nil {
		t.Fatalf("UpdateUserStatus() error = %v", err)
	}
	if response.Message != "用户已停用" || repo.lastStatus != user.StatusSuspended {
		t.Fatalf("response=%#v status=%q", response, repo.lastStatus)
	}

	_, err = service.UpdateUserStatus(context.Background(), "user-1", "inactive")
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("UpdateUserStatus(invalid) error = %v, want ErrBadRequest", err)
	}
}

func TestImportUsersRecordsCreatedSkippedAndFailedRows(t *testing.T) {
	repo := &fakeRepository{
		usersByUsername: map[string]user.User{"taken": {ID: "user-1", Username: "taken", Email: "taken@example.com", Role: user.RoleStudent, Status: user.StatusActive}},
	}
	service := newTestService(t, repo)
	displayName := "New User"

	response, err := service.ImportUsers(context.Background(), []ImportUser{
		{Username: "", Email: "", Password: ""},
		{Username: "taken", Email: "other@example.com", Password: "Strong1!", Role: "student"},
		{Username: "badrole", Email: "bad@example.com", Password: "Strong1!", Role: "guest"},
		{Username: "new", Email: "new@example.com", Password: "Strong1!", Role: "teacher", DisplayName: &displayName},
		{Username: "defaulted", Email: "defaulted@example.com", Password: "Strong1!", Role: "   "},
	})
	if err != nil {
		t.Fatalf("ImportUsers() error = %v", err)
	}
	if response.Success || response.Total != 5 || response.Created != 2 || response.Skipped != 1 || response.Failed != 2 {
		t.Fatalf("response = %#v", response)
	}
	if len(repo.created) != 2 ||
		repo.created[0].Username != "new" || repo.created[0].Role != user.RoleTeacher ||
		repo.created[1].Username != "defaulted" || repo.created[1].Role != user.RoleStudent {
		t.Fatalf("created = %#v", repo.created)
	}
}

func TestImportUsersRedactsCreateErrors(t *testing.T) {
	repo := &fakeRepository{createErr: errors.New("insert failed Authorization: Bearer import-token api_key=plain password=Strong1!")}
	service := newTestService(t, repo)

	response, err := service.ImportUsers(context.Background(), []ImportUser{
		{Username: "new", Email: "new@example.com", Password: "Strong1!", Role: "student"},
	})
	if err != nil {
		t.Fatalf("ImportUsers() error = %v", err)
	}
	if response.Failed != 1 || len(response.Details) != 1 {
		t.Fatalf("response = %#v", response)
	}
	assertNoAdminUserCredentialLeak(t, response.Details[0].Message)
}

func assertNoAdminUserCredentialLeak(t *testing.T, value string) {
	t.Helper()
	for _, leaked := range []string{"import-token", "api_key=plain", "password=Strong1", "Bearer import-token"} {
		if strings.Contains(value, leaked) {
			t.Fatalf("value leaked %q in %q", leaked, value)
		}
	}
	if !strings.Contains(value, "[REDACTED]") {
		t.Fatalf("value = %q, want redaction marker", value)
	}
}

func newTestService(t *testing.T, repo *fakeRepository) *Service {
	t.Helper()
	if repo.usersByUsername == nil {
		repo.usersByUsername = map[string]user.User{}
	}
	if repo.usersByEmail == nil {
		repo.usersByEmail = map[string]user.User{}
	}
	service, err := NewService(repo)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	service.now = func() time.Time { return time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC) }
	return service
}

type fakeRepository struct {
	stats             AccountStats
	listItems         []UserItem
	listTotal         int
	lastFilter        ListFilter
	usersByUsername   map[string]user.User
	usersByEmail      map[string]user.User
	created           []user.CreateUser
	updateUser        user.User
	updateUserFound   bool
	updateStatusUser  user.User
	updateStatusFound bool
	lastStatus        user.Status
	deleted           bool
	exportUsers       []ExportUser
	createErr         error
}

func (r *fakeRepository) AccountStats(context.Context) (AccountStats, error) {
	return r.stats, nil
}

func (r *fakeRepository) ListUsers(_ context.Context, filter ListFilter) ([]UserItem, int, error) {
	r.lastFilter = filter
	return r.listItems, r.listTotal, nil
}

func (r *fakeRepository) GetByUsername(_ context.Context, username string) (user.User, bool, error) {
	account, ok := r.usersByUsername[username]
	return account, ok, nil
}

func (r *fakeRepository) GetByEmail(_ context.Context, email string) (user.User, bool, error) {
	account, ok := r.usersByEmail[email]
	return account, ok, nil
}

func (r *fakeRepository) Create(_ context.Context, input user.CreateUser) (user.User, error) {
	if r.createErr != nil {
		return user.User{}, r.createErr
	}
	r.created = append(r.created, input)
	account := user.User{
		ID:             input.Username + "-id",
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
	r.usersByUsername[account.Username] = account
	r.usersByEmail[account.Email] = account
	return account, nil
}

func (r *fakeRepository) UpdateUser(context.Context, string, Update, *string, time.Time) (user.User, bool, error) {
	if r.updateUserFound {
		return r.updateUser, true, nil
	}
	return user.User{}, false, nil
}

func (r *fakeRepository) UpdateUserStatus(_ context.Context, _ string, status user.Status, _ time.Time) (user.User, bool, error) {
	r.lastStatus = status
	if r.updateStatusUser.ID != "" {
		return r.updateStatusUser, true, nil
	}
	return user.User{}, false, nil
}

func (r *fakeRepository) DeleteUser(context.Context, string) (bool, error) {
	return r.deleted, nil
}

func (r *fakeRepository) ExportUsers(context.Context, ListFilter) ([]ExportUser, error) {
	return r.exportUsers, nil
}
