package metrics

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const ContentType = "text/plain; version=0.0.4; charset=utf-8"

var httpDurationBuckets = [...]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// RuntimeStatsProvider returns point-in-time dependency pool statistics.
type RuntimeStatsProvider func() RuntimeStats

// RuntimeStats contains dependency statistics rendered with process metrics.
type RuntimeStats struct {
	Postgres PostgresPoolStats
	Redis    RedisPoolStats
}

// PostgresPoolStats is a dependency-neutral snapshot of pgx pool statistics.
type PostgresPoolStats struct {
	MaxConnections          int64
	TotalConnections        int64
	AcquiredConnections     int64
	IdleConnections         int64
	ConstructingConnections int64
	AcquireCount            int64
	EmptyAcquireCount       int64
	CanceledAcquireCount    int64
	AcquireDuration         time.Duration
	EmptyAcquireWaitTime    time.Duration
}

// RedisPoolStats is a dependency-neutral snapshot of go-redis pool statistics.
type RedisPoolStats struct {
	TotalConnections int64
	IdleConnections  int64
	StaleConnections int64
	Hits             uint64
	Misses           uint64
	Timeouts         uint64
	WaitCount        uint64
	Unusable         uint64
	WaitDuration     time.Duration
}

type httpLabels struct {
	method      string
	route       string
	statusClass string
}

type httpSeries struct {
	count       uint64
	durationSum float64
	buckets     []uint64
}

type httpSnapshot struct {
	labels      httpLabels
	count       uint64
	durationSum float64
	buckets     []uint64
}

// Store keeps Prometheus-compatible process and request metrics.
type Store struct {
	version     string
	environment string
	requests    atomic.Uint64

	mu                   sync.RWMutex
	http                 map[httpLabels]*httpSeries
	runtimeStatsProvider RuntimeStatsProvider
}

// NewStore creates a metrics store for the API process.
func NewStore(version, environment string) *Store {
	return &Store{
		version:     version,
		environment: environment,
		http:        make(map[httpLabels]*httpSeries),
	}
}

// IncRequests increments the legacy process-level HTTP request counter.
func (s *Store) IncRequests() {
	s.requests.Add(1)
}

// ObserveHTTPRequest records one completed HTTP request using bounded labels.
func (s *Store) ObserveHTTPRequest(method, route string, status int, duration time.Duration) {
	s.requests.Add(1)
	labels := httpLabels{
		method:      normalizeMethod(method),
		route:       normalizeRoute(route),
		statusClass: statusCodeClass(status),
	}
	durationSeconds := nonNegativeSeconds(duration)

	s.mu.Lock()
	series := s.http[labels]
	if series == nil {
		series = &httpSeries{buckets: make([]uint64, len(httpDurationBuckets))}
		s.http[labels] = series
	}
	series.count++
	series.durationSum += durationSeconds
	for i, upperBound := range httpDurationBuckets {
		if durationSeconds <= upperBound {
			series.buckets[i]++
		}
	}
	s.mu.Unlock()
}

// SetRuntimeStatsProvider configures dependency pool metrics sampled at scrape time.
func (s *Store) SetRuntimeStatsProvider(provider RuntimeStatsProvider) {
	s.mu.Lock()
	s.runtimeStatsProvider = provider
	s.mu.Unlock()
}

// Render returns Prometheus text exposition format.
func (s *Store) Render() string {
	httpStats, runtimeProvider := s.snapshot()

	var b strings.Builder
	b.WriteString("# HELP msp_app_info MathStudyPlatform Go API build information.\n")
	b.WriteString("# TYPE msp_app_info gauge\n")
	fmt.Fprintf(&b, "msp_app_info{version=%q,environment=%q} 1\n", s.version, s.environment)
	b.WriteString("# HELP msp_http_requests_total Total HTTP requests handled by the Go API.\n")
	b.WriteString("# TYPE msp_http_requests_total counter\n")
	fmt.Fprintf(&b, "msp_http_requests_total %d\n", s.requests.Load())
	renderHTTPMetrics(&b, httpStats)
	if runtimeProvider != nil {
		renderRuntimeMetrics(&b, runtimeProvider())
	}
	b.WriteString("# HELP msp_health_status Static process health marker for the Go API.\n")
	b.WriteString("# TYPE msp_health_status gauge\n")
	b.WriteString("msp_health_status{component=\"app\"} 1\n")
	return b.String()
}

