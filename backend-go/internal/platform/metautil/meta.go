package metautil

import (
	"encoding/json"
	"strconv"

	"mathstudy/backend-go/internal/platform/sliceutil"
)

// LookupString returns a string metadata value and whether it was present as a string.
func LookupString(meta map[string]any, key string) (string, bool) {
	if meta == nil {
		return "", false
	}
	value, ok := meta[key]
	if !ok || value == nil {
		return "", false
	}
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	return text, true
}

// String returns a string metadata value, or an empty string when missing or not a string.
func String(meta map[string]any, key string) string {
	value, ok := LookupString(meta, key)
	if !ok {
		return ""
	}
	return value
}

// StringPointer returns a pointer to a string metadata value when present.
func StringPointer(meta map[string]any, key string) *string {
	value, ok := LookupString(meta, key)
	if !ok {
		return nil
	}
	return &value
}

// LookupStringSlice returns a string slice metadata value and whether a slice was present.
func LookupStringSlice(meta map[string]any, key string) ([]string, bool) {
	if meta == nil {
		return nil, false
	}
	value, exists := meta[key]
	if !exists || value == nil {
		return nil, false
	}
	switch typed := value.(type) {
	case []string:
		return sliceutil.CloneStrings(typed), true
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return result, true
	default:
		return nil, false
	}
}

// StringSlice returns a string slice metadata value, or an empty slice when missing or not a slice.
func StringSlice(meta map[string]any, key string) []string {
	values, ok := LookupStringSlice(meta, key)
	if !ok {
		return []string{}
	}
	return values
}

// OptionalStringSlice returns nil when the metadata key is absent or not a slice.
func OptionalStringSlice(meta map[string]any, key string) []string {
	values, ok := LookupStringSlice(meta, key)
	if !ok {
		return nil
	}
	return values
}

// LookupInt returns an integer metadata value and whether it could be parsed.
func LookupInt(meta map[string]any, key string) (int, bool) {
	if meta == nil {
		return 0, false
	}
	value, ok := meta[key]
	if !ok || value == nil {
		return 0, false
	}
	return intFromAny(value)
}

// Int returns an integer metadata value, or 0 when missing or not numeric.
func Int(meta map[string]any, key string) int {
	return IntDefault(meta, key, 0)
}

// IntDefault returns an integer metadata value, or fallback when missing or not numeric.
func IntDefault(meta map[string]any, key string, fallback int) int {
	value, ok := LookupInt(meta, key)
	if !ok {
		return fallback
	}
	return value
}

// IntPointer returns a pointer to an integer metadata value when present.
func IntPointer(meta map[string]any, key string) *int {
	value, ok := LookupInt(meta, key)
	if !ok {
		return nil
	}
	return &value
}

func intFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed), true
		}
		asFloat, err := strconv.ParseFloat(typed.String(), 64)
		if err != nil {
			return 0, false
		}
		return int(asFloat), true
	default:
		return 0, false
	}
}
