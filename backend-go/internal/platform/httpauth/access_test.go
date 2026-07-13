package httpauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type accessPrincipal struct {
	ID      string
	Allowed bool
}

type accessError struct {
	status  int
	code    string
	message string
}

func TestRequireBearerAccessRejectsMissingToken(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	decodeCalled := false
	var written accessError

	principal, ok := RequireBearerAccess(response, request, func(string) (accessPrincipal, bool) {
		decodeCalled = true
		return accessPrincipal{}, false
	}, func(accessPrincipal) bool {
		return true
	}, "forbidden", captureAccessError(&written))

	if ok || principal != (accessPrincipal{}) {
		t.Fatalf("RequireBearerAccess() = %#v, %t; want zero principal, false", principal, ok)
	}
	if decodeCalled {
		t.Fatal("RequireBearerAccess() called decoder without a token")
	}
	if response.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q, want Bearer", response.Header().Get("WWW-Authenticate"))
	}
	assertAccessError(t, written, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
}

func TestRequireBearerAccessRejectsInvalidToken(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer invalid-token")
	response := httptest.NewRecorder()
	decodedToken := ""
	var written accessError

	_, ok := RequireBearerAccess(response, request, func(token string) (accessPrincipal, bool) {
		decodedToken = token
		return accessPrincipal{}, false
	}, func(accessPrincipal) bool {
		return true
	}, "forbidden", captureAccessError(&written))

	if ok {
		t.Fatal("RequireBearerAccess() ok = true, want false")
	}
	if decodedToken != "invalid-token" {
		t.Fatalf("decoded token = %q, want invalid-token", decodedToken)
	}
	if response.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q, want Bearer", response.Header().Get("WWW-Authenticate"))
	}
	assertAccessError(t, written, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
}

func TestRequireBearerAccessRejectsForbiddenPrincipal(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	var written accessError

	principal, ok := RequireBearerAccess(response, request, func(string) (accessPrincipal, bool) {
		return accessPrincipal{ID: "user-1"}, true
	}, func(principal accessPrincipal) bool {
		return principal.Allowed
	}, "需要指定权限", captureAccessError(&written))

	if ok || principal != (accessPrincipal{}) {
		t.Fatalf("RequireBearerAccess() = %#v, %t; want zero principal, false", principal, ok)
	}
	if response.Header().Get("WWW-Authenticate") != "" {
		t.Fatalf("WWW-Authenticate = %q, want empty", response.Header().Get("WWW-Authenticate"))
	}
	assertAccessError(t, written, http.StatusForbidden, "FORBIDDEN", "需要指定权限")
}

func TestRequireBearerAccessReturnsAllowedPrincipal(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	var written accessError
	want := accessPrincipal{ID: "user-1", Allowed: true}

	principal, ok := RequireBearerAccess(response, request, func(string) (accessPrincipal, bool) {
		return want, true
	}, func(principal accessPrincipal) bool {
		return principal.Allowed
	}, "forbidden", captureAccessError(&written))

	if !ok || principal != want {
		t.Fatalf("RequireBearerAccess() = %#v, %t; want %#v, true", principal, ok, want)
	}
	if written != (accessError{}) {
		t.Fatalf("written error = %#v, want zero", written)
	}
}

func TestRequireBearerAccessAllowsAuthenticatedPrincipalWithoutPredicate(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	var written accessError
	want := accessPrincipal{ID: "user-1"}

	principal, ok := RequireBearerAccess(
		response,
		request,
		func(string) (accessPrincipal, bool) { return want, true },
		nil,
		"",
		captureAccessError(&written),
	)

	if !ok || principal != want {
		t.Fatalf("RequireBearerAccess() = %#v, %t; want %#v, true", principal, ok, want)
	}
	if written != (accessError{}) {
		t.Fatalf("written error = %#v, want zero", written)
	}
}

func captureAccessError(target *accessError) func(http.ResponseWriter, int, string, string) {
	return func(_ http.ResponseWriter, status int, code string, message string) {
		*target = accessError{status: status, code: code, message: message}
	}
}

func assertAccessError(t *testing.T, got accessError, status int, code string, message string) {
	t.Helper()
	want := accessError{status: status, code: code, message: message}
	if got != want {
		t.Fatalf("written error = %#v, want %#v", got, want)
	}
}
