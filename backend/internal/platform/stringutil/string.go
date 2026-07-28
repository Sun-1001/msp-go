package stringutil

import "strings"

// NonBlankOr returns fallback when value is empty or only whitespace.
func NonBlankOr(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
