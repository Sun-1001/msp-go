package noticehttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	authapp "mathstudy/backend-go/internal/application/auth"
	noticeapp "mathstudy/backend-go/internal/application/notice"
	"mathstudy/backend-go/internal/domain/user"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/httpquery"
	"mathstudy/backend-go/internal/platform/ratelimit"
	"mathstudy/backend-go/internal/platform/redact"
)

// Service is the notice application surface used by HTTP handlers.
type Service interface {
	ListNotices(ctx context.Context, userID string, role user.Role, search string, status string, className string, page int, pageSize int) (noticeapp.ListResponse, error)
	GetNotice(ctx context.Context, userID string, noticeID string, role user.Role) (any, error)
	CreateNotice(ctx context.Context, teacherID string, classID string, title string, body string) (noticeapp.TeacherNoticeItem, error)
	ConfirmNotice(ctx context.Context, noticeID string, studentID string) error
}

// Authenticator decodes access tokens.
type Authenticator interface {
	DecodeActiveAccessToken(context.Context, string) (authapp.Principal, bool, error)
}

// Handler serves /notices endpoints.
type Handler struct {
	service       Service
	auth          Authenticator
	logger        *slog.Logger
	writeLimiter  *ratelimit.Limiter
	searchLimiter *ratelimit.Limiter
}

// Option customizes the notice HTTP handler.
type Option func(*Handler)

// WithWriteRateLimit applies the shared per-user message-center write limit.
func WithWriteRateLimit(limiter *ratelimit.Limiter) Option {
	return func(handler *Handler) {
		handler.writeLimiter = limiter
	}
}

// WithSearchRateLimit applies the shared per-user message-center search limit.
func WithSearchRateLimit(limiter *ratelimit.Limiter) Option {
	return func(handler *Handler) {
		handler.searchLimiter = limiter
	}
}

// NewHandler creates a notice HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator, options ...Option) (*Handler, error) {
	if service == nil {
		return nil, errors.New("notice service is nil")
	}
	if auth == nil {
		return nil, errors.New("notice authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	handler := &Handler{service: service, auth: auth, logger: logger}
	for _, option := range options {
		if option != nil {
			option(handler)
		}
	}
	return handler, nil
}

// Register attaches notice routes under prefix.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("POST "+prefix, h.createNotice)
	mux.HandleFunc("GET "+prefix+"/{id}", h.getNotice)
	mux.HandleFunc("GET "+prefix, h.listNotices)
	mux.HandleFunc("POST "+prefix+"/{id}/confirm", h.confirmNotice)
}

