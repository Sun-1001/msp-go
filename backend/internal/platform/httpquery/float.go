package httpquery

import (
	"errors"
	"strconv"
)

var (
	ErrInvalidFloat    = errors.New("invalid float query value")
	ErrFloatOutOfRange = errors.New("float query value out of range")
)

// Float parses an optional floating-point query value, returning fallback when value is empty.
func Float(value string, fallback float64) (float64, error) {
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, ErrInvalidFloat
	}
	return parsed, nil
}

// BoundedFloat parses an optional floating-point query value and enforces inclusive bounds.
func BoundedFloat(value string, fallback float64, minValue float64, maxValue float64) (float64, error) {
	parsed, err := Float(value, fallback)
	if err != nil {
		return 0, err
	}
	if parsed < minValue || parsed > maxValue {
		return 0, ErrFloatOutOfRange
	}
	return parsed, nil
}
