package auth

import (
	"strings"
	"testing"
)

func TestValidatePasswordStrengthReportsPythonCompatibleErrors(t *testing.T) {
	errors := ValidatePasswordStrength("password")
	joined := strings.Join(errors, "；")
	for _, want := range []string{
		"密码必须包含至少1个大写字母",
		"密码必须包含至少1个数字",
		"密码必须包含至少1个特殊字符",
		"密码过于常见，请使用更复杂的密码",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("ValidatePasswordStrength missing %q in %q", want, joined)
		}
	}
}

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("Strong1!")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if !VerifyPassword("Strong1!", hash) {
		t.Fatal("VerifyPassword() = false, want true")
	}
	if VerifyPassword("Wrong1!", hash) {
		t.Fatal("VerifyPassword(wrong) = true, want false")
	}
}
