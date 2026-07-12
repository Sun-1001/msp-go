package uploadhttp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	authapp "mathstudy/backend-go/internal/application/auth"
	uploadapp "mathstudy/backend-go/internal/application/upload"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/redact"
)

const (
	multipartMemory       = 32 << 20
	uploadRateLimitWindow = time.Minute
	uploadRateLimitMax    = 60
)

// Service is the upload application surface used by HTTP handlers.
type Service interface {
	SaveImage(context.Context, io.Reader, uploadapp.FileMeta) (uploadapp.Response, error)
	SaveResourceFile(context.Context, io.Reader, uploadapp.FileMeta) (uploadapp.Response, error)
}

// Authenticator decodes Go/Python-compatible access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves /upload endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
	limiter *uploadRateLimiter
}

// NewHandler creates an upload HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("upload service is nil")
	}
	if auth == nil {
		return nil, errors.New("upload authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger, limiter: newUploadRateLimiter(uploadRateLimitMax, uploadRateLimitWindow)}, nil
}

// Register attaches upload routes under prefix, for example /api/v1/upload.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("POST "+prefix+"/image", h.image)
	mux.HandleFunc("POST "+prefix+"/resource", h.resource)
}

func (h *Handler) image(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	if !h.allowUpload(w, r, principal) {
		return
	}
	h.upload(w, r, uploadapp.MaxImageSize, h.service.SaveImage, "上传图片失败")
}

func (h *Handler) resource(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
		return
	}
	if !h.allowUpload(w, r, principal) {
		return
	}
	h.upload(w, r, uploadapp.MaxResourceSize, h.service.SaveResourceFile, "上传资源文件失败")
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request, maxSize int64, save func(context.Context, io.Reader, uploadapp.FileMeta) (uploadapp.Response, error), fallback string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxSize+multipartMemory)
	// #nosec G120 -- MaxBytesReader bounds the complete multipart request body.
	if err := r.ParseMultipartForm(multipartMemory); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeUploadError(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "文件大小超过限制")
			return
		}
		writeUploadError(w, http.StatusBadRequest, "BAD_REQUEST", "请求体不是有效 multipart/form-data")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeUploadError(w, http.StatusBadRequest, "BAD_REQUEST", "缺少上传文件 file")
		return
	}
	defer file.Close()
	response, err := save(r.Context(), file, uploadapp.FileMeta{
		ContentType: header.Header.Get("Content-Type"),
		Size:        header.Size,
	})
	if err != nil {
		h.writeServiceError(w, err, fallback)
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) requirePrincipal(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	token, ok := httpauth.BearerToken(r)
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeUploadError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	principal, ok := h.auth.DecodeAccessToken(token)
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeUploadError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	return principal, true
}

func (h *Handler) requireTeacher(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return authapp.Principal{}, false
	}
	if !authapp.IsTeacherOrAdmin(principal) {
		writeUploadError(w, http.StatusForbidden, "FORBIDDEN", "权限不足，需要教师权限")
		return authapp.Principal{}, false
	}
	return principal, true
}

func (h *Handler) allowUpload(w http.ResponseWriter, r *http.Request, principal authapp.Principal) bool {
	if h.limiter == nil || h.limiter.Allow(uploadRateKey(r, principal)) {
		return true
	}
	writeUploadError(w, http.StatusTooManyRequests, "RATE_LIMITED", "上传过于频繁，请稍后重试")
	return false
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, uploadapp.ErrInvalidContentType):
		writeUploadError(w, http.StatusUnsupportedMediaType, "INVALID_CONTENT_TYPE", "不支持的文件类型")
	case errors.Is(err, uploadapp.ErrFileTooLarge):
		writeUploadError(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "文件大小超过限制")
	default:
		h.logger.Error("upload failed", "error", redact.String(err.Error()))
		writeUploadError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fallback)
	}
}

func writeUploadError(w http.ResponseWriter, status int, code, message string) {
	httpjson.WriteDetailError(w, status, code, message)
}

func uploadRateKey(r *http.Request, principal authapp.Principal) string {
	if strings.TrimSpace(principal.UserID) != "" {
		return "user:" + principal.UserID
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		host = r.RemoteAddr
	}
	if host == "" {
		host = "unknown"
	}
	return "ip:" + host
}

type uploadRateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	now    func() time.Time
	hits   map[string][]time.Time
}

func newUploadRateLimiter(limit int, window time.Duration) *uploadRateLimiter {
	if limit <= 0 {
		limit = uploadRateLimitMax
	}
	if window <= 0 {
		window = uploadRateLimitWindow
	}
	return &uploadRateLimiter{
		limit:  limit,
		window: window,
		now:    func() time.Time { return time.Now().UTC() },
		hits:   map[string][]time.Time{},
	}
}

func (l *uploadRateLimiter) Allow(key string) bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.limit <= 0 {
		l.limit = uploadRateLimitMax
	}
	if l.window <= 0 {
		l.window = uploadRateLimitWindow
	}
	if l.now == nil {
		l.now = func() time.Time { return time.Now().UTC() }
	}
	if l.hits == nil {
		l.hits = map[string][]time.Time{}
	}
	now := l.now()
	cutoff := now.Add(-l.window)
	recent := l.hits[key][:0]
	for _, hit := range l.hits[key] {
		if hit.After(cutoff) {
			recent = append(recent, hit)
		}
	}
	if len(recent) >= l.limit {
		l.hits[key] = recent
		return false
	}
	l.hits[key] = append(recent, now)
	return true
}
