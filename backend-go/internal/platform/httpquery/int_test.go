package httpquery

import (
	"errors"
	"testing"
)

func TestInt(t *testing.T) {
	value, err := Int("", 20)
	if err != nil || value != 20 {
		t.Fatalf("Int(empty) = %d, %v; want 20, nil", value, err)
	}
	value, err = Int("12", 20)
	if err != nil || value != 12 {
		t.Fatalf("Int(12) = %d, %v; want 12, nil", value, err)
	}
	if _, err = Int("bad", 20); !errors.Is(err, ErrInvalidInt) {
		t.Fatalf("Int(bad) error = %v, want ErrInvalidInt", err)
	}
}

func TestBoundedInt(t *testing.T) {
	value, err := BoundedInt("5", 10, 1, 10)
	if err != nil || value != 5 {
		t.Fatalf("BoundedInt(5) = %d, %v; want 5, nil", value, err)
	}
	value, err = BoundedInt("", 10, 1, 10)
	if err != nil || value != 10 {
		t.Fatalf("BoundedInt(empty) = %d, %v; want 10, nil", value, err)
	}
	if _, err = BoundedInt("0", 10, 1, 10); !errors.Is(err, ErrIntOutOfRange) {
		t.Fatalf("BoundedInt(0) error = %v, want ErrIntOutOfRange", err)
	}
	if _, err = BoundedInt("11", 10, 1, 10); !errors.Is(err, ErrIntOutOfRange) {
		t.Fatalf("BoundedInt(11) error = %v, want ErrIntOutOfRange", err)
	}
	if _, err = BoundedInt("bad", 10, 1, 10); !errors.Is(err, ErrInvalidInt) {
		t.Fatalf("BoundedInt(bad) error = %v, want ErrInvalidInt", err)
	}
}
