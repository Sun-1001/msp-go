package httpquery

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidTime = errors.New("invalid time query value")

// OptionalTime parses an optional time query value using the provided layouts.
func OptionalTime(value string, layouts ...string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return &parsed, nil
		}
	}
	return nil, ErrInvalidTime
}
