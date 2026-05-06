package adminsettingshttp

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	adminsettingsapp "mathstudy/backend-go/internal/application/adminsettings"
	authapp "mathstudy/backend-go/internal/application/auth"
	"mathstudy/backend-go/internal/domain/user"
)

func TestRequiresAdmin(t *testing.T) {
	handler := newAdminSettingsTestHandler(t, &fakeSettingsService{}, &fakeAuthenticator{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/settings")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings/registration", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}

	handler = newAdminSettingsTestHandler(t, &fakeSettingsService{}, &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}})
	mux = http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/settings")
	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings/registration", nil)
	request.Header.Set("Authorization", "Bearer token")
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestGeneralAndRegistrationRoutes(t *testing.T) {
	service := &fakeSettingsService{
		registration: adminsettingsapp.RegistrationSettingsResponse{AllowStudent: true, AllowTeacher: false},
		general:      adminsettingsapp.GeneralSettingsResponse{SystemName: "系统", SystemVersion: "v1"},
	}
	handler := newAdminSettingsTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/settings")

	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/registration", bytes.NewBufferString(`{"allow_student":false,"allow_teacher":true}`))
	request.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastAllowStudent || !service.lastAllowTeacher {
		t.Fatalf("status=%d student=%v teacher=%v", recorder.Code, service.lastAllowStudent, service.lastAllowTeacher)
	}

	request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/general", bytes.NewBufferString(`{"system_name":"新系统","system_description":"描述"}`))
	request.Header.Set("Authorization", "Bearer token")
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastSystemName != "新系统" || service.lastSystemDescription != "描述" {
		t.Fatalf("status=%d name=%q desc=%q", recorder.Code, service.lastSystemName, service.lastSystemDescription)
	}
}

func TestDatabaseRoutes(t *testing.T) {
	service := &fakeSettingsService{
		tables:  adminsettingsapp.ExportableTablesResponse{Tables: []adminsettingsapp.ExportableTableItem{{Name: "users", DisplayName: "用户"}}},
		export:  adminsettingsapp.DataExportResponse{Filename: "backup.json"},
		import_: adminsettingsapp.DataImportResponse{Success: true},
		monitor: adminsettingsapp.DatabaseMonitorResponse{HealthStatus: "healthy"},
	}
	handler := newAdminSettingsTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/settings")

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/settings/database/export", bytes.NewBufferString(`{"tables":["users"]}`))
	request.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || len(service.lastTables) != 1 || service.lastAdminID != "admin-1" {
		t.Fatalf("status=%d tables=%#v admin=%q", recorder.Code, service.lastTables, service.lastAdminID)
	}

	body, contentType := multipartBody(t, "backup.json", `{"tables":{"users":[]}}`)
	request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/settings/database/import", body)
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("Content-Type", contentType)
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || string(service.lastImportContent) == "" {
		t.Fatalf("status=%d import=%s body=%s", recorder.Code, service.lastImportContent, recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings/database/monitor", nil)
	request.Header.Set("Authorization", "Bearer token")
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestValidationAndServiceErrors(t *testing.T) {
	service := &fakeSettingsService{err: adminsettingsapp.Error{Kind: adminsettingsapp.ErrBadRequest, Message: "不支持导出的表: bad"}}
	handler := newAdminSettingsTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/settings")

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/settings/database/export", bytes.NewBufferString(`{"tables":["bad"]}`))
	request.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	service.err = nil
	body, contentType := multipartBody(t, "backup.txt", `{}`)
	request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/settings/database/import", body)
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("Content-Type", contentType)
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestNewHandlerRejectsMissingDependencies(t *testing.T) {
	if _, err := NewHandler(nil, nil, &fakeAuthenticator{}); err == nil {
		t.Fatal("NewHandler(nil service) error = nil, want error")
	}
	if _, err := NewHandler(nil, &fakeSettingsService{}, nil); err == nil {
		t.Fatal("NewHandler(nil auth) error = nil, want error")
	}
}

func newAdminSettingsTestHandler(t *testing.T, service Service, auth Authenticator) *Handler {
	t.Helper()
	handler, err := NewHandler(slog.New(slog.NewTextHandler(os.Stdout, nil)), service, auth)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return handler
}

func multipartBody(t *testing.T, filename string, content string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	_, _ = part.Write([]byte(content))
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return body, writer.FormDataContentType()
}

func adminAuthenticator() *fakeAuthenticator {
	return &fakeAuthenticator{principal: authapp.Principal{UserID: "admin-1", Role: user.RoleAdmin}}
}

type fakeAuthenticator struct {
	principal authapp.Principal
}

func (a *fakeAuthenticator) DecodeAccessToken(string) (authapp.Principal, bool) {
	return a.principal, a.principal.UserID != ""
}

type fakeSettingsService struct {
	registration          adminsettingsapp.RegistrationSettingsResponse
	general               adminsettingsapp.GeneralSettingsResponse
	tables                adminsettingsapp.ExportableTablesResponse
	export                adminsettingsapp.DataExportResponse
	import_               adminsettingsapp.DataImportResponse
	monitor               adminsettingsapp.DatabaseMonitorResponse
	err                   error
	lastAllowStudent      bool
	lastAllowTeacher      bool
	lastSystemName        string
	lastSystemDescription string
	lastTables            []string
	lastAdminID           string
	lastImportContent     []byte
}

func (s *fakeSettingsService) RegistrationSettings(context.Context) (adminsettingsapp.RegistrationSettingsResponse, error) {
	return s.registration, s.err
}

func (s *fakeSettingsService) UpdateRegistrationSettings(_ context.Context, allowStudent bool, allowTeacher bool) (adminsettingsapp.RegistrationSettingsResponse, error) {
	s.lastAllowStudent = allowStudent
	s.lastAllowTeacher = allowTeacher
	return adminsettingsapp.RegistrationSettingsResponse{AllowStudent: allowStudent, AllowTeacher: allowTeacher}, s.err
}

func (s *fakeSettingsService) GeneralSettings(context.Context) (adminsettingsapp.GeneralSettingsResponse, error) {
	return s.general, s.err
}

func (s *fakeSettingsService) UpdateGeneralSettings(_ context.Context, name string, description string) (adminsettingsapp.GeneralSettingsResponse, error) {
	s.lastSystemName = name
	s.lastSystemDescription = description
	return adminsettingsapp.GeneralSettingsResponse{SystemName: name, SystemDescription: description}, s.err
}

func (s *fakeSettingsService) ExportableTables(context.Context) (adminsettingsapp.ExportableTablesResponse, error) {
	return s.tables, s.err
}

func (s *fakeSettingsService) ExportData(_ context.Context, tables []string, adminID string) (adminsettingsapp.DataExportResponse, error) {
	s.lastTables = tables
	s.lastAdminID = adminID
	if s.err != nil {
		return adminsettingsapp.DataExportResponse{}, s.err
	}
	return s.export, nil
}

func (s *fakeSettingsService) ImportData(_ context.Context, content []byte, adminID string) (adminsettingsapp.DataImportResponse, error) {
	s.lastImportContent = content
	s.lastAdminID = adminID
	return s.import_, s.err
}

func (s *fakeSettingsService) DatabaseMonitor(context.Context) (adminsettingsapp.DatabaseMonitorResponse, error) {
	if s.err != nil && !errors.Is(s.err, adminsettingsapp.ErrBadRequest) {
		return adminsettingsapp.DatabaseMonitorResponse{}, s.err
	}
	return s.monitor, nil
}