func (s *Store) snapshot() ([]httpSnapshot, RuntimeStatsProvider) {
	s.mu.RLock()
	snapshots := make([]httpSnapshot, 0, len(s.http))
	for labels, series := range s.http {
		snapshots = append(snapshots, httpSnapshot{
			labels:      labels,
			count:       series.count,
			durationSum: series.durationSum,
			buckets:     append([]uint64(nil), series.buckets...),
		})
	}
	runtimeProvider := s.runtimeStatsProvider
	s.mu.RUnlock()

	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].labels.method != snapshots[j].labels.method {
			return snapshots[i].labels.method < snapshots[j].labels.method
		}
		if snapshots[i].labels.route != snapshots[j].labels.route {
			return snapshots[i].labels.route < snapshots[j].labels.route
		}
		return snapshots[i].labels.statusClass < snapshots[j].labels.statusClass
	})
	return snapshots, runtimeProvider
}

func renderHTTPMetrics(b *strings.Builder, snapshots []httpSnapshot) {
	b.WriteString("# HELP msp_http_server_requests_total HTTP requests grouped by method, route template, and status class.\n")
	b.WriteString("# TYPE msp_http_server_requests_total counter\n")
	for _, snapshot := range snapshots {
		fmt.Fprintf(
			b,
			"msp_http_server_requests_total{method=%q,route=%q,status_class=%q} %d\n",
			snapshot.labels.method,
			snapshot.labels.route,
			snapshot.labels.statusClass,
			snapshot.count,
		)
	}

	b.WriteString("# HELP msp_http_server_request_duration_seconds HTTP request duration grouped by method, route template, and status class.\n")
	b.WriteString("# TYPE msp_http_server_request_duration_seconds histogram\n")
	for _, snapshot := range snapshots {
		for i, upperBound := range httpDurationBuckets {
			fmt.Fprintf(
				b,
				"msp_http_server_request_duration_seconds_bucket{method=%q,route=%q,status_class=%q,le=%q} %d\n",
				snapshot.labels.method,
				snapshot.labels.route,
				snapshot.labels.statusClass,
				strconv.FormatFloat(upperBound, 'g', -1, 64),
				snapshot.buckets[i],
			)
		}
		fmt.Fprintf(
			b,
			"msp_http_server_request_duration_seconds_bucket{method=%q,route=%q,status_class=%q,le=\"+Inf\"} %d\n",
			snapshot.labels.method,
			snapshot.labels.route,
			snapshot.labels.statusClass,
			snapshot.count,
		)
		fmt.Fprintf(
			b,
			"msp_http_server_request_duration_seconds_sum{method=%q,route=%q,status_class=%q} %s\n",
			snapshot.labels.method,
			snapshot.labels.route,
			snapshot.labels.statusClass,
			formatFloat(snapshot.durationSum),
		)
		fmt.Fprintf(
			b,
			"msp_http_server_request_duration_seconds_count{method=%q,route=%q,status_class=%q} %d\n",
			snapshot.labels.method,
			snapshot.labels.route,
			snapshot.labels.statusClass,
			snapshot.count,
		)
	}
}

