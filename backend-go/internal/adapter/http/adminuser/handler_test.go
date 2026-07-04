package adminuserhttp

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	adminuserapp "mathstudy/backend-go/internal/application/adminuser"
	authapp "mathstudy/backend-go/internal/application/auth"
	"mathstudy/backend-go/internal/domain/user"
)

func TestAdminUserRoutesRequireAdmin(t *testing.T) {
	handler := newAdminUserTestHandler(t, &fakeAdminUserService{}, &fakeAuthenticator{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/users")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/stats", nil)
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", got)
	}

	handler = newAdminUserTestHandler(t, &fakeAdminUserService{}, &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}})
	mux = http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/users")
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/stats", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestListUsersParsesFilters(t *testing.T) {
	service := &fakeAdminUserService{listResponse: adminuserapp.ListResponse{Items: []adminuserapp.UserItem{}, Page: 2, PageSize: 25}}
	handler := newAdminUserTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/users")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=2&page_size=25&search=alice&role=student&status=active", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.lastFilter.Page != 2 || service.lastFilter.PageSize != 25 || service.lastFilter.Search != "alice" || service.lastFilter.Role != "student" {
		t.Fatalf("filter = %#v", service.lastFilter)
	}
}

func TestListUsersRejectsInvalidPagination(t *testing.T) {
	service := &fakeAdminUserService{}
	handler := newAdminUserTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/users")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=bad", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "page 必须是整数") || !strings.Contains(recorder.Body.String(), "VALIDATION_ERROR") {
		t.Fatalf("body = %s", recorder.Body.String())
	}
	if service.lastFilter.Page != 0 {
		t.Fatalf("service was called for invalid pagination: %#v", service.lastFilter)
	}
}

func TestCreateUpdateStatusAndDeleteForwardToService(t *testing.T) {
	service := &fakeAdminUserService{
		createResponse: adminuserapp.CreateResponse{Success: true, Message: "用户创建成功"},
		updateResponse: adminuserapp.UpdateResponse{Success: true, Message: "用户已停用", User: adminuserapp.UserItem{ID: "user-1"}},
		deleteResponse: adminuserapp.DeleteResponse{Success: true, Message: "用户已删除"},
	}
	handler := newAdminUserTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/users")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewBufferString(`{"username":"new","email":"new@example.com","password":"Strong1!","role":"student"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastCreate.Username != "new" {
		t.Fatalf("create status=%d create=%#v body=%s", recorder.Code, service.lastCreate, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/user-1/status", bytes.NewBufferString(`{"status":"suspended"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastUserID != "user-1" || service.lastStatus != "suspended" {
		t.Fatalf("status=%d user=%q statusValue=%q", recorder.Code, service.lastUserID, service.lastStatus)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/user-1", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastDeletedID != "user-1" {
		t.Fatalf("delete status=%d deleted=%q", recorder.Code, service.lastDeletedID)
	}
}

func TestJSONRoutesRejectTrailingJSON(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		assert func(*testing.T, *fakeAdminUserService)
	}{
		{
			name:   "create user",
			method: http.MethodPost,
			path:   "/api/v1/admin/users",
			body:   `{"username":"new","email":"new@example.com","password":"Strong1!","role":"student"} {"role":"admin"}`,
			assert: func(t *testing.T, service *fakeAdminUserService) {
				t.Helper()
				if service.lastCreate.Username != "" {
					t.Fatalf("service was called for create trailing JSON: %#v", service.lastCreate)
				}
			},
		},
		{
			name:   "update status",
			method: http.MethodPatch,
			path:   "/api/v1/admin/users/user-1/status",
			body:   `{"status":"suspended"} {"status":"active"}`,
			assert: func(t *testing.T, service *fakeAdminUserService) {
				t.Helper()
				if service.lastUserID != "" || service.lastStatus != "" {
					t.Fatalf("service was called for status trailing JSON: user=%q status=%q", service.lastUserID, service.lastStatus)
				}
			},
		},
		{
			name:   "update user",
			method: http.MethodPut,
			path:   "/api/v1/admin/users/user-1",
			body:   `{"display_name":"Alice","password":"Strong1!"} {"password":"extra"}`,
			assert: func(t *testing.T, service *fakeAdminUserService) {
				t.Helper()
				if service.lastUserID != "" {
					t.Fatalf("service was called for update trailing JSON: user=%q update=%#v", service.lastUserID, service.lastUpdate)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeAdminUserService{}
			handler := newAdminUserTestHandler(t, service, adminAuthenticator())
			mux := http.NewServeMux()
			handler.Register(mux, "/api/v1/admin/users")

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			request.Header.Set("Authorization", "Bearer token")
			mux.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), "请求体格式错误") || !strings.Contains(recorder.Body.String(), "VALIDATION_ERROR") {
				t.Fatalf("body=%s", recorder.Body.String())
			}
			tt.assert(t, service)
		})
	}
}

