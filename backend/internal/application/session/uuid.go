package session

import "mathstudy/backend/internal/platform/identifier"

// NewUUID returns a random RFC 4122 version 4 UUID string.
func NewUUID() (string, error) {
	return identifier.NewUUID()
}
