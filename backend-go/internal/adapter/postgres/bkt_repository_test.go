package postgres

import "testing"

func TestNewBKTRepositoryRejectsNilQuerier(t *testing.T) {
	if _, err := NewBKTRepository(nil); err == nil {
		t.Fatal("NewBKTRepository(nil) error = nil, want error")
	}
}

func TestOptionalFloat(t *testing.T) {
	if optionalFloat(nil) != nil {
		t.Fatal("optionalFloat(nil) != nil")
	}
	value := 0.25
	if got := optionalFloat(&value); got != value {
		t.Fatalf("optionalFloat() = %#v", got)
	}
}