func TestImportUsersParsesMultipartCSV(t *testing.T) {
	service := &fakeAdminUserService{importResponse: adminuserapp.ImportResponse{Success: true, Total: 1, Created: 1, Details: []adminuserapp.ImportResult{{Row: 1, Username: "alice", Success: true}}}}
	handler := newAdminUserTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/users")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "users.csv")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	_, _ = part.Write([]byte("用户名,邮箱,密码,角色,显示名称\nalice,alice@example.com,Strong1!,student,Alice\n"))
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/import", body)
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("Content-Type", writer.FormDataContentType())
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(service.lastImport) != 1 || service.lastImport[0].Username != "alice" || service.lastImport[0].DisplayName == nil || *service.lastImport[0].DisplayName != "Alice" {
		t.Fatalf("import rows = %#v", service.lastImport)
	}
}

func TestParseImportCSVRejectsBoundaryAbuse(t *testing.T) {
	t.Run("too many rows", func(t *testing.T) {
		var builder strings.Builder
		builder.WriteString("用户名,邮箱,密码,角色\n")
		for index := 0; index < maxImportRows+1; index++ {
			builder.WriteString("alice,alice@example.com,Strong1!,student\n")
		}
		if _, err := parseImportCSV(builder.String()); err == nil || !strings.Contains(err.Error(), "行数") {
			t.Fatalf("parseImportCSV() error = %v, want row limit", err)
		}
	})

	t.Run("too many columns", func(t *testing.T) {
		header := strings.Repeat("额外列,", maxImportColumns) + "额外列\n"
		row := strings.Repeat("value,", maxImportColumns) + "value\n"
		if _, err := parseImportCSV(header + row); err == nil || !strings.Contains(err.Error(), "列数") {
			t.Fatalf("parseImportCSV() error = %v, want column limit", err)
		}
	})

	t.Run("field too long", func(t *testing.T) {
		content := "用户名,邮箱,密码,角色\n" + strings.Repeat("a", maxImportFieldRunes+1) + ",alice@example.com,Strong1!,student\n"
		if _, err := parseImportCSV(content); err == nil || !strings.Contains(err.Error(), "字段长度") {
			t.Fatalf("parseImportCSV() error = %v, want field length limit", err)
		}
	})
}

func TestExportUsersWritesCSV(t *testing.T) {
	service := &fakeAdminUserService{exportUsers: []adminuserapp.ExportUser{{
		Username:    "=cmd|'/C calc'!A0",
		Email:       "student@example.com",
		DisplayName: "+Student",
		Role:        "student",
		Status:      "active",
		CreatedAt:   time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
	}}}
	handler := newAdminUserTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/users")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/export?role=student", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Disposition"); !strings.Contains(got, "users_export.csv") {
		t.Fatalf("Content-Disposition = %q", got)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "用户名") || !strings.Contains(body, "student@example.com") {
		t.Fatalf("csv body = %q", body)
	}
	if !strings.Contains(body, "'=cmd|'/C calc'!A0") || !strings.Contains(body, "'+Student") {
		t.Fatalf("csv formula fields were not escaped: %q", body)
	}
	if service.lastFilter.Role != "student" {
		t.Fatalf("filter = %#v", service.lastFilter)
	}
}

