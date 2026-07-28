package redact

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Marker is used when sensitive export data must keep its field shape.
const Marker = "[REDACTED]"

var (
	bearerPattern              = regexp.MustCompile(`(?i)(\bBearer\s+)[A-Za-z0-9._~+/\-]+=*`)
	jwtPattern                 = regexp.MustCompile(`\b[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`)
	sensitiveQueryPattern      = regexp.MustCompile(`(?i)([?&](?:api[_-]?key|access[_-]?token|refresh[_-]?token|auth[_-]?token|csrf[_-]?token|token|password|secret|session|session[_-]?id|cookie)=)[^&#\s]+`)
	sensitiveAssignmentPattern = regexp.MustCompile(`(?i)\b(api[_-]?key|access[_-]?token|refresh[_-]?token|auth[_-]?token|csrf[_-]?token|token|password|secret|authorization|session|session[_-]?id|cookie)(\s*[:=]\s*)([^\s,;]+)`)
)

// Value redacts sensitive keys and credential-looking strings recursively.
func Value(field string, value any) any {
	if SensitiveKey(field) && value != nil {
		return Marker
	}
	switch typed := value.(type) {
	case []byte:
		if json.Valid(typed) {
			var decoded any
			if err := json.Unmarshal(typed, &decoded); err == nil {
				return Value(field, decoded)
			}
		}
		return String(string(typed))
	case string:
		return String(typed)
	case map[string]any:
		sanitized := map[string]any{}
		for key, nested := range typed {
			sanitized[key] = Value(key, nested)
		}
		return sanitized
	case []any:
		sanitized := make([]any, len(typed))
		for index, item := range typed {
			sanitized[index] = Value(field, item)
		}
		return sanitized
	default:
		return typed
	}
}

// String redacts credential-looking substrings while preserving surrounding text.
func String(value string) string {
	value = bearerPattern.ReplaceAllString(value, `${1}`+Marker)
	value = jwtPattern.ReplaceAllString(value, Marker)
	value = sensitiveQueryPattern.ReplaceAllString(value, `${1}`+Marker)
	value = sensitiveAssignmentPattern.ReplaceAllString(value, `${1}${2}`+Marker)
	return value
}

// SensitiveKey reports whether a field name usually carries credentials.
func SensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	for _, marker := range []string{
		"hashed_password",
		"password",
		"passwd",
		"secret",
		"api_key",
		"apikey",
		"access_key",
		"refresh_token",
		"access_token",
		"auth_token",
		"csrf_token",
		"session_id",
		"session_cookie",
		"cookie_value",
		"cookie_secret",
		"authorization",
		"credential",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}
