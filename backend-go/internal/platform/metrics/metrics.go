package metrics

import (
	"fmt"
	"strings"
	"sync/atomic"
)

const ContentType = "text/plain; version=0.0.4; charset=utf-8"

// Store keeps the P1 Prometheus-compatible process metrics.
type Store struct {
	version     string
	environment string
	requests    atomic.Uint64
}

// NewStore creates a metrics store for the API process.
func NewStore(version, environment string) *Store {
	return &Store{version: version, environment: environment}
}

// IncRequests increments the HTTP request counter.
func (s *Store) IncRequests() {
	s.requests.Add(1)
}

// Render returns Prometheus text exposition format.
func (s *Store) Render() string {
	var b strings.Builder
	b.WriteString("# HELP msp_app_info MathStudyPlatform Go API build information.\n")
	b.WriteString("# TYPE msp_app_info gauge\n")
	fmt.Fprintf(&b, "msp_app_info{version=%q,environment=%q} 1\n", s.version, s.environment)
	b.WriteString("# HELP msp_http_requests_total Total HTTP requests handled by the Go API.\n")
	b.WriteString("# TYPE msp_http_requests_total counter\n")
	fmt.Fprintf(&b, "msp_http_requests_total %d\n", s.requests.Load())
	b.WriteString("# HELP msp_health_status Static process health marker for the Go API.\n")
	b.WriteString("# TYPE msp_health_status gauge\n")
	b.WriteString("msp_health_status{component=\"app\"} 1\n")
	return b.String()
}