func TestServiceErrorsMapToStatusCodes(t *testing.T) {
	service := &fakeAdminUserService{err: adminuserapp.Error{Kind: adminuserapp.ErrNotFound, Message: "用户不存在"}}
	handler := newAdminUserTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/users")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/missing", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestServiceErrorsRedactPublicMessages(t *testing.T) {
	service := &fakeAdminUserService{err: adminuserapp.Error{Kind: adminuserapp.ErrBadRequest, Message: "无效输入 Authorization: Bearer import-token api_key=plain"}}
	handler := newAdminUserTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/users")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertNoAdminUserCredentialLeak(t, recorder.Body.String())
}

func TestInternalErrorsRedactLogs(t *testing.T) {
	var logBuffer bytes.Buffer
	service := &fakeAdminUserService{err: errors.New("db failed Authorization: Bearer import-token token=query-token api_key=plain")}
	handler, err := NewHandler(slog.New(slog.NewTextHandler(&logBuffer, nil)), service, adminAuthenticator())
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/users")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/stats", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertNoAdminUserCredentialLeak(t, recorder.Body.String())
	assertNoAdminUserCredentialLeak(t, logBuffer.String())
}

func TestNewHandlerRejectsMissingDependencies(t *testing.T) {
	if _, err := NewHandler(nil, nil, &fakeAuthenticator{}); err == nil {
		t.Fatal("NewHandler(nil service) error = nil, want error")
	}
	if _, err := NewHandler(nil, &fakeAdminUserService{}, nil); err == nil {
		t.Fatal("NewHandler(nil auth) error = nil, want error")
	}
}

func newAdminUserTestHandler(t *testing.T, service Service, auth Authenticator) *Handler {
	t.Helper()
	handler, err := NewHandler(slog.New(slog.NewTextHandler(os.Stdout, nil)), service, auth)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return handler
}

func assertNoAdminUserCredentialLeak(t *testing.T, value string) {
	t.Helper()
	for _, leaked := range []string{"import-token", "token=query-token", "api_key=plain", "Bearer import-token"} {
		if strings.Contains(value, leaked) {
			t.Fatalf("value leaked %q in %q", leaked, value)
		}
	}
}

func adminAuthenticator() *fakeAuthenticator {
	return &fakeAuthenticator{principal: authapp.Principal{UserID: "admin-1", Role: user.RoleAdmin}}
}

type fakeAuthenticator struct {
	principal authapp.Principal
}

func (a *fakeAuthenticator) DecodeAccessToken(string) (authapp.Principal, bool) {
	if a.principal.UserID == "" {
		return authapp.Principal{}, false
	}
	return a.principal, true
}

type fakeAdminUserService struct {
	stats          adminuserapp.AccountStats
	listResponse   adminuserapp.ListResponse
	createResponse adminuserapp.CreateResponse
	updateResponse adminuserapp.UpdateResponse
	deleteResponse adminuserapp.DeleteResponse
	importResponse adminuserapp.ImportResponse
	exportUsers    []adminuserapp.ExportUser
	err            error
	lastFilter     adminuserapp.ListFilter
	lastCreate     adminuserapp.Create
	lastUserID     string
	lastStatus     string
	lastUpdate     adminuserapp.Update
	lastDeletedID  string
	lastImport     []adminuserapp.ImportUser
}

func (s *fakeAdminUserService) AccountStats(context.Context) (adminuserapp.AccountStats, error) {
	return s.stats, s.err
}

func (s *fakeAdminUserService) ListUsers(_ context.Context, filter adminuserapp.ListFilter) (adminuserapp.ListResponse, error) {
	s.lastFilter = filter
	return s.listResponse, s.err
}

func (s *fakeAdminUserService) UpdateUserStatus(_ context.Context, userID string, status string) (adminuserapp.UpdateResponse, error) {
	s.lastUserID = userID
	s.lastStatus = status
	return s.updateResponse, s.err
}

func (s *fakeAdminUserService) UpdateUser(_ context.Context, userID string, update adminuserapp.Update) (adminuserapp.UpdateResponse, error) {
	s.lastUserID = userID
	s.lastUpdate = update
	return s.updateResponse, s.err
}

func (s *fakeAdminUserService) DeleteUser(_ context.Context, userID string) (adminuserapp.DeleteResponse, error) {
	s.lastDeletedID = userID
	return s.deleteResponse, s.err
}

func (s *fakeAdminUserService) CreateUser(_ context.Context, input adminuserapp.Create) (adminuserapp.CreateResponse, error) {
	s.lastCreate = input
	return s.createResponse, s.err
}

func (s *fakeAdminUserService) ExportUsers(_ context.Context, filter adminuserapp.ListFilter) ([]adminuserapp.ExportUser, error) {
	s.lastFilter = filter
	return s.exportUsers, s.err
}

func (s *fakeAdminUserService) ImportUsers(_ context.Context, users []adminuserapp.ImportUser) (adminuserapp.ImportResponse, error) {
	s.lastImport = users
	return s.importResponse, s.err
}