func renderRuntimeMetrics(b *strings.Builder, stats RuntimeStats) {
	b.WriteString("# HELP msp_postgres_pool_max_connections Configured PostgreSQL pool connection limit.\n")
	b.WriteString("# TYPE msp_postgres_pool_max_connections gauge\n")
	fmt.Fprintf(b, "msp_postgres_pool_max_connections %d\n", stats.Postgres.MaxConnections)
	b.WriteString("# HELP msp_postgres_pool_connections Current PostgreSQL pool connections by state.\n")
	b.WriteString("# TYPE msp_postgres_pool_connections gauge\n")
	fmt.Fprintf(b, "msp_postgres_pool_connections{state=\"total\"} %d\n", stats.Postgres.TotalConnections)
	fmt.Fprintf(b, "msp_postgres_pool_connections{state=\"acquired\"} %d\n", stats.Postgres.AcquiredConnections)
	fmt.Fprintf(b, "msp_postgres_pool_connections{state=\"idle\"} %d\n", stats.Postgres.IdleConnections)
	fmt.Fprintf(b, "msp_postgres_pool_connections{state=\"constructing\"} %d\n", stats.Postgres.ConstructingConnections)
	b.WriteString("# HELP msp_postgres_pool_acquires_total Total successful PostgreSQL pool acquires.\n")
	b.WriteString("# TYPE msp_postgres_pool_acquires_total counter\n")
	fmt.Fprintf(b, "msp_postgres_pool_acquires_total %d\n", stats.Postgres.AcquireCount)
	b.WriteString("# HELP msp_postgres_pool_empty_acquires_total PostgreSQL acquires that waited for a connection.\n")
	b.WriteString("# TYPE msp_postgres_pool_empty_acquires_total counter\n")
	fmt.Fprintf(b, "msp_postgres_pool_empty_acquires_total %d\n", stats.Postgres.EmptyAcquireCount)
	b.WriteString("# HELP msp_postgres_pool_canceled_acquires_total PostgreSQL pool acquires canceled by context.\n")
	b.WriteString("# TYPE msp_postgres_pool_canceled_acquires_total counter\n")
	fmt.Fprintf(b, "msp_postgres_pool_canceled_acquires_total %d\n", stats.Postgres.CanceledAcquireCount)
	b.WriteString("# HELP msp_postgres_pool_acquire_duration_seconds_total Total time spent acquiring PostgreSQL connections.\n")
	b.WriteString("# TYPE msp_postgres_pool_acquire_duration_seconds_total counter\n")
	fmt.Fprintf(b, "msp_postgres_pool_acquire_duration_seconds_total %s\n", formatFloat(nonNegativeSeconds(stats.Postgres.AcquireDuration)))
	b.WriteString("# HELP msp_postgres_pool_empty_acquire_wait_seconds_total Total time waiting when the PostgreSQL pool was empty.\n")
	b.WriteString("# TYPE msp_postgres_pool_empty_acquire_wait_seconds_total counter\n")
	fmt.Fprintf(b, "msp_postgres_pool_empty_acquire_wait_seconds_total %s\n", formatFloat(nonNegativeSeconds(stats.Postgres.EmptyAcquireWaitTime)))

	b.WriteString("# HELP msp_redis_pool_connections Current Redis pool connections by state.\n")
	b.WriteString("# TYPE msp_redis_pool_connections gauge\n")
	fmt.Fprintf(b, "msp_redis_pool_connections{state=\"total\"} %d\n", stats.Redis.TotalConnections)
	fmt.Fprintf(b, "msp_redis_pool_connections{state=\"idle\"} %d\n", stats.Redis.IdleConnections)
	fmt.Fprintf(b, "msp_redis_pool_connections{state=\"stale\"} %d\n", stats.Redis.StaleConnections)
	renderRedisCounter(b, "hits", "Redis pool hits.", stats.Redis.Hits)
	renderRedisCounter(b, "misses", "Redis pool misses.", stats.Redis.Misses)
	renderRedisCounter(b, "timeouts", "Redis pool wait timeouts.", stats.Redis.Timeouts)
	renderRedisCounter(b, "waits", "Redis pool connection waits.", stats.Redis.WaitCount)
	renderRedisCounter(b, "unusable_connections", "Redis pool unusable connections.", stats.Redis.Unusable)
	b.WriteString("# HELP msp_redis_pool_wait_duration_seconds_total Total time waiting for Redis pool connections.\n")
	b.WriteString("# TYPE msp_redis_pool_wait_duration_seconds_total counter\n")
	fmt.Fprintf(b, "msp_redis_pool_wait_duration_seconds_total %s\n", formatFloat(nonNegativeSeconds(stats.Redis.WaitDuration)))
}

func renderRedisCounter(b *strings.Builder, name, help string, value uint64) {
	metricName := "msp_redis_pool_" + name + "_total"
	fmt.Fprintf(b, "# HELP %s %s\n", metricName, help)
	fmt.Fprintf(b, "# TYPE %s counter\n", metricName)
	fmt.Fprintf(b, "%s %d\n", metricName, value)
}

func normalizeMethod(method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return "UNKNOWN"
	}
	return method
}

func normalizeRoute(route string) string {
	route = strings.TrimSpace(route)
	if route == "" {
		return "<unmatched>"
	}
	return route
}

func statusCodeClass(status int) string {
	switch {
	case status >= 100 && status <= 199:
		return "1xx"
	case status >= 200 && status <= 299:
		return "2xx"
	case status >= 300 && status <= 399:
		return "3xx"
	case status >= 400 && status <= 499:
		return "4xx"
	case status >= 500 && status <= 599:
		return "5xx"
	default:
		return "unknown"
	}
}

func nonNegativeSeconds(duration time.Duration) float64 {
	if duration < 0 {
		return 0
	}
	return duration.Seconds()
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}
