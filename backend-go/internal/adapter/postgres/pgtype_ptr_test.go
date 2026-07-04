package postgres

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestTextPtr(t *testing.T) {
	got := textPtr(pgtype.Text{String: "value", Valid: true})
	if got == nil || *got != "value" {
		t.Fatalf("textPtr(valid) = %#v, want value", got)
	}
	if got := textPtr(pgtype.Text{}); got != nil {
		t.Fatalf("textPtr(invalid) = %#v, want nil", got)
	}
}

func TestBoolPtr(t *testing.T) {
	got := boolPtr(pgtype.Bool{Bool: true, Valid: true})
	if got == nil || *got != true {
		t.Fatalf("boolPtr(valid true) = %#v, want true", got)
	}
	got = boolPtr(pgtype.Bool{Bool: false, Valid: true})
	if got == nil || *got != false {
		t.Fatalf("boolPtr(valid false) = %#v, want false", got)
	}
	if got := boolPtr(pgtype.Bool{}); got != nil {
		t.Fatalf("boolPtr(invalid) = %#v, want nil", got)
	}
}

func TestIntPtr(t *testing.T) {
	got := intPtr(pgtype.Int4{Int32: 42, Valid: true})
	if got == nil || *got != 42 {
		t.Fatalf("intPtr(valid) = %#v, want 42", got)
	}
	if got := intPtr(pgtype.Int4{}); got != nil {
		t.Fatalf("intPtr(invalid) = %#v, want nil", got)
	}
}

func TestFloatPtr(t *testing.T) {
	got := floatPtr(pgtype.Float8{Float64: 0.75, Valid: true})
	if got == nil || *got != 0.75 {
		t.Fatalf("floatPtr(valid) = %#v, want 0.75", got)
	}
	if got := floatPtr(pgtype.Float8{}); got != nil {
		t.Fatalf("floatPtr(invalid) = %#v, want nil", got)
	}
}

func TestTimestampPtr(t *testing.T) {
	now := time.Date(2026, 7, 4, 10, 20, 30, 0, time.UTC)
	got := timestampPtr(pgtype.Timestamp{Time: now, Valid: true})
	if got == nil || !got.Equal(now) {
		t.Fatalf("timestampPtr(valid) = %#v, want %s", got, now)
	}
	if got := timestampPtr(pgtype.Timestamp{}); got != nil {
		t.Fatalf("timestampPtr(invalid) = %#v, want nil", got)
	}
}
