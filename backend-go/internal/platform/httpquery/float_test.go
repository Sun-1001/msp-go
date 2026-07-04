package httpquery

import (
	"errors"
	"testing"
)

func TestFloat(t *testing.T) {
	value, err := Float("", 0.5)
	if err != nil || value != 0.5 {
		t.Fatalf("Float(empty) = %v, %v; want 0.5, nil", value, err)
	}

	value, err = Float("0.25", 0.5)
	if err != nil || value != 0.25 {
		t.Fatalf("Float(value) = %v, %v; want 0.25, nil", value, err)
	}

	if _, err = Float("bad", 0.5); !errors.Is(err, ErrInvalidFloat) {
		t.Fatalf("Float(bad) error = %v, want ErrInvalidFloat", err)
	}
}

func TestBoundedFloat(t *testing.T) {
	value, err := BoundedFloat("0.75", 0.5, 0, 1)
	if err != nil || value != 0.75 {
		t.Fatalf("BoundedFloat(value) = %v, %v; want 0.75, nil", value, err)
	}

	value, err = BoundedFloat("", 0.5, 0, 1)
	if err != nil || value != 0.5 {
		t.Fatalf("BoundedFloat(empty) = %v, %v; want 0.5, nil", value, err)
	}

	if _, err = BoundedFloat("-0.1", 0.5, 0, 1); !errors.Is(err, ErrFloatOutOfRange) {
		t.Fatalf("BoundedFloat(low) error = %v, want ErrFloatOutOfRange", err)
	}

	if _, err = BoundedFloat("1.1", 0.5, 0, 1); !errors.Is(err, ErrFloatOutOfRange) {
		t.Fatalf("BoundedFloat(high) error = %v, want ErrFloatOutOfRange", err)
	}

	if _, err = BoundedFloat("bad", 0.5, 0, 1); !errors.Is(err, ErrInvalidFloat) {
		t.Fatalf("BoundedFloat(bad) error = %v, want ErrInvalidFloat", err)
	}
}
