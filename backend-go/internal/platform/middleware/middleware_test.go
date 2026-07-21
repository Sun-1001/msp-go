package middleware

import (
	"compress/gzip"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mathstudy/backend-go/internal/platform/metrics"
)

func TestCORSAllowsCredentialsOnlyForExplicitOrigins(t *testing.T) {
	handler := CORS([]string{"https://app.example.com"}, []string{"GET"}, []string{"Authorization"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Origin", "https://app.example.com")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Access-Control-Allow-Credentials = %q", got)
	}
}

func TestCORSWildcardDoesNotAllowCredentials(t *testing.T) {
	handler := CORS([]string{"*"}, []string{"GET"}, []string{"Authorization"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Origin", "https://app.example.com")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want empty", got)
	}
}

func TestRequestIDPreservesSafeClientHeader(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Context().Value(responseKey{}); got != "trace-123_A.B:/span" {
			t.Fatalf("request id context = %#v", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Request-ID", " trace-123_A.B:/span ")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("X-Request-ID"); got != "trace-123_A.B:/span" {
		t.Fatalf("X-Request-ID = %q", got)
	}
}

func TestRequestIDReplacesUnsafeClientHeader(t *testing.T) {
	original := readRequestIDRandom
	readRequestIDRandom = func(data []byte) (int, error) {
		for i := range data {
			data[i] = byte(i)
		}
		return len(data), nil
	}
	defer func() {
		readRequestIDRandom = original
	}()
	for _, tc := range []struct {
		name   string
		header string
	}{
		{name: "space", header: "bad value"},
		{name: "control", header: "bad\rvalue"},
		{name: "too-long", header: strings.Repeat("a", maxRequestIDLength+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Context().Value(responseKey{}); got != "000102030405060708090a0b0c0d0e0f" {
					t.Fatalf("request id context = %#v", got)
				}
			}))
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("X-Request-ID", tc.header)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			if got := recorder.Header().Get("X-Request-ID"); got != "000102030405060708090a0b0c0d0e0f" {
				t.Fatalf("X-Request-ID = %q", got)
			}
		})
	}
}

func TestNewRequestIDFallbackIsUnique(t *testing.T) {
	original := readRequestIDRandom
	readRequestIDRandom = func([]byte) (int, error) {
		return 0, errors.New("random unavailable")
	}
	requestIDFallbackSerial.Store(0)
	defer func() {
		readRequestIDRandom = original
		requestIDFallbackSerial.Store(0)
	}()

	first := newRequestID()
	second := newRequestID()

	if first == second {
		t.Fatalf("fallback request IDs should be unique, got %q", first)
	}
	if len(first) != 32 || len(second) != 32 {
		t.Fatalf("fallback request IDs = %q, %q; want 32 hex characters", first, second)
	}
}

func TestGzipHonorsEncodingQualityAndBoundaries(t *testing.T) {
	tests := []struct {
		header string
		want   bool
	}{
		{header: "gzip", want: true},
		{header: "br, GZip; q=0.5", want: true},
		{header: "gzip;q=0, *;q=1", want: false},
		{header: "*;q=0.5", want: true},
		{header: "xgzip", want: false},
		{header: "gzip;q=invalid", want: false},
	}
	for _, test := range tests {
		if got := acceptsGzip(test.header); got != test.want {
			t.Errorf("acceptsGzip(%q) = %t, want %t", test.header, got, test.want)
		}
	}
}

func TestGzipRemovesContentLengthAndPreservesStreamingFlush(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := Gzip(RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Content-Length", "999")
		_, _ = w.Write([]byte("data: ready\n\n"))
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("wrapped response writer does not implement http.Flusher")
			return
		}
		flusher.Flush()
	})))
	request := httptest.NewRequest(http.MethodGet, "/stream", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if !recorder.Flushed {
		t.Fatal("stream response was not flushed")
	}
	if got := recorder.Header().Get("Content-Length"); got != "" {
		t.Fatalf("Content-Length = %q, want empty", got)
	}
	if got := recorder.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q", got)
	}
	reader, err := gzip.NewReader(recorder.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader() error = %v", err)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll(gzip) error = %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Close(gzip) error = %v", err)
	}
	if string(body) != "data: ready\n\n" {
		t.Fatalf("decoded body = %q", body)
	}
}

func TestStatusRecorderKeepsFirstStatusCode(t *testing.T) {
	recorder := httptest.NewRecorder()
	wrapped := &statusRecorder{ResponseWriter: recorder, status: http.StatusOK}
	wrapped.WriteHeader(http.StatusCreated)
	wrapped.WriteHeader(http.StatusInternalServerError)
	if wrapped.status != http.StatusCreated || recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, recorder = %d", wrapped.status, recorder.Code)
	}
}

func TestRequestMetricsUsesServeMuxPatternAndStatusClass(t *testing.T) {
	store := metrics.NewStore("test", "test")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/items/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	handler := RequestMetrics(store)(mux)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/items/item-42", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	rendered := store.Render()
	want := `msp_http_server_requests_total{method="GET",route="/api/v1/items/{id}",status_class="2xx"} 1`
	if !strings.Contains(rendered, want) {
		t.Fatalf("metrics missing %q in:\n%s", want, rendered)
	}
	if strings.Contains(rendered, "item-42") {
		t.Fatalf("metrics leaked raw path in:\n%s", rendered)
	}
}

func TestRequestMetricsUsesFixedFallbackLabels(t *testing.T) {
	store := metrics.NewStore("test", "test")
	handler := RequestMetrics(store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	request := httptest.NewRequest(http.MethodGet, "/users/private-user-id", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	rendered := store.Render()
	want := `msp_http_server_requests_total{method="GET",route="<unmatched>",status_class="4xx"} 1`
	if !strings.Contains(rendered, want) {
		t.Fatalf("metrics missing %q in:\n%s", want, rendered)
	}
	if strings.Contains(rendered, "private-user-id") {
		t.Fatalf("metrics leaked raw path in:\n%s", rendered)
	}
}

func TestMetricRouteClassifiesCORSPreflight(t *testing.T) {
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/session", nil)
	if got := metricRoute(request); got != "<cors-preflight>" {
		t.Fatalf("metricRoute() = %q", got)
	}
}

func BenchmarkGzipMiddleware(b *testing.B) {
	payload := []byte(strings.Repeat("benchmark-response-", 128))
	handler := Gzip(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	request := httptest.NewRequest(http.MethodGet, "/benchmark", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	writer := &discardResponseWriter{header: make(http.Header)}
	b.ReportAllocs()
	for range b.N {
		clear(writer.header)
		handler.ServeHTTP(writer, request)
	}
}

type discardResponseWriter struct {
	header http.Header
}

func (w *discardResponseWriter) Header() http.Header            { return w.header }
func (w *discardResponseWriter) Write(data []byte) (int, error) { return len(data), nil }
func (*discardResponseWriter) WriteHeader(int)                  {}
