package authhttp

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
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"alice","password":"Strong1!","captcha_token":"proof"}`))
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
	refreshCookie := cookieByName(t, recorder.Result().Cookies(), refreshCookieName)
	if refreshCookie.Value != "refresh-token" {
		t.Fatalf("cookie = %#v", refreshCookie)
	}
	if refreshCookie.Path != "/api/v1/auth" || !refreshCookie.HttpOnly || refreshCookie.Secure {
		t.Fatalf("cookie flags = %#v", refreshCookie)
	}
	csrfCookie := cookieByName(t, recorder.Result().Cookies(), csrfCookieName)
	if len(csrfCookie.Value) != csrfTokenBytes*2 || csrfCookie.Path != "/" || csrfCookie.HttpOnly || csrfCookie.Secure {
		t.Fatalf("csrf cookie = %#v", csrfCookie)
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
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"alice","password":"bad","captcha_token":"proof"}`))
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

func TestLoginRequiresOneTimeCaptchaProof(t *testing.T) {
	service := &fakeAuthService{loginResult: authapp.AuthResult{Success: true}}
	handler := newTestHandlerWithCaptcha(t, service, &fakeCaptchaService{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/auth")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"alice","password":"Strong1!"}`))
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest || service.authenticateCalls != 0 {
		t.Fatalf("status/calls = %d/%d, body = %s", recorder.Code, service.authenticateCalls, recorder.Body.String())
	}
	assertDetailCode(t, recorder.Body.Bytes(), "CAPTCHA_REQUIRED")
}

func TestCaptchaChallengeAndVerificationRoutes(t *testing.T) {
	captcha := &fakeCaptchaService{
		challenge:    authapp.SliderCaptchaChallenge{ID: "challenge-1", Width: 320, Height: 160, ExpiresIn: 120},
		proof:        "proof-1",
		verified:     true,
		consumeValid: true,
	}
	handler := newTestHandlerWithCaptcha(t, &fakeAuthService{}, captcha)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/auth")

	challengeRecorder := httptest.NewRecorder()
	challengeRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/captcha", nil)
	challengeRequest.RemoteAddr = "127.0.0.1:1234"
	challengeRequest.Header.Set("X-Real-IP", "203.0.113.8")
	mux.ServeHTTP(challengeRecorder, challengeRequest)
	if challengeRecorder.Code != http.StatusOK || challengeRecorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("challenge status/headers = %d/%v", challengeRecorder.Code, challengeRecorder.Header())
	}
	if captcha.clientKey != "203.0.113.8" {
		t.Fatalf("captcha client key = %q", captcha.clientKey)
	}

	verifyRecorder := httptest.NewRecorder()
	verifyRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/captcha/verify", bytes.NewBufferString(`{"captcha_id":"challenge-1","position":93}`))
	mux.ServeHTTP(verifyRecorder, verifyRequest)
	if verifyRecorder.Code != http.StatusOK {
		t.Fatalf("verify status = %d, body = %s", verifyRecorder.Code, verifyRecorder.Body.String())
	}
	var response verifyCaptchaResponse
	if err := json.Unmarshal(verifyRecorder.Body.Bytes(), &response); err != nil || response.CaptchaToken != "proof-1" || response.ExpiresIn != 90 {
		t.Fatalf("verify response = %#v, err = %v", response, err)
	}
}

func TestCaptchaRoutesHandleInvalidAndRateLimitedRequests(t *testing.T) {
	t.Run("missing position", func(t *testing.T) {
		handler := newTestHandlerWithCaptcha(t, &fakeAuthService{}, &fakeCaptchaService{})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/captcha/verify", bytes.NewBufferString(`{"captcha_id":"challenge-1"}`))
		handler.verifyCaptcha(recorder, request)
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("wrong position", func(t *testing.T) {
		handler := newTestHandlerWithCaptcha(t, &fakeAuthService{}, &fakeCaptchaService{})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/captcha/verify", bytes.NewBufferString(`{"captcha_id":"challenge-1","position":1}`))
		handler.verifyCaptcha(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		assertDetailCode(t, recorder.Body.Bytes(), "CAPTCHA_INVALID")
	})

	t.Run("rate limited", func(t *testing.T) {
		handler := newTestHandlerWithCaptcha(t, &fakeAuthService{}, &fakeCaptchaService{err: authapp.ErrCaptchaRateLimited})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/captcha", nil)
		handler.captchaChallenge(recorder, request)
		if recorder.Code != http.StatusTooManyRequests || recorder.Header().Get("Retry-After") != "60" {
			t.Fatalf("status/Retry-After = %d/%q", recorder.Code, recorder.Header().Get("Retry-After"))
		}
	})
}

func TestSensitiveJSONRoutesRejectTrailingJSON(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		path    string
		body    string
		prepare func(*http.Request)
	}{
		{
			name:   "login",
			method: http.MethodPost,
			path:   "/api/v1/auth/login",
			body:   `{"username":"alice","password":"Strong1!"} {"password":"extra"}`,
		},
		{
			name:   "register",
			method: http.MethodPost,
			path:   "/api/v1/auth/register",
			body:   `{"username":"alice","email":"alice@example.com","password":"Strong1!","role":"student"} {"role":"admin"}`,
		},
		{
			name:    "change password",
			method:  http.MethodPut,
			path:    "/api/v1/auth/change-password",
			body:    `{"old_password":"old","new_password":"Strong1!"} {"new_password":"extra"}`,
			prepare: addBearerToken,
		},
		{
			name:   "forgot password",
			method: http.MethodPost,
			path:   "/api/v1/auth/forgot-password",
			body:   `{"username":"alice","email":"alice@example.com","reason":"lost"} {"reason":"extra"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeAuthService{principal: authapp.Principal{UserID: "user-1", Role: user.RoleStudent}}
			handler := newTestHandler(t, service)
			mux := http.NewServeMux()
			handler.Register(mux, "/api/v1/auth")

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			if tt.prepare != nil {
				tt.prepare(request)
			}
			mux.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			var body map[string]string
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if body["detail"] != "请求体格式错误" || body["code"] != "VALIDATION_ERROR" {
				t.Fatalf("body = %#v", body)
			}
		})
	}
}

func TestRefreshClearsInvalidCookie(t *testing.T) {
	handler := newTestHandler(t, &fakeAuthService{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/auth")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	addCSRFCookies(request, "csrf-token")
	request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "bad"})
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	cookies := recorder.Result().Cookies()
	if cookieByName(t, cookies, refreshCookieName).MaxAge != -1 || cookieByName(t, cookies, csrfCookieName).MaxAge != -1 {
		t.Fatalf("cookies = %#v", cookies)
	}
}

func TestRefreshRequiresCSRFToken(t *testing.T) {
	handler := newTestHandler(t, &fakeAuthService{
		refreshPrincipal: authapp.RefreshPrincipal{UserID: "user-1", JTI: "jti"},
	})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/auth")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "refresh-token"})
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["code"] != "CSRF_TOKEN_MISSING" {
		t.Fatalf("body = %#v", body)
	}
}

func TestRefreshRotatesRefreshAndCSRFCookies(t *testing.T) {
	handler := newTestHandler(t, &fakeAuthService{
		refreshPrincipal:    authapp.RefreshPrincipal{UserID: "user-1", JTI: "jti"},
		refreshAccessToken:  "new-access-token",
		refreshRefreshToken: "new-refresh-token",
	})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/auth")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "old-refresh-token"})
	addCSRFCookies(request, "csrf-token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body refreshResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.AccessToken != "new-access-token" || body.TokenType != "bearer" {
		t.Fatalf("body = %#v", body)
	}
	cookies := recorder.Result().Cookies()
	if cookieByName(t, cookies, refreshCookieName).Value != "new-refresh-token" {
		t.Fatalf("cookies = %#v", cookies)
	}
	if csrfCookie := cookieByName(t, cookies, csrfCookieName); len(csrfCookie.Value) != csrfTokenBytes*2 || csrfCookie.Value == "csrf-token" {
		t.Fatalf("csrf cookie was not rotated: %#v", csrfCookie)
	}
}

func TestLogoutRequiresCSRFAndClearsCookies(t *testing.T) {
	service := &fakeAuthService{}
	handler := newTestHandler(t, service)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/auth")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "refresh-token"})
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("missing csrf status = %d", recorder.Code)
	}
	if service.revokedToken != "" {
		t.Fatalf("revoked token without csrf = %q", service.revokedToken)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "refresh-token"})
	addCSRFCookies(request, "csrf-token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if service.revokedToken != "refresh-token" {
		t.Fatalf("revoked token = %q", service.revokedToken)
	}
	cookies := recorder.Result().Cookies()
	if cookieByName(t, cookies, refreshCookieName).MaxAge != -1 || cookieByName(t, cookies, csrfCookieName).MaxAge != -1 {
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
	if _, err := NewHandler(config.Config{}, nil, nil, &fakeCaptchaService{}); err == nil {
		t.Fatal("NewHandler(nil service) error = nil, want error")
	}
	if _, err := NewHandler(config.Config{}, nil, &fakeAuthService{}, nil); err == nil {
		t.Fatal("NewHandler(nil captcha) error = nil, want error")
	}
}

func TestInternalErrorsRedactLogs(t *testing.T) {
	credentialErr := errors.New("db failed Authorization: Bearer auth-secret token=query-token api_key=plain password=letmein session_id=sess-123")
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		service    fakeAuthService
		prepare    func(*http.Request)
		wantStatus int
	}{
		{
			name:       "login",
			method:     http.MethodPost,
			path:       "/api/v1/auth/login",
			body:       `{"username":"alice","password":"Strong1!","captcha_token":"proof"}`,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "register",
			method:     http.MethodPost,
			path:       "/api/v1/auth/register",
			body:       `{"username":"alice","email":"alice@example.com","password":"Strong1!","role":"student"}`,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "change password",
			method:     http.MethodPut,
			path:       "/api/v1/auth/change-password",
			body:       `{"old_password":"old","new_password":"Strong1!"}`,
			service:    fakeAuthService{principal: authapp.Principal{UserID: "user-1", Role: user.RoleStudent}},
			prepare:    addBearerToken,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:    "refresh",
			method:  http.MethodPost,
			path:    "/api/v1/auth/refresh",
			service: fakeAuthService{refreshPrincipal: authapp.RefreshPrincipal{UserID: "user-1", JTI: "jti"}},
			prepare: func(request *http.Request) {
				request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "refresh-token"})
				addCSRFCookies(request, "csrf-token")
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:   "logout",
			method: http.MethodPost,
			path:   "/api/v1/auth/logout",
			prepare: func(request *http.Request) {
				request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "refresh-token"})
				addCSRFCookies(request, "csrf-token")
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "me",
			method:     http.MethodGet,
			path:       "/api/v1/auth/me",
			service:    fakeAuthService{principal: authapp.Principal{UserID: "user-1", Role: user.RoleStudent}},
			prepare:    addBearerToken,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "registration status",
			method:     http.MethodGet,
			path:       "/api/v1/auth/registration-status",
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "forgot password",
			method:     http.MethodPost,
			path:       "/api/v1/auth/forgot-password",
			body:       `{"username":"alice","email":"alice@example.com","reason":"lost"}`,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "forgot password status",
			method:     http.MethodGet,
			path:       "/api/v1/auth/forgot-password/status?username=alice&email=alice@example.com",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logBuffer bytes.Buffer
			service := tt.service
			service.err = credentialErr
			handler, err := NewHandler(config.Config{
				Environment:           "development",
				APIV1Prefix:           "/api/v1",
				JWTRefreshTokenExpire: 7 * 24 * time.Hour,
			}, slog.New(slog.NewTextHandler(&logBuffer, nil)), &service, &fakeCaptchaService{consumeValid: true})
			if err != nil {
				t.Fatalf("NewHandler() error = %v", err)
			}
			mux := http.NewServeMux()
			handler.Register(mux, "/api/v1/auth")

			var body *bytes.Reader
			if tt.body == "" {
				body = bytes.NewReader(nil)
			} else {
				body = bytes.NewReader([]byte(tt.body))
			}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, body)
			if tt.prepare != nil {
				tt.prepare(request)
			}
			mux.ServeHTTP(recorder, request)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if logBuffer.Len() == 0 {
				t.Fatal("expected internal error log")
			}
			assertNoAuthCredentialLeak(t, recorder.Body.String())
			assertNoAuthCredentialLeak(t, logBuffer.String())
		})
	}
}

func newTestHandler(t *testing.T, service Service) *Handler {
	t.Helper()
	return newTestHandlerWithCaptcha(t, service, &fakeCaptchaService{consumeValid: true, verified: true, proof: "proof"})
}

func newTestHandlerWithCaptcha(t *testing.T, service Service, captcha CaptchaService) *Handler {
	t.Helper()
	handler, err := NewHandler(config.Config{
		Environment:           "development",
		APIV1Prefix:           "/api/v1",
		JWTRefreshTokenExpire: 7 * 24 * time.Hour,
	}, slog.New(slog.NewTextHandler(os.Stdout, nil)), service, captcha)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return handler
}

type fakeAuthService struct {
	loginResult          authapp.AuthResult
	registerResult       authapp.AuthResult
	principal            authapp.Principal
	refreshPrincipal     authapp.RefreshPrincipal
	refreshAccessToken   string
	refreshRefreshToken  string
	registrationSettings authapp.RegistrationSettings
	revokedToken         string
	err                  error
	authenticateCalls    int
}

func (s *fakeAuthService) Authenticate(context.Context, string, string) (authapp.AuthResult, error) {
	s.authenticateCalls++
	if s.err != nil {
		return authapp.AuthResult{}, s.err
	}
	return s.loginResult, nil
}

type fakeCaptchaService struct {
	challenge    authapp.SliderCaptchaChallenge
	proof        string
	verified     bool
	consumeValid bool
	clientKey    string
	err          error
}

func (s *fakeCaptchaService) NewChallenge(_ context.Context, clientKey string) (authapp.SliderCaptchaChallenge, error) {
	s.clientKey = clientKey
	return s.challenge, s.err
}

func (s *fakeCaptchaService) Verify(_ context.Context, _ string, _ int, clientKey string) (string, bool, error) {
	s.clientKey = clientKey
	return s.proof, s.verified, s.err
}

func (s *fakeCaptchaService) ConsumeProof(_ context.Context, _ string, clientKey string) (bool, error) {
	s.clientKey = clientKey
	return s.consumeValid, s.err
}

func (s *fakeCaptchaService) ProofTTL() time.Duration {
	return 90 * time.Second
}

func (s *fakeCaptchaService) IssueWindow() time.Duration {
	return time.Minute
}

func (s *fakeAuthService) Register(context.Context, string, string, string, string) (authapp.AuthResult, error) {
	if s.err != nil {
		return authapp.AuthResult{}, s.err
	}
	return s.registerResult, nil
}

func (s *fakeAuthService) ChangePassword(context.Context, string, string, string) (bool, string, error) {
	if s.err != nil {
		return false, "", s.err
	}
	return true, "密码修改成功", nil
}

func (s *fakeAuthService) GetUserByID(context.Context, string) (user.User, bool, error) {
	if s.err != nil {
		return user.User{}, false, s.err
	}
	return user.User{ID: "user-1", Username: "alice", Email: "alice@example.com", Role: user.RoleStudent}, true, nil
}

func (s *fakeAuthService) RefreshTokens(context.Context, authapp.RefreshPrincipal) (string, string, bool, error) {
	if s.err != nil {
		return "", "", false, s.err
	}
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

func (s *fakeAuthService) DecodeRefreshToken(string) (authapp.RefreshPrincipal, string, bool) {
	if s.refreshPrincipal.UserID == "" {
		return authapp.RefreshPrincipal{}, "Refresh token 无效或已过期", false
	}
	return s.refreshPrincipal, "", true
}

func (s *fakeAuthService) RevokeRefreshToken(_ context.Context, token string) error {
	s.revokedToken = token
	return s.err
}

func (s *fakeAuthService) RegistrationSettings(context.Context) (authapp.RegistrationSettings, error) {
	if s.err != nil {
		return authapp.RegistrationSettings{}, s.err
	}
	if !s.registrationSettings.AllowStudent && !s.registrationSettings.AllowTeacher {
		return authapp.RegistrationSettings{AllowStudent: true, AllowTeacher: true}, nil
	}
	return s.registrationSettings, nil
}

func (s *fakeAuthService) SubmitPasswordReset(context.Context, string, string, string) (authapp.PasswordResetResult, error) {
	if s.err != nil {
		return authapp.PasswordResetResult{}, s.err
	}
	return authapp.PasswordResetResult{Success: true, Message: "申请已提交，请等待管理员审批"}, nil
}

func (s *fakeAuthService) PasswordResetStatus(context.Context, string, string) (authapp.PasswordResetStatus, error) {
	if s.err != nil {
		return authapp.PasswordResetStatus{}, s.err
	}
	return authapp.PasswordResetStatus{HasPending: false}, nil
}

func addBearerToken(request *http.Request) {
	request.Header.Set("Authorization", "Bearer token")
}

func addCSRFCookies(request *http.Request, token string) {
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	request.Header.Set(csrfHeaderName, token)
}

func assertNoAuthCredentialLeak(t *testing.T, value string) {
	t.Helper()
	for _, leaked := range []string{"auth-secret", "token=query-token", "api_key=plain", "password=letmein", "session_id=sess-123", "Bearer auth-secret"} {
		if strings.Contains(value, leaked) {
			t.Fatalf("value leaked %q in %q", leaked, value)
		}
	}
}

func assertDetailCode(t *testing.T, payload []byte, want string) {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["code"] != want {
		t.Fatalf("body code = %q, want %q", body["code"], want)
	}
}

func cookieByName(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("cookie %q not found in %#v", name, cookies)
	return nil
}
