package auth

import (
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

func TestNewTokenServiceRejectsUnsupportedAlgorithm(t *testing.T) {
	if _, err := NewTokenService("secret", "RS256", time.Minute, time.Hour); err == nil {
		t.Fatal("NewTokenService(RS256) error = nil, want error")
	}
}
