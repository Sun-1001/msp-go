package postgres

import (
	"regexp"
	"testing"
)

func TestNewUUIDReturnsRFC4122Version4Shape(t *testing.T) {
	id, err := newUUID()
	if err != nil {
		t.Fatalf("newUUID() error = %v", err)
	}
	matched := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(id)
	if !matched {
		t.Fatalf("newUUID() = %q", id)
	}
}
