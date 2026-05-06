package securityloghttp

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	authapp "mathstudy/backend-go/internal/application/auth"
	securitylogapp "mathstudy/backend-go/internal/application/securitylog"
	"mathstudy/backend-go/internal/domain/user"
)

func TestRequiresAdmin(t *testing.T) {
	handler := newSecurityLogTestHandler(t, &fakeSecurityLogService{}, &fakeAuthenticator{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/security-logs")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/security-logs", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}

	handler = newSecurityLogTestHandler(t, &fakeSecurityLogService{}, &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}})
	mux = http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/security-logs")
	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/security-logs", nil)
	request.Header.Set("Authorization", "Bearer token")
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestListLogsParsesFilters(t *testing.T) {
	service := &fakeSecurityLogService{listResponse: securitylogapp.ListResponse{Total: 1}}
	handler := newSecurityLogTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/security-logs")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/security-logs?event_types=login_failed,service_error&severities=warning&page=2&page_size=25&start_date=2026-05-01&include_archived=true", nil)
	request.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.lastFilter.Page != 2 || service.lastFilter.PageSize != 25 || len(service.lastFilter.EventTypes) != 2 || !service.lastFilter.IncludeArchived || service.lastFilter.StartDate == nil {
		t.Fatalf("filter = %#v", service.lastFilter)
	}
}

func TestMutationRoutesForwardBodies(t *testing.T) {
	service := &fakeSecurityLogService{
		deleteResponse:  map[string]int{"deleted_count": 2},
		exportResponse:  securitylogapp.ExportResponse{Filename: "logs.json"},
		archiveResponse: securitylogapp.ArchiveResponse{ArchivedCount: 3},
		reportResponse:  securitylogapp.DailyReportResponse{Generated: true},
		cleanupResponse: securitylogapp.CleanupResponse{ArchivedCount: 1},
		volumeResponse:  securitylogapp.VolumeResponse{Total: 10},
	}
	handler := newSecurityLogTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/security-logs")

	request := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/security-logs", bytes.NewBufferString(`{"log_ids":["log-1"]}`))
	request.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || len(service.lastDelete.LogIDs) != 1 {
		t.Fatalf("status=%d delete=%#v", recorder.Code, service.lastDelete)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/security-logs/export", bytes.NewBufferString(`{"format":"csv","severities":["error"]}`))
	request.Header.Set("Authorization", "Bearer token")
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastExport.Format != "csv" || len(service.lastExport.Severities) != 1 {
		t.Fatalf("status=%d export=%#v", recorder.Code, service.lastExport)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/security-logs/archive", bytes.NewBufferString(`{"before_date":"2026-05-01T00:00:00Z"}`))
	request.Header.Set("Authorization", "Bearer token")
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastArchive.IsZero() {
		t.Fatalf("status=%d archive=%s", recorder.Code, service.lastArchive)
	}

	for _, item := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/admin/security-logs/generate-daily-report"},
		{http.MethodPost, "/api/v1/admin/security-logs/cleanup"},
		{http.MethodGet, "/api/v1/admin/security-logs/volume"},
	} {
		request = httptest.NewRequest(item.method, item.path, nil)
		request.Header.Set("Authorization", "Bearer token")
		recorder = httptest.NewRecorder()
		mux.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s %s status=%d body=%s", item.method, item.path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestValidationAndServiceErrors(t *testing.T) {
	service := &fakeSecurityLogService{err: securitylogapp.Error{Kind: securitylogapp.ErrBadRequest, Message: "format 必须是 json 或 csv"}}
	handler := newSecurityLogTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/security-logs")

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/security-logs/export", bytes.NewBufferString(`{"format":"xml"}`))
	request.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	service.err = nil
	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/security-logs?page=bad", nil)
	request.Header.Set("Authorization", "Bearer token")
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestNewHandlerRejectsMissingDependencies(t *testing.T) {
	if _, err := NewHandler(nil, nil, &fakeAuthenticator{}); err == nil {
		t.Fatal("NewHandler(nil service) error = nil, want error")
	}
	if _, err := NewHandler(nil, &fakeSecurityLogService{}, nil); err == nil {
		t.Fatal("NewHandler(nil auth) error = nil, want error")
	}
}

func newSecurityLogTestHandler(t *testing.T, service Service, auth Authenticator) *Handler {
	t.Helper()
	handler, err := NewHandler(slog.New(slog.NewTextHandler(os.Stdout, nil)), service, auth)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return handler
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

type fakeSecurityLogService struct {
	listResponse    securitylogapp.ListResponse
	statsResponse   securitylogapp.StatsResponse
	deleteResponse  map[string]int
	exportResponse  securitylogapp.ExportResponse
	archiveResponse securitylogapp.ArchiveResponse
	reportResponse  securitylogapp.DailyReportResponse
	cleanupResponse securitylogapp.CleanupResponse
	volumeResponse  securitylogapp.VolumeResponse
	err             error
	lastFilter      securitylogapp.QueryFilter
	lastDelete      securitylogapp.DeleteRequest
	lastExport      securitylogapp.ExportRequest
	lastArchive     time.Time
}

func (s *fakeSecurityLogService) ListLogs(_ context.Context, filter securitylogapp.QueryFilter) (securitylogapp.ListResponse, error) {
	s.lastFilter = filter
	if s.err != nil && !errors.Is(s.err, securitylogapp.ErrBadRequest) {
		return securitylogapp.ListResponse{}, s.err
	}
	return s.listResponse, nil
}

func (s *fakeSecurityLogService) Stats(context.Context) (securitylogapp.StatsResponse, error) {
	return s.statsResponse, s.err
}

func (s *fakeSecurityLogService) DeleteLogs(_ context.Context, request securitylogapp.DeleteRequest) (map[string]int, error) {
	s.lastDelete = request
	return s.deleteResponse, s.err
}

func (s *fakeSecurityLogService) ExportLogs(_ context.Context, request securitylogapp.ExportRequest) (securitylogapp.ExportResponse, error) {
	s.lastExport = request
	if s.err != nil {
		return securitylogapp.ExportResponse{}, s.err
	}
	return s.exportResponse, nil
}

func (s *fakeSecurityLogService) ArchiveLogs(_ context.Context, before time.Time) (securitylogapp.ArchiveResponse, error) {
	s.lastArchive = before
	return s.archiveResponse, s.err
}

func (s *fakeSecurityLogService) GenerateDailyReport(context.Context) (securitylogapp.DailyReportResponse, error) {
	return s.reportResponse, s.err
}

func (s *fakeSecurityLogService) Cleanup(context.Context) (securitylogapp.CleanupResponse, error) {
	return s.cleanupResponse, s.err
}

func (s *fakeSecurityLogService) Volume(context.Context) (securitylogapp.VolumeResponse, error) {
	return s.volumeResponse, s.err
}
