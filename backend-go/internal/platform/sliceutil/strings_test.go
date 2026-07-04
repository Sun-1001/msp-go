package sliceutil

import "testing"

func TestCloneStrings(t *testing.T) {
	source := []string{"a", "b"}
	got := CloneStrings(source)

	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("CloneStrings() = %#v", got)
	}
	got[0] = "changed"
	if source[0] != "a" {
		t.Fatalf("CloneStrings() did not copy backing array")
	}
}

func TestCloneStringsNilBecomesEmptySlice(t *testing.T) {
	got := CloneStrings(nil)
	if got == nil {
		t.Fatal("CloneStrings(nil) = nil, want empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("CloneStrings(nil) len = %d, want 0", len(got))
	}
}

func TestAppendUniqueNonEmptyStrings(t *testing.T) {
	got := AppendUniqueNonEmptyStrings([]string{" a ", "", "b", "a"}, " c ", "b", " ")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("AppendUniqueNonEmptyStrings() = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("AppendUniqueNonEmptyStrings() = %#v, want %#v", got, want)
		}
	}
}

func TestAppendUniqueNonEmptyStringsNilBecomesEmptySlice(t *testing.T) {
	got := AppendUniqueNonEmptyStrings(nil)
	if got == nil {
		t.Fatal("AppendUniqueNonEmptyStrings(nil) = nil, want empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("AppendUniqueNonEmptyStrings(nil) len = %d, want 0", len(got))
	}
}
