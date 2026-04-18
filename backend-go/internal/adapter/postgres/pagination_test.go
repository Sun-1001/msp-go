package postgres

import "testing"

func TestNewPageDefaultsAndCapsLimit(t *testing.T) {
	page, err := NewPage(10, 0)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	if page.Offset != 10 || page.Limit != DefaultLimit {
		t.Fatalf("NewPage() = %#v, want offset 10 default limit", page)
	}

	page, err = NewPage(0, MaxLimit+1)
	if err != nil {
		t.Fatalf("NewPage() cap error = %v", err)
	}
	if page.Limit != MaxLimit {
		t.Fatalf("Limit = %d, want %d", page.Limit, MaxLimit)
	}
}

func TestNewPageRejectsNegativeValues(t *testing.T) {
	if _, err := NewPage(-1, 10); err == nil {
		t.Fatal("NewPage(-1, 10) error = nil, want error")
	}
	if _, err := NewPage(0, -1); err == nil {
		t.Fatal("NewPage(0, -1) error = nil, want error")
	}
}
