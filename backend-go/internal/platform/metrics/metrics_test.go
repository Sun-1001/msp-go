package metrics

import (
	"strings"
	"testing"
)

func TestStoreRenderIncludesRequestCount(t *testing.T) {
	store := NewStore("0.1.0", "test")
	store.IncRequests()
	store.IncRequests()

	rendered := store.Render()

	for _, want := range []string{
		`msp_app_info{version="0.1.0",environment="test"} 1`,
		"msp_http_requests_total 2",
		`msp_health_status{component="app"} 1`,
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("Render() missing %q in:\n%s", want, rendered)
		}
	}
}
