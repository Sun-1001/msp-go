package httpserver

import (
	"log/slog"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mathstudy/backend-go/internal/platform/config"
	"mathstudy/backend-go/internal/platform/health"
	"mathstudy/backend-go/internal/platform/metrics"
	"mathstudy/backend-go/internal/platform/middleware"
	"mathstudy/backend-go/internal/platform/uploadpath"
)

// RouteRegistrar attaches business routes to the shared mux.
type RouteRegistrar func(*http.ServeMux)

type handlerOptions struct {
	registrars []RouteRegistrar
}

// Option customizes the HTTP handler tree.
type Option func(*handlerOptions)

// WithRoutes registers business routes on the shared mux.
func WithRoutes(registrar RouteRegistrar) Option {
	return func(options *handlerOptions) {
		if registrar != nil {
			options.registrars = append(options.registrars, registrar)
		}
	}
}

// NewHandler builds the complete HTTP handler tree.
func NewHandler(cfg config.Config, logger *slog.Logger, checker health.Checker, store *metrics.Store, opts ...Option) (http.Handler, error) {
	options := handlerOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	uploadsDir, err := filepath.Abs(cfg.UploadsDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		return nil, err
	}
	managementAccess, err := newManagementAccess(cfg.ManagementAllowedCIDRs)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, checker.Simple())
	})
	mux.HandleFunc("GET /health/detailed", func(w http.ResponseWriter, r *http.Request) {
		if !managementAccess.Allow(r) {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "management endpoint is restricted")
			return
		}
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
		if !managementAccess.Allow(r) {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "management endpoint is restricted")
			return
		}
		w.Header().Set("Content-Type", metrics.ContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(store.Render()))
	})
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", uploadsFileHandler(uploadsDir)))
	for _, registrar := range options.registrars {
		registrar(mux)
	}
	router := muxWithFallbacks(mux)

	return middleware.Chain(
		router,
		middleware.RequestID,
		middleware.SecurityHeaders,
		middleware.Timeout(cfg.RequestTimeout),
		middleware.CORS(cfg.CORSOrigins, cfg.CORSAllowMethods, cfg.CORSAllowHeaders),
		middleware.Gzip,
		middleware.RequestMetrics(store),
		middleware.RequestLogger(logger),
	), nil
}

func muxWithFallbacks(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		candidate, pattern := mux.Handler(r)
		if pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}

		// ServeMux uses an empty pattern for both its internal 404 and 405 handlers.
		probe := newStatusProbe()
		candidate.ServeHTTP(probe, r)
		switch probe.status {
		case http.StatusNotFound:
			notFoundHandler(w, r)
		case http.StatusMethodNotAllowed:
			w.Header().Set("Allow", probe.Header().Get("Allow"))
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		default:
			mux.ServeHTTP(w, r)
		}
	})
}

type statusProbe struct {
	header http.Header
	status int
}

func newStatusProbe() *statusProbe {
	return &statusProbe{header: make(http.Header)}
}

func (p *statusProbe) Header() http.Header {
	return p.header
}

func (p *statusProbe) WriteHeader(status int) {
	if p.status == 0 {
		p.status = status
	}
}

func (p *statusProbe) Write(body []byte) (int, error) {
	if p.status == 0 {
		p.status = http.StatusOK
	}
	return len(body), nil
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "API route not found")
		return
	}
	writeError(w, http.StatusNotFound, "NOT_FOUND", "route not found")
}

func uploadsFileHandler(root string) http.Handler {
	fs := http.Dir(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		cleanPath, ok := uploadpath.CleanServablePath(r.URL.Path)
		if !ok {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "uploaded file not found")
			return
		}
		file, err := fs.Open(cleanPath)
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "uploaded file not found")
			return
		}
		defer file.Close()
		stat, err := file.Stat()
		if err != nil || stat.IsDir() {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "uploaded file not found")
			return
		}
		if uploadpath.IsDocumentKey(cleanPath) {
			w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": stat.Name()}))
		}
		http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
	})
}

type managementAccess struct {
	networks []*net.IPNet
}

func newManagementAccess(cidrs []string) (managementAccess, error) {
	access := managementAccess{networks: []*net.IPNet{}}
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return managementAccess{}, err
		}
		access.networks = append(access.networks, network)
	}
	return access, nil
}

func (a managementAccess) Allow(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return false
	}
	for _, network := range a.networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
