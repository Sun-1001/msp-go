package httpauth

import (
	"net/http"
	"strings"
)

// BearerToken extracts a single RFC 6750-style bearer token from Authorization.
func BearerToken(r *http.Request) (string, bool) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	fields := strings.Fields(authHeader)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return "", false
	}
	return fields[1], true
}
