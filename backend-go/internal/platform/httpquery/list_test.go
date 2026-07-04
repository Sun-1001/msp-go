package httpquery

import (
	"net/url"
	"reflect"
	"testing"
)

func TestStringList(t *testing.T) {
	values := []string{" login_failed, service_error ", "", "warning,,error"}
	got := StringList(values)
	want := []string{"login_failed", "service_error", "warning", "error"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("StringList() = %#v, want %#v", got, want)
	}

	if got := StringList(nil); len(got) != 0 {
		t.Fatalf("StringList(nil) = %#v, want empty slice", got)
	}
}

func TestNamedStringList(t *testing.T) {
	query := url.Values{
		"tags":   []string{"calculus, exam"},
		"tags[]": []string{"ai", "  "},
		"other":  []string{"ignored"},
	}

	got := NamedStringList(query, "tags")
	want := []string{"calculus", "exam", "ai"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NamedStringList() = %#v, want %#v", got, want)
	}
}
