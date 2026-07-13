package httpauth

import "net/http"

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

func writeBearerUnauthorized(w http.ResponseWriter, writeError func(http.ResponseWriter, int, string, string)) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeError(w, http.StatusUnauthorized, unauthorizedCode, unauthorizedMessage)
}
