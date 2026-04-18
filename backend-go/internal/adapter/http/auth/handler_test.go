package authhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	authapp "mathstudy/backend-go/internal/application/auth"
	"mathstudy/backend-go/internal/domain/user"
	"mathstudy/backend-go/internal/platform/config"
)

func TestLoginSetsRefreshCookieAndReturnsAccessToken(t *testing.T) {
	service := &fakeAuthService{
		loginResult: authapp.AuthResult{
			Success:      true,
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			User: user.User{
				ID:       "user-1",
				Username: "alice",
				Email:    "alice@example.com",
				Role:     user.RoleStudent,
			},
		},
	}
	handler := newTestHandler(t, service)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/auth")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"alice","password":"Strong1!"}`))
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body loginResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.AccessToken != "access-token" || body.TokenType != "bearer" || body.User.Role != "student" {
		t.Fatalf("body = %#v", body)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != "refresh_token" || cookie.Value != "refresh-token" {
		t.Fatalf("cookie = %#v", cookie)
	}
	if cookie.Path != "/api/v1/auth" || !cookie.HttpOnly || cookie.Secure {
		t.Fatalf("cookie flags = %#v", cookie)
	}
}

func TestLoginFailureUsesFastAPICompatibleDetail(t *testing.T) {
	service := &fakeAuthService{
		loginResult: authapp.AuthResult{Success: false, Error: "用户名或密码错误"},
	}
	handler := newTestHandler(t, service)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/auth")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"alice","password":"bad"}`))
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["detail"] != "用户名或密码错误" || body["code"] != "UNAUTHORIZED" {
		t.Fatalf("body = %#v", body)
	}
	if got := recorder.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
}

func TestRefreshClearsInvalidCookie(t *testing.T) {
	handler := newTestHandler(t, &fakeAuthService{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/auth")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	request.AddCookie(&http.Cookie{Name: "refresh_token", Value: "bad"})
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "refresh_token" || cookies[0].MaxAge != -1 {
		t.Fatalf("cookies = %#v", cookies)
	}
}

func TestMeRequiresBearerToken(t *testing.T) {
	handler := newTestHandler(t, &fakeAuthService{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/auth")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["detail"] != "未认证，请先登录" {
		t.Fatalf("body = %#v", body)
	}
}

func TestNewHandlerRejectsNilService(t *testing.T) {
	if _, err := NewHandler(config.Config{}, nil, nil); err == nil {
		t.Fatal("NewHandler(nil service) error = nil, want error")
	}
}

func newTestHandler(t *testing.T, service Service) *Handler {
	t.Helper()
	handler, err := NewHandler(config.Config{
		Environment:           "development",
		APIV1Prefix:           "/api/v1",
		JWTRefreshTokenExpire: 7 * 24 * time.Hour,
	}, slog.New(slog.NewTextHandler(os.Stdout, nil)), service)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return handler
}

type fakeAuthService struct {
	loginResult          authapp.AuthResult
	registerResult       authapp.AuthResult
	principal            authapp.Principal
	refreshSubject       string
	refreshAccessToken   string
	refreshRefreshToken  string
	registrationSettings authapp.RegistrationSettings
}

func (s *fakeAuthService) Authenticate(context.Context, string, string) (authapp.AuthResult, error) {
	return s.loginResult, nil
}

func (s *fakeAuthService) Register(context.Context, string, string, string, string) (authapp.AuthResult, error) {
	return s.registerResult, nil
}

func (s *fakeAuthService) ChangePassword(context.Context, string, string, string) (bool, string, error) {
	return true, "密码修改成功", nil
}

func (s *fakeAuthService) GetUserByID(context.Context, string) (user.User, bool, error) {
	return user.User{ID: "user-1", Username: "alice", Email: "alice@example.com", Role: user.RoleStudent}, true, nil
}

func (s *fakeAuthService) RefreshTokens(context.Context, string) (string, string, bool, error) {
	if s.refreshAccessToken == "" {
		return "", "", false, nil
	}
	return s.refreshAccessToken, s.refreshRefreshToken, true, nil
}

func (s *fakeAuthService) DecodeAccessToken(string) (authapp.Principal, bool) {
	if s.principal.UserID == "" {
		return authapp.Principal{}, false
	}
	return s.principal, true
}

func (s *fakeAuthService) DecodeRefreshToken(string) (string, bool) {
	if s.refreshSubject == "" {
		return "", false
	}
	return s.refreshSubject, true
}

func (s *fakeAuthService) RegistrationSettings(context.Context) (authapp.RegistrationSettings, error) {
	if !s.registrationSettings.AllowStudent && !s.registrationSettings.AllowTeacher {
		return authapp.RegistrationSettings{AllowStudent: true, AllowTeacher: true}, nil
	}
	return s.registrationSettings, nil
}

func (s *fakeAuthService) SubmitPasswordReset(context.Context, string, string, string) (authapp.PasswordResetResult, error) {
	return authapp.PasswordResetResult{Success: true, Message: "申请已提交，请等待管理员审批"}, nil
}

func (s *fakeAuthService) PasswordResetStatus(context.Context, string, string) (authapp.PasswordResetStatus, error) {
	return authapp.PasswordResetStatus{HasPending: false}, nil
}
