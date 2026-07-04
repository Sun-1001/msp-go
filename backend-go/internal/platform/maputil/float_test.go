package maputil

import (
	"reflect"
	"testing"
)

func TestCloneFloatMap(t *testing.T) {
	source := map[string]float64{"a": 0.1, "b": 0.2}
	got := CloneFloatMap(source)

	if !reflect.DeepEqual(got, source) {
		t.Fatalf("CloneFloatMap() = %#v, want %#v", got, source)
	}
	got["a"] = 0.9
	if source["a"] != 0.1 {
		t.Fatalf("CloneFloatMap() did not copy backing map")
	}
}

func TestCloneFloatMapNilBecomesEmptyMap(t *testing.T) {
	got := CloneFloatMap(nil)
	if got == nil {
		t.Fatal("CloneFloatMap(nil) = nil, want empty map")
	}
	if len(got) != 0 {
		t.Fatalf("CloneFloatMap(nil) len = %d, want 0", len(got))
	}
}

func TestSortedFloatKeys(t *testing.T) {
	got := SortedFloatKeys(map[string]float64{
		"gamma": 0.3,
		"alpha": 0.1,
		"beta":  0.2,
	})
	want := []string{"alpha", "beta", "gamma"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SortedFloatKeys() = %#v, want %#v", got, want)
	}
}

func TestSortedFloatKeysEmpty(t *testing.T) {
	got := SortedFloatKeys(nil)
	if got == nil {
		t.Fatal("SortedFloatKeys(nil) = nil, want empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("SortedFloatKeys(nil) len = %d, want 0", len(got))
	}
}
