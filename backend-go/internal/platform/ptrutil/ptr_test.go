package ptrutil

import "testing"

func TestClone(t *testing.T) {
	source := "value"
	got := Clone(&source)

	if got == nil || *got != "value" {
		t.Fatalf("Clone() = %#v, want pointer to value", got)
	}
	*got = "changed"
	if source != "value" {
		t.Fatalf("Clone() did not copy pointed value")
	}
}

func TestCloneNil(t *testing.T) {
	if got := Clone[string](nil); got != nil {
		t.Fatalf("Clone(nil) = %#v, want nil", got)
	}
}

func TestValueOrZero(t *testing.T) {
	source := "value"
	if got := ValueOrZero(&source); got != "value" {
		t.Fatalf("ValueOrZero() = %q, want value", got)
	}
}

func TestValueOrZeroNil(t *testing.T) {
	if got := ValueOrZero[string](nil); got != "" {
		t.Fatalf("ValueOrZero(nil string) = %q, want empty", got)
	}
	if got := ValueOrZero[int](nil); got != 0 {
		t.Fatalf("ValueOrZero(nil int) = %d, want 0", got)
	}
}

func TestValueOrDefault(t *testing.T) {
	source := 0.3
	if got := ValueOrDefault(&source, 0.7); got != 0.3 {
		t.Fatalf("ValueOrDefault() = %v, want 0.3", got)
	}
}

func TestValueOrDefaultNil(t *testing.T) {
	if got := ValueOrDefault[int](nil, 60); got != 60 {
		t.Fatalf("ValueOrDefault(nil int) = %d, want 60", got)
	}
}
