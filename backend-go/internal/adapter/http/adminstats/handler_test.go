package adminstatshttp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	adminstatsapp "mathstudy/backend-go/internal/application/adminstats"
	authapp "mathstudy/backend-go/internal/application/auth"
	"mathstudy/backend-go/internal/domain/user"
)

func TestRequiresAdmin(t *testing.T) {
	handler := newAdminStatsTestHandler(t, &fakeStatsService{}, &fakeAuthenticator{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/stats")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/overview", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}

	handler = newAdminStatsTestHandler(t, &fakeStatsService{}, &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}})
	mux = http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/stats")
	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/overview", nil)
	request.Header.Set("Authorization", "Bearer token")
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestRoutesForwardQueries(t *testing.T) {
	service := &fakeStatsService{
		overview: adminstatsapp.OverviewStatsResponse{TotalUsers: 10},
		growth:   adminstatsapp.UserGrowthResponse{Period: "7d"},
		recent:   adminstatsapp.RecentActivitiesResponse{Total: 1},
		status:   adminstatsapp.SystemStatusResponse{Services: []adminstatsapp.ServiceStatus{{Name: "PostgreSQL", Status: "running"}}},
	}
	handler := newAdminStatsTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/stats")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/user-growth?period=7d", nil)
	request.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastPeriod != "7d" {
		t.Fatalf("status=%d period=%q", recorder.Code, service.lastPeriod)
	}
	var growth map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &growth); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if growth["period"] != "7d" {
		t.Fatalf("growth = %#v", growth)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/recent-activities?limit=5", nil)
	request.Header.Set("Authorization", "Bearer token")
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastLimit != 5 {
		t.Fatalf("status=%d limit=%d", recorder.Code, service.lastLimit)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/system-status", nil)
	request.Header.Set("Authorization", "Bearer token")
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestValidationAndServiceErrors(t *testing.T) {
	service := &fakeStatsService{err: adminstatsapp.Error{Kind: adminstatsapp.ErrBadRequest, Message: "period 必须是 7d、30d 或 90d"}}
	handler := newAdminStatsTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/stats")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/user-growth?period=bad", nil)
	request.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	service.err = nil
	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/recent-activities?limit=bad", nil)
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
	if _, err := NewHandler(nil, &fakeStatsService{}, nil); err == nil {
		t.Fatal("NewHandler(nil auth) error = nil, want error")
	}
}

func newAdminStatsTestHandler(t *testing.T, service Service, auth Authenticator) *Handler {
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

type fakeStatsService struct {
	overview   adminstatsapp.OverviewStatsResponse
	growth     adminstatsapp.UserGrowthResponse
	recent     adminstatsapp.RecentActivitiesResponse
	status     adminstatsapp.SystemStatusResponse
	err        error
	lastPeriod string
	lastLimit  int
}

func (s *fakeStatsService) OverviewStats(context.Context) (adminstatsapp.OverviewStatsResponse, error) {
	if s.err != nil && !errors.Is(s.err, adminstatsapp.ErrBadRequest) {
		return adminstatsapp.OverviewStatsResponse{}, s.err
	}
	return s.overview, nil
}

func (s *fakeStatsService) UserGrowth(_ context.Context, period string) (adminstatsapp.UserGrowthResponse, error) {
	s.lastPeriod = period
	if s.err != nil {
		return adminstatsapp.UserGrowthResponse{}, s.err
	}
	return s.growth, nil
}

func (s *fakeStatsService) RecentActivities(_ context.Context, limit int) (adminstatsapp.RecentActivitiesResponse, error) {
	s.lastLimit = limit
	if s.err != nil {
		return adminstatsapp.RecentActivitiesResponse{}, s.err
	}
	return s.recent, nil
}

func (s *fakeStatsService) SystemStatus(context.Context) (adminstatsapp.SystemStatusResponse, error) {
	if s.err != nil {
		return adminstatsapp.SystemStatusResponse{}, s.err
	}
	return s.status, nil
}
