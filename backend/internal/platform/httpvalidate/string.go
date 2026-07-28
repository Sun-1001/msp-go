package httpvalidate

import (
	"net/http"
	"strings"
)

const validationErrorCode = "VALIDATION_ERROR"

// ErrorWriter writes an application-specific HTTP error body.
type ErrorWriter func(http.ResponseWriter, int, string, string)

// RequiredTrimmedString validates a required string using trimmed byte length.
func RequiredTrimmedString(w http.ResponseWriter, value string, min int, max int, name string, write ErrorWriter) bool {
	length := len(strings.TrimSpace(value))
	if length < min {
		write(w, http.StatusUnprocessableEntity, validationErrorCode, name+" 不能为空")
		return false
	}
	if max > 0 && length > max {
		write(w, http.StatusUnprocessableEntity, validationErrorCode, name+" 长度超出限制")
		return false
	}
	return true
}

// StringLength validates an optional plain string value without trimming it.
func StringLength(w http.ResponseWriter, value string, max int, name string, write ErrorWriter) bool {
	if len(value) <= max {
		return true
	}
	write(w, http.StatusUnprocessableEntity, validationErrorCode, name+" 长度超出限制")
	return false
}

// OptionalString validates an optional plain string pointer without trimming it.
func OptionalString(w http.ResponseWriter, value *string, max int, name string, write ErrorWriter) bool {
	if value == nil {
		return true
	}
	return StringLength(w, *value, max, name, write)
}

// OptionalRequiredTrimmedString validates a present string pointer with required trimmed length rules.
func OptionalRequiredTrimmedString(w http.ResponseWriter, value *string, min int, max int, name string, write ErrorWriter) bool {
	if value == nil {
		return true
	}
	return RequiredTrimmedString(w, *value, min, max, name, write)
}

// RequiredField validates that a JSON pointer field was present.
func RequiredField[T any](w http.ResponseWriter, value *T, name string, write ErrorWriter) bool {
	if value != nil {
		return true
	}
	write(w, http.StatusUnprocessableEntity, validationErrorCode, name+" 为必填字段")
	return false
}

// StringSlice validates list length and each trimmed item byte length.
func StringSlice(w http.ResponseWriter, values []string, maxItems int, maxItemBytes int, name string, write ErrorWriter) bool {
	if len(values) > maxItems {
		write(w, http.StatusUnprocessableEntity, validationErrorCode, name+" 数量超出限制")
		return false
	}
	for _, value := range values {
		if len(strings.TrimSpace(value)) > maxItemBytes {
			write(w, http.StatusUnprocessableEntity, validationErrorCode, name+" 单项长度超出限制")
			return false
		}
	}
	return true
}

// OptionalStringSlice validates a present string slice pointer.
func OptionalStringSlice(w http.ResponseWriter, values *[]string, maxItems int, maxItemBytes int, name string, write ErrorWriter) bool {
	if values == nil {
		return true
	}
	return StringSlice(w, *values, maxItems, maxItemBytes, name, write)
}
