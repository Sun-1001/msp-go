package httpquery

import (
	"errors"
	"strconv"
)

var (
	// ErrInvalidInt is returned when a query value is present but not an integer.
	ErrInvalidInt = errors.New("invalid integer query value")
	// ErrIntOutOfRange is returned when a parsed integer falls outside the allowed range.
	ErrIntOutOfRange = errors.New("integer query value out of range")
)

// Int parses an optional integer query value, returning fallback when value is empty.
func Int(value string, fallback int) (int, error) {
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, ErrInvalidInt
	}
	return parsed, nil
}

// BoundedInt parses an optional integer query value and enforces inclusive bounds.
func BoundedInt(value string, fallback int, minValue int, maxValue int) (int, error) {
	parsed, err := Int(value, fallback)
	if err != nil {
		return 0, err
	}
	if parsed < minValue || parsed > maxValue {
		return 0, ErrIntOutOfRange
	}
	return parsed, nil
}
