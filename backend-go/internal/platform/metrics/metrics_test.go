package metrics

import (
	"strings"
	"sync"
	"testing"
	"time"
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

func TestStoreObserveHTTPRequestRendersCounterAndHistogram(t *testing.T) {
	store := NewStore("0.1.0", "test")
	store.ObserveHTTPRequest("get", "/api/v1/items/{id}", 201, 4*time.Millisecond)
	store.ObserveHTTPRequest("GET", "/api/v1/items/{id}", 503, 3*time.Second)

	rendered := store.Render()
	for _, want := range []string{
		"msp_http_requests_total 2",
		`msp_http_server_requests_total{method="GET",route="/api/v1/items/{id}",status_class="2xx"} 1`,
		`msp_http_server_requests_total{method="GET",route="/api/v1/items/{id}",status_class="5xx"} 1`,
		`msp_http_server_request_duration_seconds_bucket{method="GET",route="/api/v1/items/{id}",status_class="2xx",le="0.005"} 1`,
		`msp_http_server_request_duration_seconds_bucket{method="GET",route="/api/v1/items/{id}",status_class="5xx",le="2.5"} 0`,
		`msp_http_server_request_duration_seconds_bucket{method="GET",route="/api/v1/items/{id}",status_class="5xx",le="+Inf"} 1`,
		`msp_http_server_request_duration_seconds_sum{method="GET",route="/api/v1/items/{id}",status_class="2xx"} 0.004`,
		`msp_http_server_request_duration_seconds_count{method="GET",route="/api/v1/items/{id}",status_class="5xx"} 1`,
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("Render() missing %q in:\n%s", want, rendered)
		}
	}
}

func TestStoreObserveHTTPRequestNormalizesInvalidInputs(t *testing.T) {
	store := NewStore("0.1.0", "test")
	store.ObserveHTTPRequest("", "", 0, -time.Second)

	rendered := store.Render()
	for _, want := range []string{
		`msp_http_server_requests_total{method="UNKNOWN",route="<unmatched>",status_class="unknown"} 1`,
		`msp_http_server_request_duration_seconds_sum{method="UNKNOWN",route="<unmatched>",status_class="unknown"} 0`,
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("Render() missing %q in:\n%s", want, rendered)
		}
	}
}

func TestStatusCodeClassBoundaries(t *testing.T) {
	tests := []struct {
		status int
		want   string
	}{
		{status: 99, want: "unknown"},
		{status: 100, want: "1xx"},
		{status: 199, want: "1xx"},
		{status: 200, want: "2xx"},
		{status: 299, want: "2xx"},
		{status: 300, want: "3xx"},
		{status: 399, want: "3xx"},
		{status: 400, want: "4xx"},
		{status: 499, want: "4xx"},
		{status: 500, want: "5xx"},
		{status: 599, want: "5xx"},
		{status: 600, want: "unknown"},
	}
	for _, test := range tests {
		if got := statusCodeClass(test.status); got != test.want {
			t.Errorf("statusCodeClass(%d) = %q, want %q", test.status, got, test.want)
		}
	}
}

func TestStoreRenderIncludesRuntimePoolStats(t *testing.T) {
	store := NewStore("0.1.0", "test")
	store.SetRuntimeStatsProvider(func() RuntimeStats {
		return RuntimeStats{
			Postgres: PostgresPoolStats{
				MaxConnections:          20,
				TotalConnections:        8,
				AcquiredConnections:     3,
				IdleConnections:         4,
				ConstructingConnections: 1,
				AcquireCount:            50,
				EmptyAcquireCount:       2,
				CanceledAcquireCount:    1,
				AcquireDuration:         1250 * time.Millisecond,
				EmptyAcquireWaitTime:    250 * time.Millisecond,
			},
			Redis: RedisPoolStats{
				TotalConnections: 7,
				IdleConnections:  5,
				StaleConnections: 2,
				Hits:             100,
				Misses:           10,
				Timeouts:         3,
				WaitCount:        4,
				Unusable:         1,
				WaitDuration:     75 * time.Millisecond,
			},
		}
	})

	rendered := store.Render()
	for _, want := range []string{
		"msp_postgres_pool_max_connections 20",
		`msp_postgres_pool_connections{state="acquired"} 3`,
		"msp_postgres_pool_empty_acquires_total 2",
		"msp_postgres_pool_acquire_duration_seconds_total 1.25",
		`msp_redis_pool_connections{state="idle"} 5`,
		"msp_redis_pool_hits_total 100",
		"msp_redis_pool_timeouts_total 3",
		"msp_redis_pool_wait_duration_seconds_total 0.075",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("Render() missing %q in:\n%s", want, rendered)
		}
	}
}

func TestStoreObserveHTTPRequestIsConcurrencySafe(t *testing.T) {
	store := NewStore("0.1.0", "test")
	const goroutines = 8
	const requestsPerGoroutine = 100
	var waitGroup sync.WaitGroup
	for range goroutines {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for range requestsPerGoroutine {
				store.ObserveHTTPRequest("GET", "/health", 200, time.Millisecond)
			}
		}()
	}
	waitGroup.Wait()

	rendered := store.Render()
	for _, want := range []string{
		"msp_http_requests_total 800",
		`msp_http_server_requests_total{method="GET",route="/health",status_class="2xx"} 800`,
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("Render() missing %q in:\n%s", want, rendered)
		}
	}
}

func BenchmarkStoreObserveHTTPRequest(b *testing.B) {
	store := NewStore("0.1.0", "benchmark")
	store.ObserveHTTPRequest("GET", "/api/v1/exercise/{id}", 200, 25*time.Millisecond)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		store.ObserveHTTPRequest("GET", "/api/v1/exercise/{id}", 200, 25*time.Millisecond)
	}
}

func BenchmarkStoreObserveHTTPRequestParallel(b *testing.B) {
	store := NewStore("0.1.0", "benchmark")
	store.ObserveHTTPRequest("GET", "/api/v1/exercise/{id}", 200, 25*time.Millisecond)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(parallel *testing.PB) {
		for parallel.Next() {
			store.ObserveHTTPRequest("GET", "/api/v1/exercise/{id}", 200, 25*time.Millisecond)
		}
	})
}
