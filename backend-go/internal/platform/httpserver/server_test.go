package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"mathstudy/backend-go/internal/platform/config"
	"mathstudy/backend-go/internal/platform/health"
	"mathstudy/backend-go/internal/platform/metrics"
)

type okPinger struct{}

func (okPinger) Ping(context.Context) error {
	return nil
}

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	cfg := config.Config{
		AppVersion:       "test",
		Environment:      "test",
		APIV1Prefix:      "/api/v1",
		CORSOrigins:      []string{"http://localhost:5173"},
		CORSAllowMethods: []string{"GET", "POST", "OPTIONS"},
		CORSAllowHeaders: []string{"Authorization", "Content-Type"},
		RequestTimeout:   0,
		MetricsEnabled:   true,
		UploadsDir:       t.TempDir(),
	}
	handler, err := NewHandler(
		cfg,
		slog.New(slog.NewTextHandler(os.Stdout, nil)),
		health.NewChecker("test", okPinger{}, okPinger{}),
		metrics.NewStore("test", "test"),
	)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return handler
}

func TestHealthRoute(t *testing.T) {
	handler := testHandler(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "healthy" || body["version"] != "test" {
		t.Fatalf("body = %#v", body)
	}
}

func TestAPIV1FallbackReturnsNotImplemented(t *testing.T) {
	handler := testHandler(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["code"] != "NOT_IMPLEMENTED" {
		t.Fatalf("body = %#v", body)
	}
}

func TestCORSPreflight(t *testing.T) {
	handler := testHandler(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/session", nil)
	request.Header.Set("Origin", "http://localhost:5173")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestMetricsRoute(t *testing.T) {
	handler := testHandler(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Header().Get("Content-Type") != metrics.ContentType {
		t.Fatalf("Content-Type = %q", recorder.Header().Get("Content-Type"))
	}
}
