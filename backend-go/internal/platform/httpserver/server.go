package httpserver

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mathstudy/backend-go/internal/platform/config"
	"mathstudy/backend-go/internal/platform/health"
	"mathstudy/backend-go/internal/platform/metrics"
	"mathstudy/backend-go/internal/platform/middleware"
)

// NewHandler builds the complete HTTP handler tree.
func NewHandler(cfg config.Config, logger *slog.Logger, checker health.Checker, store *metrics.Store) (http.Handler, error) {
	uploadsDir, err := filepath.Abs(cfg.UploadsDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, checker.Simple())
	})
	mux.HandleFunc("GET /health/detailed", func(w http.ResponseWriter, r *http.Request) {
		status := checker.Detailed(r.Context())
		httpStatus := http.StatusOK
		if status.Status != "healthy" {
			httpStatus = http.StatusServiceUnavailable
		}
		writeJSON(w, httpStatus, status)
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		if !cfg.MetricsEnabled {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "metrics disabled")
			return
		}
		w.Header().Set("Content-Type", metrics.ContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(store.Render()))
	})
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadsDir))))
	mux.HandleFunc(cfg.APIV1Prefix+"/", notMigratedHandler)
	mux.HandleFunc("/", notFoundHandler)

	return middleware.Chain(
		mux,
		middleware.RequestID,
		middleware.SecurityHeaders,
		middleware.Timeout(cfg.RequestTimeout),
		middleware.CORS(cfg.CORSOrigins, cfg.CORSAllowMethods, cfg.CORSAllowHeaders),
		middleware.Gzip,
		middleware.RequestMetrics(store),
		middleware.RequestLogger(logger),
	), nil
}

func notMigratedHandler(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Go backend route not migrated yet")
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "API route not found")
		return
	}
	writeError(w, http.StatusNotFound, "NOT_FOUND", "route not found")
}
