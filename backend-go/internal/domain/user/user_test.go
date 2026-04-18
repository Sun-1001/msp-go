package user

import "testing"

func TestParseRoleAcceptsAPIAndDatabaseValues(t *testing.T) {
	tests := map[string]Role{
		"student": RoleStudent,
		"STUDENT": RoleStudent,
		"teacher": RoleTeacher,
		"ADMIN":   RoleAdmin,
	}
	for input, want := range tests {
		got, err := ParseRole(input)
		if err != nil {
			t.Fatalf("ParseRole(%q) error = %v", input, err)
		}
		if got != want {
			t.Fatalf("ParseRole(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRoleDBValueUsesMigratedEnumName(t *testing.T) {
	if got := RoleTeacher.DBValue(); got != "TEACHER" {
		t.Fatalf("RoleTeacher.DBValue() = %q", got)
	}
}

func TestParseStatusAcceptsAPIAndDatabaseValues(t *testing.T) {
	got, err := ParseStatus("SUSPENDED")
	if err != nil {
		t.Fatalf("ParseStatus() error = %v", err)
	}
	if got != StatusSuspended {
		t.Fatalf("ParseStatus() = %q", got)
	}
	if db := got.DBValue(); db != "SUSPENDED" {
		t.Fatalf("Status.DBValue() = %q", db)
	}
}
