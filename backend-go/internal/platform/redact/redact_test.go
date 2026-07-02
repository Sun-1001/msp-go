package redact

import (
	"strings"
	"testing"
)

func TestValueRedactsSensitiveNestedData(t *testing.T) {
	value := Value("metadata", []byte(`{
		"request_id":"req-1",
		"authorization":"Bearer token",
		"nested":{"api_key":"secret","safe":"ok"},
		"items":[{"refresh_token":"rt","count":1}]
	}`))

	metadata, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("Value() = %#v", value)
	}
	if metadata["request_id"] != "req-1" {
		t.Fatalf("request_id = %#v", metadata["request_id"])
	}
	if metadata["authorization"] != Marker {
		t.Fatalf("authorization = %#v, want redacted", metadata["authorization"])
	}
	nested := metadata["nested"].(map[string]any)
	if nested["api_key"] != Marker || nested["safe"] != "ok" {
		t.Fatalf("nested = %#v", nested)
	}
	items := metadata["items"].([]any)
	first := items[0].(map[string]any)
	if first["refresh_token"] != Marker || first["count"] != float64(1) {
		t.Fatalf("items = %#v", items)
	}
}

func TestStringRedactsCredentialFragments(t *testing.T) {
	value := String("Authorization: Bearer secret-token url=/callback?token=abc&cookie=sid-123 api_key=plain session_id=abc123 eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature")
	for _, leaked := range []string{"secret-token", "token=abc", "cookie=sid-123", "api_key=plain", "session_id=abc123", "eyJhbGci"} {
		if strings.Contains(value, leaked) {
			t.Fatalf("String leaked %q in %q", leaked, value)
		}
	}
	if !strings.Contains(value, Marker) {
		t.Fatalf("String() = %q, want redaction marker", value)
	}
}
