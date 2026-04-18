package user

import (
	"fmt"
	"strings"
	"time"
)

// Role is the public lower-case user role used by API responses and JWT claims.
type Role string

const (
	RoleStudent Role = "student"
	RoleTeacher Role = "teacher"
	RoleAdmin   Role = "admin"
)

// ParseRole normalizes either API values or PostgreSQL enum names into a Role.
func ParseRole(value string) (Role, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(RoleStudent):
		return RoleStudent, nil
	case string(RoleTeacher):
		return RoleTeacher, nil
	case string(RoleAdmin):
		return RoleAdmin, nil
	default:
		return "", fmt.Errorf("unknown user role %q", value)
	}
}

// DBValue returns the PostgreSQL enum representation used by the migrated schema.
func (r Role) DBValue() string {
	return strings.ToUpper(string(r))
}

// Status is the public lower-case account status.
type Status string

const (
	StatusActive    Status = "active"
	StatusInactive  Status = "inactive"
	StatusSuspended Status = "suspended"
)

// ParseStatus normalizes either API values or PostgreSQL enum names into a Status.
func ParseStatus(value string) (Status, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(StatusActive):
		return StatusActive, nil
	case string(StatusInactive):
		return StatusInactive, nil
	case string(StatusSuspended):
		return StatusSuspended, nil
	default:
		return "", fmt.Errorf("unknown user status %q", value)
	}
}

// DBValue returns the PostgreSQL enum representation used by the migrated schema.
func (s Status) DBValue() string {
	return strings.ToUpper(string(s))
}

// User is the user account shape shared by auth and later user-domain modules.
type User struct {
	ID             string
	Username       string
	Email          string
	HashedPassword string
	Role           Role
	DisplayName    *string
	AvatarURL      *string
	IsActive       bool
	Status         Status
	LastLoginAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CreateUser contains fields required to persist a new user.
type CreateUser struct {
	ID             string
	Username       string
	Email          string
	HashedPassword string
	Role           Role
	DisplayName    *string
	IsActive       bool
	Status         Status
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
