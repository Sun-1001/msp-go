package auth

import (
	"testing"

	"mathstudy/backend-go/internal/domain/user"
)

func TestRoleAuthorizationHelpers(t *testing.T) {
	admin := Principal{UserID: "admin", Role: user.RoleAdmin}
	teacher := Principal{UserID: "teacher", Role: user.RoleTeacher}
	student := Principal{UserID: "student", Role: user.RoleStudent}

	if !IsAdmin(admin) || IsAdmin(teacher) {
		t.Fatal("IsAdmin role check failed")
	}
	if !IsTeacherOrAdmin(admin) || !IsTeacherOrAdmin(teacher) || IsTeacherOrAdmin(student) {
		t.Fatal("IsTeacherOrAdmin role check failed")
	}
	if !IsStudent(student) || IsStudent(admin) {
		t.Fatal("IsStudent role check failed")
	}
}
