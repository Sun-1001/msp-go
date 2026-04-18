package auth

import "mathstudy/backend-go/internal/domain/user"

// HasAnyRole reports whether a principal has one of the allowed roles.
func HasAnyRole(principal Principal, allowed ...user.Role) bool {
	for _, role := range allowed {
		if principal.Role == role {
			return true
		}
	}
	return false
}

// IsAdmin reports whether the principal has administrator privileges.
func IsAdmin(principal Principal) bool {
	return HasAnyRole(principal, user.RoleAdmin)
}

// IsTeacherOrAdmin reports whether the principal can access teacher/admin endpoints.
func IsTeacherOrAdmin(principal Principal) bool {
	return HasAnyRole(principal, user.RoleTeacher, user.RoleAdmin)
}

// IsStudent reports whether the principal can access student-only endpoints.
func IsStudent(principal Principal) bool {
	return HasAnyRole(principal, user.RoleStudent)
}