const (
	maxJSONBodyBytes = 1 << 20
	maxPageNumber    = 10000
)

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (h *Handler) listNotices(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireMessageUser(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	page, ok := parseBoundedInt(w, q.Get("page"), 1, 1, maxPageNumber, "page")
	if !ok {
		return
	}
	pageSize, ok := parseBoundedInt(w, q.Get("page_size"), 20, 1, 100, "page_size")
	if !ok {
		return
	}
	if strings.TrimSpace(q.Get("search")) != "" && !h.allowSearch(w, r, principal.UserID) {
		return
	}
	response, err := h.service.ListNotices(r.Context(), principal.UserID, principal.Role,
		q.Get("search"), q.Get("status"), q.Get("class_name"), page, pageSize)
	if err != nil {
		if errors.Is(err, noticeapp.ErrInvalidInput) {
			writeNoticeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "筛选条件长度或格式无效")
			return
		}
		h.logError("list notices failed", err)
		writeNoticeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取通知列表失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) getNotice(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireMessageUser(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetNotice(r.Context(), principal.UserID, r.PathValue("id"), principal.Role)
	if err != nil {
		if errors.Is(err, noticeapp.ErrInvalidInput) {
			writeNoticeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "通知 ID 无效")
			return
		}
		if errors.Is(err, noticeapp.ErrNotFound) {
			writeNoticeError(w, http.StatusNotFound, "NOT_FOUND", "通知不存在")
			return
		}
		h.logError("get notice failed", err)
		writeNoticeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取通知失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

type createNoticeRequest struct {
	ClassID string `json:"class_id"`
	Title   string `json:"title"`
	Body    string `json:"body"`
}

func (h *Handler) createNotice(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
		return
	}
	if !h.allowWrite(w, r, principal.UserID) {
		return
	}
	var req createNoticeRequest
	if !httpjson.DecodeStrictOrBadRequest(w, r, maxJSONBodyBytes, &req) {
		return
	}
	if strings.TrimSpace(req.ClassID) == "" || strings.TrimSpace(req.Title) == "" {
		writeNoticeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "class_id 和 title 不能为空")
		return
	}
	response, err := h.service.CreateNotice(r.Context(), principal.UserID, req.ClassID, req.Title, req.Body)
	if err != nil {
		if errors.Is(err, noticeapp.ErrInvalidInput) {
			writeNoticeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "通知标题、正文或班级 ID 长度或格式无效")
			return
		}
		if errors.Is(err, noticeapp.ErrForbidden) {
			writeNoticeError(w, http.StatusForbidden, "FORBIDDEN", "只能向本人班级发布通知")
			return
		}
		h.logError("create notice failed", err)
		writeNoticeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "发布通知失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) confirmNotice(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireStudent(w, r)
	if !ok {
		return
	}
	if !h.allowWrite(w, r, principal.UserID) {
		return
	}
	if err := h.service.ConfirmNotice(r.Context(), r.PathValue("id"), principal.UserID); err != nil {
		if errors.Is(err, noticeapp.ErrInvalidInput) {
			writeNoticeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "通知 ID 无效")
			return
		}
		if errors.Is(err, noticeapp.ErrForbidden) {
			writeNoticeError(w, http.StatusForbidden, "FORBIDDEN", "无权确认该通知")
			return
		}
		if errors.Is(err, noticeapp.ErrNotFound) {
			writeNoticeError(w, http.StatusNotFound, "NOT_FOUND", "通知不存在")
			return
		}
		h.logError("confirm notice failed", err)
		writeNoticeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "确认通知失败")
		return
	}
	httpjson.Write(w, http.StatusOK, map[string]string{"status": "confirmed"})
}

func (h *Handler) allowWrite(w http.ResponseWriter, r *http.Request, userID string) bool {
	if h.writeLimiter == nil || h.writeLimiter.Allow(r.Context(), userID) {
		return true
	}
	w.Header().Set("Retry-After", "60")
	writeNoticeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "消息操作过于频繁，请稍后重试")
	return false
}

func (h *Handler) allowSearch(w http.ResponseWriter, r *http.Request, userID string) bool {
	if h.searchLimiter == nil || h.searchLimiter.Allow(r.Context(), userID) {
		return true
	}
	w.Header().Set("Retry-After", "60")
	writeNoticeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "消息搜索过于频繁，请稍后重试")
	return false
}

func parseBoundedInt(w http.ResponseWriter, raw string, fallback int, minValue int, maxValue int, name string) (int, bool) {
	value, err := httpquery.BoundedInt(raw, fallback, minValue, maxValue)
	if err != nil {
		writeNoticeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", name+" 参数超出范围")
		return 0, false
	}
	return value, true
}

// ---------------------------------------------------------------------------
// Auth helpers
// ---------------------------------------------------------------------------

func (h *Handler) requireMessageUser(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccessContext(
		w, r, h.auth.DecodeActiveAccessToken,
		func(principal authapp.Principal) bool {
			return authapp.HasAnyRole(principal, user.RoleStudent, user.RoleTeacher)
		},
		"权限不足，仅学生或教师可以访问消息中心", writeNoticeError,
		func(err error) { h.logError("validate active access token failed", err) },
	)
}

func (h *Handler) requireStudent(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccessContext(
		w, r, h.auth.DecodeActiveAccessToken, authapp.IsStudent,
		"权限不足，需要学生权限", writeNoticeError,
		func(err error) { h.logError("validate active access token failed", err) },
	)
}

func (h *Handler) requireTeacher(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccessContext(
		w, r, h.auth.DecodeActiveAccessToken,
		func(principal authapp.Principal) bool { return principal.Role == user.RoleTeacher },
		"权限不足，需要教师权限", writeNoticeError,
		func(err error) { h.logError("validate active access token failed", err) },
	)
}

func (h *Handler) logError(message string, err error) {
	h.logger.Error(message, "error", redact.String(err.Error()))
}

func writeNoticeError(w http.ResponseWriter, status int, code, message string) {
	httpjson.WriteDetailError(w, status, code, message)
}
