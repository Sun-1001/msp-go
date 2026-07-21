package xidianhttp

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

	authapp "mathstudy/backend-go/internal/application/auth"
	xidianapp "mathstudy/backend-go/internal/application/xidian"
	"mathstudy/backend-go/internal/domain/user"
)

func TestBindingRequiresBearerToken(t *testing.T) {
	handler := newTestHandler(t, &fakeXidianService{}, &fakeAuthenticator{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/xidian")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/xidian/binding", nil)
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
}

func TestBindingStatusForwardsUserID(t *testing.T) {
	service := &fakeXidianService{status: xidianapp.BindingStatus{IsBound: true}}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "user-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/xidian")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/xidian/binding", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if service.lastUserID != "user-1" {
		t.Fatalf("userID = %q", service.lastUserID)
	}
}

func TestStartCompleteAndUnbindRoutes(t *testing.T) {
	service := &fakeXidianService{
		start:    xidianapp.BindStartResponse{ChallengeID: "challenge-1"},
		complete: xidianapp.BindCompleteResponse{IsBound: true, Username: "student"},
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "user-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/xidian")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/xidian/binding/start", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !service.startCalled {
		t.Fatalf("start status = %d called=%t", recorder.Code, service.startCalled)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/xidian/binding/complete", bytes.NewBufferString(`{"challenge_id":"challenge-1","slider_position":0.5,"username":"student","password":"pw"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.completeInput.ChallengeID != "challenge-1" || service.completeInput.SliderPosition != 0.5 {
		t.Fatalf("complete status = %d input=%#v body=%s", recorder.Code, service.completeInput, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/xidian/binding/unbind", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !service.unbindCalled {
		t.Fatalf("unbind status = %d called=%t", recorder.Code, service.unbindCalled)
	}
	var body xidianapp.UnbindResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || !body.Success {
		t.Fatalf("unbind body = %#v err=%v", body, err)
	}
}

func TestCompleteBindingRejectsOversizedBody(t *testing.T) {
	service := &fakeXidianService{}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "user-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/xidian")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/xidian/binding/complete", bytes.NewBufferString(`{"challenge_id":"challenge-1","slider_position":0.5,"username":"student","password":"`+strings.Repeat("p", maxJSONBodyBytes)+`"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if service.completeInput.ChallengeID != "" {
		t.Fatalf("service was called for oversized body: %#v", service.completeInput)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["message"] != "请求体不是有效 JSON" || body["code"] != "BAD_REQUEST" {
		t.Fatalf("body = %#v", body)
	}
}

func TestCompleteBindingRejectsTrailingJSON(t *testing.T) {
	service := &fakeXidianService{}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "user-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/xidian")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/xidian/binding/complete", bytes.NewBufferString(`{"challenge_id":"challenge-1","slider_position":0.5,"username":"student","password":"pw"} {"password":"extra"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if service.completeInput.ChallengeID != "" {
		t.Fatalf("service was called for trailing JSON body: %#v", service.completeInput)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["message"] != "请求体不是有效 JSON" || body["code"] != "BAD_REQUEST" {
		t.Fatalf("body = %#v", body)
	}
}

func TestAcademicDerivativeRoutesAreNotRegistered(t *testing.T) {
	service := &fakeXidianService{}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "user-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/xidian")

	for _, route := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/api/v1/xidian/sync/classtable"},
		{method: http.MethodPost, path: "/api/v1/xidian/sync/exams"},
		{method: http.MethodPost, path: "/api/v1/xidian/sync/scores"},
		{method: http.MethodGet, path: "/api/v1/xidian/snapshot/score"},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(route.method, route.path, nil)
		request.Header.Set("Authorization", "Bearer token")
		mux.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s %s status = %d, body = %s", route.method, route.path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestServiceErrorsRedactPublicMessages(t *testing.T) {
	service := &fakeXidianService{
		err: xidianapp.ServiceError{
			Code:    "LOGIN_FAILED",
			Message: "登录失败 Authorization: Bearer portal-token url=https://ids.example.com/authserver/login?token=query-token api_key=plain",
			Status:  http.StatusUnauthorized,
		},
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "user-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/xidian")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/xidian/binding/start", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	assertNoXidianCredentialLeak(t, recorder.Body.String())
}

func TestInternalErrorsRedactLogs(t *testing.T) {
	var logBuffer bytes.Buffer
	service := &fakeXidianService{err: errors.New("repo failed Authorization: Bearer portal-token token=query-token api_key=plain")}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "user-1", Role: user.RoleStudent}}
	handler, err := NewHandler(slog.New(slog.NewTextHandler(&logBuffer, nil)), service, auth)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/xidian")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/xidian/binding", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	assertNoXidianCredentialLeak(t, recorder.Body.String())
	assertNoXidianCredentialLeak(t, logBuffer.String())
}

func TestNewHandlerRejectsMissingDependencies(t *testing.T) {
	if _, err := NewHandler(nil, nil, &fakeAuthenticator{}); err == nil {
		t.Fatal("NewHandler(nil service) error = nil, want error")
	}
	if _, err := NewHandler(nil, &fakeXidianService{}, nil); err == nil {
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

func assertNoXidianCredentialLeak(t *testing.T, value string) {
	t.Helper()
	for _, leaked := range []string{"portal-token", "token=query-token", "api_key=plain", "Bearer portal-token"} {
		if strings.Contains(value, leaked) {
			t.Fatalf("value leaked %q in %q", leaked, value)
		}
	}
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

type fakeXidianService struct {
	status        xidianapp.BindingStatus
	start         xidianapp.BindStartResponse
	complete      xidianapp.BindCompleteResponse
	err           error
	lastUserID    string
	startCalled   bool
	unbindCalled  bool
	completeInput xidianapp.CompleteBindingInput
}

func (s *fakeXidianService) GetBindingStatus(_ context.Context, userID string) (xidianapp.BindingStatus, error) {
	s.lastUserID = userID
	return s.status, s.err
}

func (s *fakeXidianService) StartBinding(context.Context) (xidianapp.BindStartResponse, error) {
	s.startCalled = true
	return s.start, s.err
}

func (s *fakeXidianService) CompleteBinding(_ context.Context, userID string, input xidianapp.CompleteBindingInput) (xidianapp.BindCompleteResponse, error) {
	s.lastUserID = userID
	s.completeInput = input
	return s.complete, s.err
}

func (s *fakeXidianService) Unbind(_ context.Context, userID string) error {
	s.lastUserID = userID
	s.unbindCalled = true
	return s.err
}
