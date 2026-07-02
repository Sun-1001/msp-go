package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
		AppVersion:             "test",
		Environment:            "test",
		APIV1Prefix:            "/api/v1",
		CORSOrigins:            []string{"http://localhost:5173"},
		CORSAllowMethods:       []string{"GET", "POST", "OPTIONS"},
		CORSAllowHeaders:       []string{"Authorization", "Content-Type"},
		RequestTimeout:         0,
		MetricsEnabled:         true,
		ManagementAllowedCIDRs: []string{"127.0.0.1/32"},
		UploadsDir:             t.TempDir(),
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

func testHandlerWithRoutes(t *testing.T, registrar RouteRegistrar) http.Handler {
	t.Helper()
	cfg := config.Config{
		AppVersion:             "test",
		Environment:            "test",
		APIV1Prefix:            "/api/v1",
		CORSOrigins:            []string{"http://localhost:5173"},
		CORSAllowMethods:       []string{"GET", "POST", "OPTIONS"},
		CORSAllowHeaders:       []string{"Authorization", "Content-Type"},
		RequestTimeout:         0,
		MetricsEnabled:         true,
		ManagementAllowedCIDRs: []string{"127.0.0.1/32"},
		UploadsDir:             t.TempDir(),
	}
	handler, err := NewHandler(
		cfg,
		slog.New(slog.NewTextHandler(os.Stdout, nil)),
		health.NewChecker("test", okPinger{}, okPinger{}),
		metrics.NewStore("test", "test"),
		WithRoutes(registrar),
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
	if recorder.Header().Get("Content-Security-Policy") == "" || recorder.Header().Get("Permissions-Policy") == "" {
		t.Fatalf("security headers missing: %#v", recorder.Header())
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

func TestMigratedRouteRunsBeforeAPIV1Fallback(t *testing.T) {
	handler := testHandlerWithRoutes(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api/v1/auth/registration-status", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		})
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/registration-status", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
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
	request.RemoteAddr = "127.0.0.1:1234"

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Header().Get("Content-Type") != metrics.ContentType {
		t.Fatalf("Content-Type = %q", recorder.Header().Get("Content-Type"))
	}
}

func TestManagementRoutesRejectPublicRemote(t *testing.T) {
	handler := testHandler(t)
	for _, target := range []string{"/health/detailed", "/metrics"} {
		t.Run(target, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, target, nil)
			request.RemoteAddr = "203.0.113.10:4567"

			handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestUploadsHandlerServesFilesWithoutDirectoryListings(t *testing.T) {
	uploadsDir := t.TempDir()
	imagesDir := filepath.Join(uploadsDir, "images")
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(imagesDir, "file.txt"), []byte("image-data"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	documentsDir := filepath.Join(uploadsDir, "documents")
	if err := os.MkdirAll(documentsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(documents) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(documentsDir, "file.pdf"), []byte("%PDF-1.7\nbody"), 0o644); err != nil {
		t.Fatalf("WriteFile(document) error = %v", err)
	}
	videosDir := filepath.Join(uploadsDir, "videos")
	if err := os.MkdirAll(videosDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(videos) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(videosDir, "clip.mp4"), []byte("video-data"), 0o644); err != nil {
		t.Fatalf("WriteFile(video) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(uploadsDir, "secret.txt"), []byte("secret-data"), 0o644); err != nil {
		t.Fatalf("WriteFile(secret) error = %v", err)
	}
	cfg := config.Config{
		AppVersion:             "test",
		Environment:            "test",
		APIV1Prefix:            "/api/v1",
		CORSOrigins:            []string{"http://localhost:5173"},
		CORSAllowMethods:       []string{"GET", "POST", "OPTIONS"},
		CORSAllowHeaders:       []string{"Authorization", "Content-Type"},
		RequestTimeout:         0,
		MetricsEnabled:         true,
		ManagementAllowedCIDRs: []string{"127.0.0.1/32"},
		UploadsDir:             uploadsDir,
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

	for _, target := range []string{"/uploads/", "/uploads/images/"} {
		t.Run(target, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, target, nil)
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/uploads/images/file.txt", nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "image-data" {
		t.Fatalf("file response = status %d body %q", recorder.Code, recorder.Body.String())
	}
	if disposition := recorder.Header().Get("Content-Disposition"); disposition != "" {
		t.Fatalf("image Content-Disposition = %q, want empty", disposition)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/uploads/documents/file.pdf", nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "%PDF-1.7\nbody" {
		t.Fatalf("document response = status %d body %q", recorder.Code, recorder.Body.String())
	}
	disposition := recorder.Header().Get("Content-Disposition")
	if !strings.HasPrefix(disposition, "attachment;") || !strings.Contains(disposition, "filename=file.pdf") {
		t.Fatalf("document Content-Disposition = %q", disposition)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/uploads/videos/clip.mp4", nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "video-data" {
		t.Fatalf("video response = status %d body %q", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/uploads/secret.txt", nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound || strings.Contains(recorder.Body.String(), "secret-data") {
		t.Fatalf("secret response = status %d body %q", recorder.Code, recorder.Body.String())
	}
}
