package adminairiskhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	airiskapp "mathstudy/backend-go/internal/application/airisk"
	authapp "mathstudy/backend-go/internal/application/auth"
	"mathstudy/backend-go/internal/domain/user"
)

func TestHandlerRequiresAdministrator(t *testing.T) {
	service := &fakeService{}
	handler := newTestHandler(t, service, &fakeAuthenticator{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/risk-control")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/risk-control/overview", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", recorder.Code)
	}

	handler = newTestHandler(t, service, &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}})
	mux = http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/risk-control")
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, authedRequest(http.MethodGet, "/api/v1/admin/risk-control/overview", ""))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("forbidden status = %d", recorder.Code)
	}
}

func TestHandlerRoutesForwardValidatedRequests(t *testing.T) {
	service := &fakeService{
		overview: airiskapp.Overview{TotalStudents: 3},
		settings: airiskapp.Settings{DailyReplyLimit: 50, MaxConcurrentRequests: 2},
		students: airiskapp.StudentListResponse{Items: []airiskapp.StudentItem{{ID: "student-1"}}, Total: 1},
		access:   airiskapp.StudentAccessResponse{StudentID: "student-1", AIBlocked: true},
		events:   airiskapp.EventListResponse{Items: []airiskapp.RiskEvent{{ID: "event-1"}}, Total: 1},
	}
	handler := newTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/risk-control")

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/admin/risk-control/overview", ""},
		{http.MethodGet, "/api/v1/admin/risk-control/settings", ""},
		{http.MethodPut, "/api/v1/admin/risk-control/settings", `{"daily_reply_limit":80,"max_concurrent_requests":3,"blocked_keywords":["代考"]}`},
		{http.MethodGet, "/api/v1/admin/risk-control/students?page=2&page_size=25&search=alice&status=blocked", ""},
		{http.MethodPatch, "/api/v1/admin/risk-control/students/student-1/access", `{"blocked":true,"reason":"违规"}`},
		{http.MethodGet, "/api/v1/admin/risk-control/events?page=3&page_size=10&search=alice&event_type=content_blocked", ""},
	}
	for _, test := range tests {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, authedRequest(test.method, test.path, test.body))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s %s status=%d body=%s", test.method, test.path, recorder.Code, recorder.Body.String())
		}
	}
	if service.settingsUpdate.DailyReplyLimit != 80 || len(service.settingsUpdate.BlockedKeywords) != 1 {
		t.Fatalf("settings update = %#v", service.settingsUpdate)
	}
	if service.studentFilter.Page != 2 || service.studentFilter.PageSize != 25 || service.studentFilter.Status != "blocked" {
		t.Fatalf("student filter = %#v", service.studentFilter)
	}
	if service.studentID != "student-1" || service.actorID != "admin-1" || service.accessUpdate.Reason != "违规" {
		t.Fatalf("access update = student=%q actor=%q request=%#v", service.studentID, service.actorID, service.accessUpdate)
	}
	if service.eventFilter.Page != 3 || service.eventFilter.EventType != "content_blocked" {
		t.Fatalf("event filter = %#v", service.eventFilter)
	}
}

func TestHandlerRejectsInvalidQueryAndTrailingJSON(t *testing.T) {
	handler := newTestHandler(t, &fakeService{}, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/risk-control")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, authedRequest(http.MethodGet, "/api/v1/admin/risk-control/students?page_size=101", ""))
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid query status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, authedRequest(http.MethodPut, "/api/v1/admin/risk-control/settings", `{"daily_reply_limit":1,"max_concurrent_requests":1,"blocked_keywords":[]} {"extra":true}`))
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("trailing JSON status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlerMapsDomainAndInternalErrors(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{
		{airiskapp.Error{Kind: airiskapp.ErrBadRequest, Message: "bad input"}, http.StatusBadRequest},
		{airiskapp.Error{Kind: airiskapp.ErrNotFound, Message: "missing"}, http.StatusNotFound},
		{errors.New("database down token=secret"), http.StatusInternalServerError},
	}
	for _, test := range tests {
		handler := newTestHandler(t, &fakeService{err: test.err}, adminAuthenticator())
		mux := http.NewServeMux()
		handler.Register(mux, "/api/v1/admin/risk-control")
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, authedRequest(http.MethodGet, "/api/v1/admin/risk-control/overview", ""))
		if recorder.Code != test.want {
			t.Fatalf("error=%v status=%d body=%s", test.err, recorder.Code, recorder.Body.String())
		}
		if test.want == http.StatusInternalServerError && bytes.Contains(recorder.Body.Bytes(), []byte("token=secret")) {
			t.Fatalf("internal error leaked: %s", recorder.Body.String())
		}
	}
}

func TestNewHandlerValidatesDependencies(t *testing.T) {
	if _, err := NewHandler(nil, nil, adminAuthenticator()); err == nil {
		t.Fatal("NewHandler(nil service) error = nil")
	}
	if _, err := NewHandler(nil, &fakeService{}, nil); err == nil {
		t.Fatal("NewHandler(nil auth) error = nil")
	}
}

func TestWriteRiskErrorSerializesCode(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeRiskError(recorder, http.StatusConflict, "CONFLICT", "冲突")
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != "CONFLICT" || body["detail"] != "冲突" {
		t.Fatalf("body = %#v", body)
	}
}

func newTestHandler(t *testing.T, service Service, auth Authenticator) *Handler {
	t.Helper()
	handler, err := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), service, auth)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return handler
}

func authedRequest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer token")
	return request
}

func adminAuthenticator() *fakeAuthenticator {
	return &fakeAuthenticator{principal: authapp.Principal{UserID: "admin-1", Role: user.RoleAdmin}}
}

type fakeAuthenticator struct{ principal authapp.Principal }

func (a *fakeAuthenticator) DecodeAccessToken(string) (authapp.Principal, bool) {
	return a.principal, a.principal.UserID != ""
}

type fakeService struct {
	overview       airiskapp.Overview
	settings       airiskapp.Settings
	students       airiskapp.StudentListResponse
	access         airiskapp.StudentAccessResponse
	events         airiskapp.EventListResponse
	err            error
	settingsUpdate airiskapp.UpdateSettingsRequest
	studentFilter  airiskapp.StudentListFilter
	eventFilter    airiskapp.EventListFilter
	studentID      string
	actorID        string
	accessUpdate   airiskapp.UpdateStudentAccessRequest
}

func (s *fakeService) GetOverview(context.Context) (airiskapp.Overview, error) {
	return s.overview, s.err
}

func (s *fakeService) GetSettings(context.Context) (airiskapp.Settings, error) {
	return s.settings, s.err
}

func (s *fakeService) UpdateSettings(_ context.Context, request airiskapp.UpdateSettingsRequest) (airiskapp.Settings, error) {
	s.settingsUpdate = request
	return s.settings, s.err
}

func (s *fakeService) ListStudents(_ context.Context, filter airiskapp.StudentListFilter) (airiskapp.StudentListResponse, error) {
	s.studentFilter = filter
	return s.students, s.err
}

func (s *fakeService) UpdateStudentAccess(_ context.Context, studentID, actorID string, request airiskapp.UpdateStudentAccessRequest) (airiskapp.StudentAccessResponse, error) {
	s.studentID = studentID
	s.actorID = actorID
	s.accessUpdate = request
	return s.access, s.err
}

func (s *fakeService) ListRiskEvents(_ context.Context, filter airiskapp.EventListFilter) (airiskapp.EventListResponse, error) {
	s.eventFilter = filter
	return s.events, s.err
}
