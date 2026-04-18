package auth

import (
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var (
	commonPasswords = map[string]struct{}{
		"password": {}, "12345678": {}, "123456789": {}, "1234567890": {},
		"qwerty123": {}, "admin123": {}, "password1": {}, "iloveyou": {},
		"sunshine1": {}, "princess1": {}, "football1": {}, "charlie1": {},
		"access14": {}, "master12": {}, "dragon12": {}, "monkey12": {},
		"letmein1": {}, "abc12345": {}, "qwerty12": {}, "trustno1": {},
	}
	upperRegexp   = regexp.MustCompile(`[A-Z]`)
	lowerRegexp   = regexp.MustCompile(`[a-z]`)
	digitRegexp   = regexp.MustCompile(`\d`)
	specialRegexp = regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>/?` + "`" + `~]`)
)

// ValidatePasswordStrength mirrors the Python password policy for registration and changes.
func ValidatePasswordStrength(password string) []string {
	var errors []string
	if len(password) < 8 {
		errors = append(errors, "密码长度不能少于8位")
	}
	if len(password) > 128 {
		errors = append(errors, "密码长度不能超过128位")
	}
	if !upperRegexp.MatchString(password) {
		errors = append(errors, "密码必须包含至少1个大写字母")
	}
	if !lowerRegexp.MatchString(password) {
		errors = append(errors, "密码必须包含至少1个小写字母")
	}
	if !digitRegexp.MatchString(password) {
		errors = append(errors, "密码必须包含至少1个数字")
	}
	if !specialRegexp.MatchString(password) {
		errors = append(errors, "密码必须包含至少1个特殊字符")
	}
	if _, ok := commonPasswords[strings.ToLower(password)]; ok {
		errors = append(errors, "密码过于常见，请使用更复杂的密码")
	}
	return errors
}

// HashPassword returns a bcrypt hash compatible with the Python bcrypt implementation.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword checks a plaintext password against a bcrypt hash.
func VerifyPassword(password string, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
