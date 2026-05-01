package admininboxhttp

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	admininboxapp "mathstudy/backend-go/internal/application/admininbox"
	authapp "mathstudy/backend-go/internal/application/auth"
	"mathstudy/backend-go/internal/domain/user"
)

func TestAdminInboxRoutesRequireAdmin(t *testing.T) {
	handler := newAdminInboxTestHandler(t, &fakeInboxService{}, &fakeAuthenticator{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/inbox")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/inbox", nil)
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", got)
	}

	handler = newAdminInboxTestHandler(t, &fakeInboxService{}, &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}})
	mux = http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/inbox")
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/inbox", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestListRequestsParsesFilter(t *testing.T) {
	service := &fakeInboxService{listResponse: admininboxapp.ListResponse{Items: []admininboxapp.RequestItem{}, Total: 0, PendingCount: 2}}
	handler := newAdminInboxTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/inbox")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/inbox?status=pending&page=2&page_size=15", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.lastFilter.Status != "pending" || service.lastFilter.Page != 2 || service.lastFilter.PageSize != 15 {
		t.Fatalf("filter = %#v", service.lastFilter)
	}
}

func TestPendingCountRoute(t *testing.T) {
	service := &fakeInboxService{pendingCount: 4}
	handler := newAdminInboxTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/inbox")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/inbox/pending-count", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || !service.pendingCalled {
		t.Fatalf("status=%d pendingCalled=%t", recorder.Code, service.pendingCalled)
	}
	if body := recorder.Body.String(); body != "{\"pending_count\":4}\n" {
		t.Fatalf("body = %s", body)
	}
}

func TestReviewRequestForwardsAdminAndBody(t *testing.T) {
	service := &fakeInboxService{reviewResponse: admininboxapp.ReviewResponse{Success: true, Message: "已拒绝该申请"}}
	handler := newAdminInboxTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/inbox")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/inbox/reset-1/review", bytes.NewBufferString(`{"action":"reject","reject_reason":"信息不匹配"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.lastRequestID != "reset-1" || service.lastAdminID != "admin-1" || service.lastAction != "reject" {
		t.Fatalf("requestID=%q adminID=%q action=%q", service.lastRequestID, service.lastAdminID, service.lastAction)
	}
	if service.lastReason == nil || *service.lastReason != "信息不匹配" {
		t.Fatalf("reject reason = %#v", service.lastReason)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/inbox/reset-1/review", bytes.NewBufferString(`{"action":"invalid"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid action status = %d", recorder.Code)
	}
}

func TestAdminInboxServiceErrorsMapToStatusCodes(t *testing.T) {
	service := &fakeInboxService{err: admininboxapp.Error{Kind: admininboxapp.ErrBadRequest, Message: "status 必须是 pending、approved 或 rejected"}}
	handler := newAdminInboxTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/inbox")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/inbox?status=done", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestNewHandlerRejectsMissingDependencies(t *testing.T) {
	if _, err := NewHandler(nil, nil, &fakeAuthenticator{}); err == nil {
		t.Fatal("NewHandler(nil service) error = nil, want error")
	}
	if _, err := NewHandler(nil, &fakeInboxService{}, nil); err == nil {
		t.Fatal("NewHandler(nil auth) error = nil, want error")
	}
}

func newAdminInboxTestHandler(t *testing.T, service Service, auth Authenticator) *Handler {
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
	if a.principal.UserID == "" {
		return authapp.Principal{}, false
	}
	return a.principal, true
}

type fakeInboxService struct {
	listResponse   admininboxapp.ListResponse
	reviewResponse admininboxapp.ReviewResponse
	err            error
	pendingCount   int
	pendingCalled  bool
	lastFilter     admininboxapp.ListFilter
	lastRequestID  string
	lastAdminID    string
	lastAction     string
	lastReason     *string
}

func (s *fakeInboxService) ListRequests(_ context.Context, filter admininboxapp.ListFilter) (admininboxapp.ListResponse, error) {
	s.lastFilter = filter
	return s.listResponse, s.err
}

func (s *fakeInboxService) PendingCount(context.Context) (int, error) {
	s.pendingCalled = true
	return s.pendingCount, s.err
}

func (s *fakeInboxService) ReviewRequest(_ context.Context, requestID string, adminID string, action string, rejectReason *string) (admininboxapp.ReviewResponse, error) {
	s.lastRequestID = requestID
	s.lastAdminID = adminID
	s.lastAction = action
	s.lastReason = rejectReason
	return s.reviewResponse, s.err
}
