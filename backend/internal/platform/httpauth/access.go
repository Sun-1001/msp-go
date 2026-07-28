package httpauth

import (
	"context"
	"net/http"
)

const (
	unauthorizedCode    = "UNAUTHORIZED"
	unauthorizedMessage = "未认证，请先登录"
	forbiddenCode       = "FORBIDDEN"
)

// RequireBearerAccess decodes a bearer token and optionally enforces an authorization predicate.
func RequireBearerAccess[T any](
	w http.ResponseWriter,
	r *http.Request,
	decode func(string) (T, bool),
	allow func(T) bool,
	forbiddenMessage string,
	writeError func(http.ResponseWriter, int, string, string),
) (T, bool) {
	var zero T
	token, ok := BearerToken(r)
	if !ok {
		writeBearerUnauthorized(w, writeError)
		return zero, false
	}

	principal, ok := decode(token)
	if !ok {
		writeBearerUnauthorized(w, writeError)
		return zero, false
	}
	if allow != nil && !allow(principal) {
		writeError(w, http.StatusForbidden, forbiddenCode, forbiddenMessage)
		return zero, false
	}
	return principal, true
}

// RequireBearerAccessContext is the request-aware variant used when decoding
// also checks current server-side account state.
func RequireBearerAccessContext[T any](
	w http.ResponseWriter,
	r *http.Request,
	decode func(context.Context, string) (T, bool, error),
	allow func(T) bool,
	forbiddenMessage string,
	writeError func(http.ResponseWriter, int, string, string),
	onDecodeError func(error),
) (T, bool) {
	var zero T
	token, ok := BearerToken(r)
	if !ok {
		writeBearerUnauthorized(w, writeError)
		return zero, false
	}

	principal, ok, err := decode(r.Context(), token)
	if err != nil {
		if onDecodeError != nil {
			onDecodeError(err)
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "验证登录状态失败，请稍后重试")
		return zero, false
	}
	if !ok {
		writeBearerUnauthorized(w, writeError)
		return zero, false
	}
	if allow != nil && !allow(principal) {
		writeError(w, http.StatusForbidden, forbiddenCode, forbiddenMessage)
		return zero, false
	}
	return principal, true
}

func writeBearerUnauthorized(w http.ResponseWriter, writeError func(http.ResponseWriter, int, string, string)) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeError(w, http.StatusUnauthorized, unauthorizedCode, unauthorizedMessage)
}
