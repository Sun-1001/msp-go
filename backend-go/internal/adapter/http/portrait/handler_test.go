package portraithttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	airiskapp "mathstudy/backend-go/internal/application/airisk"
	authapp "mathstudy/backend-go/internal/application/auth"
	portraitapp "mathstudy/backend-go/internal/application/portrait"
	"mathstudy/backend-go/internal/domain/user"
)

func TestGetPortraitRequiresBearerToken(t *testing.T) {
	handler := newTestHandler(t, &fakePortraitService{}, &fakeAuthenticator{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/portrait")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/portrait", nil)
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["detail"] != "未认证，请先登录" || body["code"] != "UNAUTHORIZED" {
		t.Fatalf("body = %#v", body)
	}
	if got := recorder.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
}

func TestGetPortraitReturnsCurrentUsersProfile(t *testing.T) {
	content := "portrait"
	service := &fakePortraitService{
		portrait: portraitapp.PortraitResponse{
			StudentID:       "student-1",
			PortraitContent: &content,
			CorrectRate:     0.75,
			HasContent:      true,
		},
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/portrait")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/portrait", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if service.lastUserID != "student-1" {
		t.Fatalf("lastUserID = %q", service.lastUserID)
	}
	var body portraitapp.PortraitResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.StudentID != "student-1" || !body.HasContent || body.CorrectRate != 0.75 {
		t.Fatalf("body = %#v", body)
	}
}

func TestGenerateAndClearPortrait(t *testing.T) {
	service := &fakePortraitService{
		generated: portraitapp.GenerateResponse{
			PortraitContent:     "generated",
			PortraitGeneratedAt: "2026-04-25T11:00:00",
			PortraitVersion:     2,
		},
		clear: portraitapp.ClearResponse{Cleared: true, Message: "画像已清除"},
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/portrait")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/portrait/generate", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("generate status = %d", recorder.Code)
	}
	var generated portraitapp.GenerateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &generated); err != nil {
		t.Fatalf("invalid generate JSON: %v", err)
	}
	if generated.PortraitVersion != 2 || service.lastUserID != "student-1" {
		t.Fatalf("generated = %#v user=%q", generated, service.lastUserID)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodDelete, "/api/v1/portrait", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("clear status = %d", recorder.Code)
	}
	var clear portraitapp.ClearResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &clear); err != nil {
		t.Fatalf("invalid clear JSON: %v", err)
	}
	if !clear.Cleared || clear.Message != "画像已清除" {
		t.Fatalf("clear = %#v", clear)
	}
}

func TestGeneratePortraitAppliesAIGuard(t *testing.T) {
	service := &fakePortraitService{generated: portraitapp.GenerateResponse{PortraitContent: "generated"}}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	guard := &fakePortraitAIGuard{err: airiskapp.Error{Kind: airiskapp.ErrAccessBlocked, Message: "AI 已封禁"}}
	handler, err := NewHandler(nil, service, auth, WithAIRequestGuard(guard))
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/portrait")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/portrait/generate", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || service.lastUserID != "" {
		t.Fatalf("blocked status=%d user=%q body=%s", recorder.Code, service.lastUserID, recorder.Body.String())
	}

	lease := &fakePortraitAILease{}
	guard.err = nil
	guard.lease = lease
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/portrait/generate", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastUserID != "student-1" || lease.releases != 1 {
		t.Fatalf("allowed status=%d user=%q releases=%d", recorder.Code, service.lastUserID, lease.releases)
	}
	if guard.source != "portrait_generate" || guard.metered {
		t.Fatalf("guard = %#v", guard)
	}
}

func TestInternalErrorsRedactLogs(t *testing.T) {
	credentialErr := errors.New("portrait repo failed Authorization: Bearer portrait-secret token=query-token api_key=plain password=letmein")
	tests := []struct {
		name    string
		method  string
		path    string
		service fakePortraitService
	}{
		{
			name:    "get",
			method:  http.MethodGet,
			path:    "/api/v1/portrait",
			service: fakePortraitService{getErr: credentialErr},
		},
		{
			name:    "generate",
			method:  http.MethodPost,
			path:    "/api/v1/portrait/generate",
			service: fakePortraitService{generateErr: credentialErr},
		},
		{
			name:    "clear",
			method:  http.MethodDelete,
			path:    "/api/v1/portrait",
			service: fakePortraitService{clearErr: credentialErr},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logBuffer bytes.Buffer
			service := tt.service
			handler, err := NewHandler(slog.New(slog.NewTextHandler(&logBuffer, nil)), &service, &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}})
			if err != nil {
				t.Fatalf("NewHandler() error = %v", err)
			}
			mux := http.NewServeMux()
			handler.Register(mux, "/api/v1/portrait")

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)
			request.Header.Set("Authorization", "Bearer token")
			mux.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if logBuffer.Len() == 0 {
				t.Fatal("expected internal error log")
			}
			assertNoPortraitCredentialLeak(t, recorder.Body.String())
			assertNoPortraitCredentialLeak(t, logBuffer.String())
		})
	}
}

func TestNewHandlerRejectsMissingDependencies(t *testing.T) {
	if _, err := NewHandler(nil, nil, &fakeAuthenticator{}); err == nil {
		t.Fatal("NewHandler(nil service) error = nil, want error")
	}
	if _, err := NewHandler(nil, &fakePortraitService{}, nil); err == nil {
		t.Fatal("NewHandler(nil auth) error = nil, want error")
	}
}

func newTestHandler(t *testing.T, service Service, auth Authenticator) *Handler {
	t.Helper()
	handler, err := NewHandler(slog.New(slog.NewTextHandler(os.Stdout, nil)), service, auth)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return handler
}

func assertNoPortraitCredentialLeak(t *testing.T, value string) {
	t.Helper()
	for _, leaked := range []string{"portrait-secret", "token=query-token", "api_key=plain", "password=letmein", "Bearer portrait-secret"} {
		if strings.Contains(value, leaked) {
			t.Fatalf("value leaked %q in %q", leaked, value)
		}
	}
}

type fakeAuthenticator struct {
	principal authapp.Principal
}

type fakePortraitAIGuard struct {
	lease   airiskapp.Lease
	err     error
	source  string
	metered bool
}

type fakePortraitAILease struct{ releases int }

func (l *fakePortraitAILease) Release(context.Context) error {
	l.releases++
	return nil
}

func (g *fakePortraitAIGuard) Acquire(_ context.Context, _ string, source, _ string, metered bool) (airiskapp.Lease, error) {
	g.source = source
	g.metered = metered
	return g.lease, g.err
}

func (a *fakeAuthenticator) DecodeAccessToken(string) (authapp.Principal, bool) {
	if a.principal.UserID == "" {
		return authapp.Principal{}, false
	}
	return a.principal, true
}

type fakePortraitService struct {
	portrait    portraitapp.PortraitResponse
	generated   portraitapp.GenerateResponse
	clear       portraitapp.ClearResponse
	getErr      error
	generateErr error
	clearErr    error
	lastUserID  string
}

func (s *fakePortraitService) GetPortrait(_ context.Context, userID string) (portraitapp.PortraitResponse, error) {
	s.lastUserID = userID
	return s.portrait, s.getErr
}

func (s *fakePortraitService) GeneratePortrait(_ context.Context, userID string) (portraitapp.GenerateResponse, error) {
	s.lastUserID = userID
	return s.generated, s.generateErr
}

func (s *fakePortraitService) ClearPortrait(_ context.Context, userID string) (portraitapp.ClearResponse, error) {
	s.lastUserID = userID
	return s.clear, s.clearErr
}
