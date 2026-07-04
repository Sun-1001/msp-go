package metautil

import (
	"encoding/json"
	"testing"
)

func TestLookupString(t *testing.T) {
	tests := []struct {
		name   string
		meta   map[string]any
		want   string
		wantOK bool
	}{
		{name: "nil meta", meta: nil},
		{name: "missing key", meta: map[string]any{}},
		{name: "nil value", meta: map[string]any{"field": nil}},
		{name: "non string", meta: map[string]any{"field": 42}},
		{name: "string", meta: map[string]any{"field": "value"}, want: "value", wantOK: true},
		{name: "empty string is present", meta: map[string]any{"field": ""}, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := LookupString(tt.meta, "field")
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("LookupString() = %q, %v; want %q, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestString(t *testing.T) {
	if got := String(map[string]any{"field": "value"}, "field"); got != "value" {
		t.Fatalf("String() = %q, want value", got)
	}
	if got := String(map[string]any{"field": 42}, "field"); got != "" {
		t.Fatalf("String() = %q, want empty", got)
	}
}

func TestStringPointer(t *testing.T) {
	if got := StringPointer(map[string]any{}, "field"); got != nil {
		t.Fatalf("StringPointer() = %v, want nil", got)
	}
	got := StringPointer(map[string]any{"field": "value"}, "field")
	if got == nil || *got != "value" {
		t.Fatalf("StringPointer() = %v, want value", got)
	}
}

func TestLookupStringSlice(t *testing.T) {
	tests := []struct {
		name   string
		meta   map[string]any
		want   []string
		wantOK bool
	}{
		{name: "nil meta", meta: nil},
		{name: "missing key", meta: map[string]any{}},
		{name: "nil value", meta: map[string]any{"items": nil}},
		{name: "non slice", meta: map[string]any{"items": "one"}},
		{name: "string slice", meta: map[string]any{"items": []string{"one", "two"}}, want: []string{"one", "two"}, wantOK: true},
		{name: "any slice filters strings", meta: map[string]any{"items": []any{"one", 2, "two"}}, want: []string{"one", "two"}, wantOK: true},
		{name: "empty any slice is present", meta: map[string]any{"items": []any{}}, want: []string{}, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := LookupStringSlice(tt.meta, "items")
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			assertStrings(t, got, tt.want)
			if ok && len(got) > 0 {
				got[0] = "changed"
				again, _ := LookupStringSlice(tt.meta, "items")
				if again[0] == "changed" {
					t.Fatal("LookupStringSlice returned slice sharing backing storage")
				}
			}
		})
	}
}

func TestStringSlice(t *testing.T) {
	got := StringSlice(map[string]any{"items": []any{"one"}}, "items")
	assertStrings(t, got, []string{"one"})

	got = StringSlice(map[string]any{"items": 42}, "items")
	if got == nil || len(got) != 0 {
		t.Fatalf("StringSlice() = %#v, want non-nil empty slice", got)
	}
}

func TestOptionalStringSlice(t *testing.T) {
	if got := OptionalStringSlice(map[string]any{}, "items"); got != nil {
		t.Fatalf("OptionalStringSlice() = %#v, want nil", got)
	}
	got := OptionalStringSlice(map[string]any{"items": []any{}}, "items")
	if got == nil || len(got) != 0 {
		t.Fatalf("OptionalStringSlice() = %#v, want non-nil empty slice for present value", got)
	}
}

func TestLookupInt(t *testing.T) {
	tests := []struct {
		name   string
		meta   map[string]any
		want   int
		wantOK bool
	}{
		{name: "nil meta", meta: nil},
		{name: "missing key", meta: map[string]any{}},
		{name: "nil value", meta: map[string]any{"field": nil}},
		{name: "non numeric", meta: map[string]any{"field": "7"}},
		{name: "int", meta: map[string]any{"field": 7}, want: 7, wantOK: true},
		{name: "int32", meta: map[string]any{"field": int32(8)}, want: 8, wantOK: true},
		{name: "int64", meta: map[string]any{"field": int64(9)}, want: 9, wantOK: true},
		{name: "float64 truncates", meta: map[string]any{"field": 10.9}, want: 10, wantOK: true},
		{name: "json integer number", meta: map[string]any{"field": json.Number("11")}, want: 11, wantOK: true},
		{name: "json float number truncates", meta: map[string]any{"field": json.Number("12.8")}, want: 12, wantOK: true},
		{name: "invalid json number", meta: map[string]any{"field": json.Number("bad")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := LookupInt(tt.meta, "field")
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("LookupInt() = %d, %v; want %d, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestIntAndIntDefault(t *testing.T) {
	if got := Int(map[string]any{"field": int64(42)}, "field"); got != 42 {
		t.Fatalf("Int() = %d, want 42", got)
	}
	if got := Int(map[string]any{"field": "42"}, "field"); got != 0 {
		t.Fatalf("Int(non numeric) = %d, want 0", got)
	}
	if got := IntDefault(map[string]any{"field": "42"}, "field", 7); got != 7 {
		t.Fatalf("IntDefault(non numeric) = %d, want 7", got)
	}
}

func TestIntPointer(t *testing.T) {
	if got := IntPointer(map[string]any{}, "field"); got != nil {
		t.Fatalf("IntPointer() = %v, want nil", got)
	}
	got := IntPointer(map[string]any{"field": 13.9}, "field")
	if got == nil || *got != 13 {
		t.Fatalf("IntPointer() = %v, want 13", got)
	}
}

func assertStrings(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
