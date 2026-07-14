package uploadhttp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	authapp "mathstudy/backend-go/internal/application/auth"
	uploadapp "mathstudy/backend-go/internal/application/upload"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/ratelimit"
	"mathstudy/backend-go/internal/platform/redact"
)

const (
	multipartMemory        = 32 << 20
	uploadRateLimitWindow  = time.Minute
	uploadRateLimitMax     = 60
	uploadRateLimitMaxKeys = 500
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
	limiter *ratelimit.Limiter
}

// Option customizes the upload HTTP handler.
type Option func(*Handler) error

// WithRedisRateLimit shares upload limits across API instances.
func WithRedisRateLimit(client *goredis.Client, maxLocalKeys int) Option {
	return func(handler *Handler) error {
		limiter, err := ratelimit.New(
			client,
			"msp:upload",
			uploadRateLimitMax,
			uploadRateLimitWindow,
			maxLocalKeys,
			handler.logger,
		)
		if err != nil {
			return err
		}
		handler.limiter = limiter
		return nil
	}
}

// NewHandler creates an upload HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator, options ...Option) (*Handler, error) {
	if service == nil {
		return nil, errors.New("upload service is nil")
	}
	if auth == nil {
		return nil, errors.New("upload authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	handler := &Handler{service: service, auth: auth, logger: logger, limiter: newUploadRateLimiter(uploadRateLimitMax, uploadRateLimitWindow)}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(handler); err != nil {
			return nil, err
		}
	}
	return handler, nil
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
	return httpauth.RequireBearerAccess(w, r, h.auth.DecodeAccessToken, nil, "", writeUploadError)
}

func (h *Handler) requireTeacher(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccess(
		w, r, h.auth.DecodeAccessToken, authapp.IsTeacherOrAdmin,
		"权限不足，需要教师权限", writeUploadError,
	)
}

func (h *Handler) allowUpload(w http.ResponseWriter, r *http.Request, principal authapp.Principal) bool {
	if h.limiter == nil || h.limiter.Allow(r.Context(), uploadRateKey(r, principal)) {
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

func newUploadRateLimiter(limit int, window time.Duration) *ratelimit.Limiter {
	if limit <= 0 {
		limit = uploadRateLimitMax
	}
	if window <= 0 {
		window = uploadRateLimitWindow
	}
	limiter, err := ratelimit.New(nil, "msp:upload", limit, window, uploadRateLimitMaxKeys, nil)
	if err != nil {
		panic(err)
	}
	return limiter
}
