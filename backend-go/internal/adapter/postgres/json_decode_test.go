package postgres

import (
	"reflect"
	"testing"
)

func TestDecodeFloatMap(t *testing.T) {
	got, err := decodeFloatMap([]byte(`{"a":0.25,"b":0.75}`))
	if err != nil {
		t.Fatalf("decodeFloatMap() error = %v", err)
	}
	want := map[string]float64{"a": 0.25, "b": 0.75}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decodeFloatMap() = %#v, want %#v", got, want)
	}
}

func TestDecodeFloatMapEmpty(t *testing.T) {
	got, err := decodeFloatMap(nil)
	if err != nil {
		t.Fatalf("decodeFloatMap(nil) error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("decodeFloatMap(nil) = %#v, want empty map", got)
	}
}

func TestDecodeStringSlice(t *testing.T) {
	got, err := decodeStringSlice([]byte(`["a","b"]`))
	if err != nil {
		t.Fatalf("decodeStringSlice() error = %v", err)
	}
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decodeStringSlice() = %#v, want %#v", got, want)
	}
}

func TestDecodeStringSliceEmpty(t *testing.T) {
	got, err := decodeStringSlice(nil)
	if err != nil {
		t.Fatalf("decodeStringSlice(nil) error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("decodeStringSlice(nil) = %#v, want empty slice", got)
	}
}

func TestDecodeObjectMap(t *testing.T) {
	got, err := decodeObjectMap([]byte(`{"type":"video"}`))
	if err != nil {
		t.Fatalf("decodeObjectMap() error = %v", err)
	}
	want := map[string]any{"type": "video"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decodeObjectMap() = %#v, want %#v", got, want)
	}
}

func TestDecodeObjectMapEmpty(t *testing.T) {
	got, err := decodeObjectMap(nil)
	if err != nil {
		t.Fatalf("decodeObjectMap(nil) error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("decodeObjectMap(nil) = %#v, want empty map", got)
	}
}

func TestDecodeObjectMapNull(t *testing.T) {
	got, err := decodeObjectMap([]byte(`null`))
	if err != nil {
		t.Fatalf("decodeObjectMap(null) error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("decodeObjectMap(null) = %#v, want empty map", got)
	}
}

func TestDecodeInvalidJSON(t *testing.T) {
	if _, err := decodeFloatMap([]byte(`{`)); err == nil {
		t.Fatal("decodeFloatMap(invalid) error = nil, want error")
	}
	if _, err := decodeStringSlice([]byte(`{`)); err == nil {
		t.Fatal("decodeStringSlice(invalid) error = nil, want error")
	}
	if _, err := decodeObjectMap([]byte(`{`)); err == nil {
		t.Fatal("decodeObjectMap(invalid) error = nil, want error")
	}
}
