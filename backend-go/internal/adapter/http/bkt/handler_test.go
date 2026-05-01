package bkthttp

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	authapp "mathstudy/backend-go/internal/application/auth"
	bktapp "mathstudy/backend-go/internal/application/bkt"
	"mathstudy/backend-go/internal/domain/user"
)

func TestBKTRoutesRequireAdmin(t *testing.T) {
	handler := newBKTTestHandler(t, &fakeBKTService{}, &fakeAuthenticator{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/bkt")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/bkt/params", nil)
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", got)
	}

	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}}
	handler = newBKTTestHandler(t, &fakeBKTService{}, auth)
	mux = http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/bkt")
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/bkt/params", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestListParamsParsesPagination(t *testing.T) {
	service := &fakeBKTService{listResponse: bktapp.ListResponse{Items: []bktapp.Param{}, Total: 0, Offset: 10, Limit: 25}}
	handler := newBKTTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/bkt")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/bkt/params?offset=10&limit=25", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.lastOffset != 10 || service.lastLimit != 25 {
		t.Fatalf("offset=%d limit=%d", service.lastOffset, service.lastLimit)
	}
}

func TestUpdateParamValidatesAndForwards(t *testing.T) {
	service := &fakeBKTService{paramResponse: bktapp.Param{ConceptID: "node-1", PL0: 0.3, PT: 0.12, PG: 0.2, PS: 0.1}}
	handler := newBKTTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/bkt")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/bkt/params/node-1", bytes.NewBufferString(`{"p_g":0.6}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", recorder.Code)
	}
	if service.updateCalled {
		t.Fatal("UpdateParam called for invalid request")
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/bkt/params/node-1", bytes.NewBufferString(`{"p_l0":0.3}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.lastConceptID != "node-1" || service.lastUpdate.PL0 == nil || *service.lastUpdate.PL0 != 0.3 {
		t.Fatalf("concept=%q update=%#v", service.lastConceptID, service.lastUpdate)
	}
}

func TestResetAndSeedRoutes(t *testing.T) {
	service := &fakeBKTService{
		paramResponse: bktapp.Param{ConceptID: "node-1", PL0: 0.25, PT: 0.12, PG: 0.2, PS: 0.1},
		seedResponse:  bktapp.SeedResponse{SeededCount: 2, Message: "已为 2 个知识点创建默认 BKT 参数"},
	}
	handler := newBKTTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/bkt")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/bkt/params/reset/node-1", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastConceptID != "node-1" {
		t.Fatalf("reset status=%d concept=%q", recorder.Code, service.lastConceptID)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/bkt/seed", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !service.seedCalled {
		t.Fatalf("seed status=%d called=%t", recorder.Code, service.seedCalled)
	}
}

func TestServiceErrorsMapToStatusCodes(t *testing.T) {
	service := &fakeBKTService{err: bktapp.Error{Kind: bktapp.ErrNotFound, Message: "知识点 node-1 的 BKT 参数不存在"}}
	handler := newBKTTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/bkt")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/bkt/params/reset/node-1", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestNewHandlerRejectsMissingDependencies(t *testing.T) {
	if _, err := NewHandler(nil, nil, &fakeAuthenticator{}); err == nil {
		t.Fatal("NewHandler(nil service) error = nil, want error")
	}
	if _, err := NewHandler(nil, &fakeBKTService{}, nil); err == nil {
		t.Fatal("NewHandler(nil auth) error = nil, want error")
	}
}

func newBKTTestHandler(t *testing.T, service Service, auth Authenticator) *Handler {
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

type fakeBKTService struct {
	listResponse  bktapp.ListResponse
	paramResponse bktapp.Param
	seedResponse  bktapp.SeedResponse
	err           error
	lastOffset    int
	lastLimit     int
	lastConceptID string
	lastUpdate    bktapp.Update
	updateCalled  bool
	seedCalled    bool
}

func (s *fakeBKTService) ListParams(_ context.Context, offset int, limit int) (bktapp.ListResponse, error) {
	s.lastOffset = offset
	s.lastLimit = limit
	return s.listResponse, s.err
}

func (s *fakeBKTService) UpdateParam(_ context.Context, conceptID string, update bktapp.Update) (bktapp.Param, error) {
	s.updateCalled = true
	s.lastConceptID = conceptID
	s.lastUpdate = update
	return s.paramResponse, s.err
}

func (s *fakeBKTService) ResetParam(_ context.Context, conceptID string) (bktapp.Param, error) {
	s.lastConceptID = conceptID
	return s.paramResponse, s.err
}

func (s *fakeBKTService) SeedDefaultParams(context.Context) (bktapp.SeedResponse, error) {
	s.seedCalled = true
	return s.seedResponse, s.err
}
