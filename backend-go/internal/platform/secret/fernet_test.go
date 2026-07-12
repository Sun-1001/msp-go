package secret

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

const testFernetKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

func TestFernetEncryptDecryptRoundTrip(t *testing.T) {
	service, err := NewFernet(testFernetKey)
	if err != nil {
		t.Fatalf("NewFernet() error = %v", err)
	}
	service.now = func() time.Time { return time.Unix(1778060000, 0).UTC() }
	service.random = bytes.NewReader([]byte("0123456789abcdef"))

	token, err := service.Encrypt("secret-password")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	plaintext, err := service.Decrypt(token)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if plaintext != "secret-password" {
		t.Fatalf("plaintext = %q", plaintext)
	}
}

func TestFernetRejectsInvalidToken(t *testing.T) {
	service, err := NewFernet(testFernetKey)
	if err != nil {
		t.Fatalf("NewFernet() error = %v", err)
	}
	token, err := service.Encrypt("secret-password")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	tampered := strings.TrimRight(token, "=") + "A="
	if _, err := service.Decrypt(tampered); err == nil {
		t.Fatal("Decrypt(tampered) error = nil, want error")
	}
}

func TestFernetRejectsTimestampBeforeUnixEpoch(t *testing.T) {
	service, err := NewFernet(testFernetKey)
	if err != nil {
		t.Fatalf("NewFernet() error = %v", err)
	}
	service.now = func() time.Time { return time.Unix(-1, 0).UTC() }

	if _, err := service.Encrypt("secret-password"); err == nil {
		t.Fatal("Encrypt() error = nil, want invalid timestamp error")
	}
}

func TestNewFernetRejectsInvalidKey(t *testing.T) {
	if _, err := NewFernet("bad-key"); err == nil {
		t.Fatal("NewFernet(bad-key) error = nil, want error")
	}
}
