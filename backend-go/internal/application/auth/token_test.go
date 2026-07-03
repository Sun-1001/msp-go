package auth

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"mathstudy/backend-go/internal/domain/user"
)

func TestTokenServiceCreatesCompatibleAccessClaims(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	service, err := newTokenServiceWithClock("secret", "HS256", 30*time.Minute, 7*24*time.Hour, func() time.Time {
		return now
	})
	if err != nil {
		t.Fatalf("newTokenServiceWithClock() error = %v", err)
	}

	token, err := service.CreateAccessToken("user-1", user.RoleTeacher)
	if err != nil {
		t.Fatalf("CreateAccessToken() error = %v", err)
	}

	claims, err := service.Decode(token)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if claims.Subject != "user-1" || claims.Role != user.RoleTeacher || claims.Type != "access" {
		t.Fatalf("claims = %#v", claims)
	}
	if claims.Issued != now || claims.Expires != now.Add(30*time.Minute) {
		t.Fatalf("claim times = %s/%s", claims.Issued, claims.Expires)
	}
	if len(claims.JTI) != 32 {
		t.Fatalf("jti length = %d, want 32", len(claims.JTI))
	}
}

func TestTokenServiceRejectsWrongAlgorithmAndExpiredToken(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	service, err := newTokenServiceWithClock("secret", "HS256", time.Minute, time.Hour, func() time.Time {
		return now
	})
	if err != nil {
		t.Fatalf("newTokenServiceWithClock() error = %v", err)
	}
	token, err := service.CreateAccessToken("user-1", user.RoleStudent)
	if err != nil {
		t.Fatalf("CreateAccessToken() error = %v", err)
	}

	wrongAlg := strings.Replace(token, "NiJ9.", "NiJ9.", 1)
	if _, err := service.Decode(wrongAlg[:len(wrongAlg)-1] + "x"); err == nil {
		t.Fatal("Decode(tampered) error = nil, want error")
	}

	expired, err := newTokenServiceWithClock("secret", "HS256", time.Minute, time.Hour, func() time.Time {
		return now.Add(2 * time.Minute)
	})
	if err != nil {
		t.Fatalf("newTokenServiceWithClock(expired) error = %v", err)
	}
	if _, err := expired.Decode(token); err == nil {
		t.Fatal("Decode(expired) error = nil, want error")
	}
}

func TestTokenServiceRejectsTrailingJSONSegments(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	service, err := newTokenServiceWithClock("secret", "HS256", time.Minute, time.Hour, func() time.Time {
		return now
	})
	if err != nil {
		t.Fatalf("newTokenServiceWithClock() error = %v", err)
	}

	validHeader := `{"alg":"HS256","typ":"JWT"}`
	validClaims := `{"aud":"msp-api","exp":1700000060,"iat":1700000000,"iss":"math-study-platform","jti":"1234567890abcdef1234567890abcdef","role":"student","sub":"user-1","type":"access"}`
	tests := []struct {
		name   string
		header string
		claims string
	}{
		{name: "header", header: validHeader + ` {}`, claims: validClaims},
		{name: "claims", header: validHeader, claims: validClaims + ` {}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := signedTestToken(t, service, tt.header, tt.claims)
			if _, err := service.Decode(token); err == nil {
				t.Fatal("Decode(trailing JSON segment) error = nil, want error")
			}
		})
	}
}

func TestNewTokenServiceRejectsUnsupportedAlgorithm(t *testing.T) {
	if _, err := NewTokenService("secret", "RS256", time.Minute, time.Hour); err == nil {
		t.Fatal("NewTokenService(RS256) error = nil, want error")
	}
}

func signedTestToken(t *testing.T, service TokenService, header string, claims string) string {
	t.Helper()
	unsigned := base64.RawURLEncoding.EncodeToString([]byte(header)) + "." +
		base64.RawURLEncoding.EncodeToString([]byte(claims))
	signature, err := service.sign(unsigned)
	if err != nil {
		t.Fatalf("sign() error = %v", err)
	}
	return unsigned + "." + signature
}
