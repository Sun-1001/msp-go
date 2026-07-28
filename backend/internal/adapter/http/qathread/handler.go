package qathreadhttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	authapp "mathstudy/backend/internal/application/auth"
	"mathstudy/backend/internal/application/messageattachment"
	qathreadapp "mathstudy/backend/internal/application/qathread"
	"mathstudy/backend/internal/domain/user"
	"mathstudy/backend/internal/platform/httpauth"
	"mathstudy/backend/internal/platform/httpjson"
	"mathstudy/backend/internal/platform/httpquery"
	"mathstudy/backend/internal/platform/ratelimit"
	"mathstudy/backend/internal/platform/redact"
)

// Service is the Q&A thread application surface used by HTTP handlers.
type Service interface {
	ListThreads(ctx context.Context, userID string, role user.Role, search string, status string, className string, teacherID string, page int, pageSize int) (qathreadapp.ListResponse, error)
	GetThread(ctx context.Context, userID string, threadID string, role user.Role, page int, pageSize int) (any, error)
	AcknowledgeThreadRead(ctx context.Context, userID string, role user.Role, threadID string, throughMessageID string) error
	CreateThread(ctx context.Context, studentID string, teacherID string, content string, source string, attachments []messageattachment.Attachment) (qathreadapp.ThreadDetail, error)
	CreateThreadMessage(ctx context.Context, threadID string, senderID string, senderRole string, text string, attachments []messageattachment.Attachment) (qathreadapp.Message, error)
	UpdateThreadStatus(ctx context.Context, threadID string, teacherID string, status string) error
}

// Authenticator decodes access tokens.
type Authenticator interface {
	DecodeActiveAccessToken(context.Context, string) (authapp.Principal, bool, error)
}

// Handler serves /qa-threads endpoints.
type Handler struct {
	service       Service
	auth          Authenticator
	logger        *slog.Logger
	writeLimiter  *ratelimit.Limiter
	searchLimiter *ratelimit.Limiter
}

// Option customizes the Q&A thread HTTP handler.
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

// NewHandler creates a Q&A thread HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator, options ...Option) (*Handler, error) {
	if service == nil {
		return nil, errors.New("qathread service is nil")
	}
	if auth == nil {
		return nil, errors.New("qathread authenticator is nil")
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

// Register attaches Q&A thread routes under prefix.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("POST "+prefix+"/import", h.importThread)
	mux.HandleFunc("POST "+prefix, h.createThread)
	mux.HandleFunc("GET "+prefix+"/{id}", h.getThread)
	mux.HandleFunc("GET "+prefix, h.listThreads)
	mux.HandleFunc("PUT "+prefix+"/{id}/read", h.acknowledgeRead)
	mux.HandleFunc("POST "+prefix+"/{id}/messages", h.createMessage)
	mux.HandleFunc("PUT "+prefix+"/{id}/status", h.updateStatus)
}

const (
	maxJSONBodyBytes = 1 << 20
	maxPageNumber    = 10000
)

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (h *Handler) listThreads(w http.ResponseWriter, r *http.Request) {
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
	response, err := h.service.ListThreads(r.Context(), principal.UserID, principal.Role,
		q.Get("search"), q.Get("status"), q.Get("class_name"), q.Get("teacher_id"), page, pageSize)
	if err != nil {
		if errors.Is(err, qathreadapp.ErrInvalidInput) {
			writeQAError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "筛选条件长度或格式无效")
			return
		}
		h.logError("list threads failed", err)
		writeQAError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取提问列表失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) getThread(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireMessageUser(w, r)
	if !ok {
		return
	}
	page, ok := parseBoundedInt(w, r.URL.Query().Get("messages_page"), 1, 1, maxPageNumber, "messages_page")
	if !ok {
		return
	}
	pageSize, ok := parseBoundedInt(w, r.URL.Query().Get("messages_page_size"), 50, 1, 100, "messages_page_size")
	if !ok {
		return
	}
	response, err := h.service.GetThread(r.Context(), principal.UserID, r.PathValue("id"), principal.Role, page, pageSize)
	if err != nil {
		if errors.Is(err, qathreadapp.ErrInvalidInput) {
			writeQAError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "提问 ID 无效")
			return
		}
		if errors.Is(err, qathreadapp.ErrNotFound) {
			writeQAError(w, http.StatusNotFound, "NOT_FOUND", "提问不存在")
			return
		}
		h.logError("get thread failed", err)
		writeQAError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取提问失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

type acknowledgeReadRequest struct {
	ThroughMessageID string `json:"through_message_id"`
}

func (h *Handler) acknowledgeRead(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireMessageUser(w, r)
	if !ok {
		return
	}
	if !h.allowWrite(w, r, principal.UserID) {
		return
	}
	var req acknowledgeReadRequest
	if !httpjson.DecodeStrictOrBadRequest(w, r, maxJSONBodyBytes, &req) {
		return
	}
	if strings.TrimSpace(req.ThroughMessageID) == "" {
		writeQAError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "through_message_id 不能为空")
		return
	}
	if err := h.service.AcknowledgeThreadRead(r.Context(), principal.UserID, principal.Role, r.PathValue("id"), req.ThroughMessageID); err != nil {
		if errors.Is(err, qathreadapp.ErrInvalidInput) {
			writeQAError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "已读确认参数无效")
			return
		}
		if errors.Is(err, qathreadapp.ErrNotFound) {
			writeQAError(w, http.StatusNotFound, "NOT_FOUND", "提问或消息不存在")
			return
		}
		h.logError("acknowledge thread read failed", err)
		writeQAError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "确认提问已读失败")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type createThreadRequest struct {
	TeacherID   string                         `json:"teacher_id"`
	Content     string                         `json:"content"`
	Source      string                         `json:"source"`
	Attachments []messageattachment.Attachment `json:"attachments"`
}

func (h *Handler) createThread(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireStudent(w, r)
	if !ok {
		return
	}
	if !h.allowWrite(w, r, principal.UserID) {
		return
	}
	var req createThreadRequest
	if !httpjson.DecodeStrictOrBadRequest(w, r, maxJSONBodyBytes, &req) {
		return
	}
	if strings.TrimSpace(req.Content) == "" && len(req.Attachments) == 0 {
		writeQAError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "问题文字和附件不能同时为空")
		return
	}
	if req.Source == "" {
		req.Source = "消息中心"
	}
	response, err := h.service.CreateThread(r.Context(), principal.UserID, req.TeacherID, req.Content, req.Source, req.Attachments)
	if err != nil {
		if errors.Is(err, qathreadapp.ErrInvalidInput) {
			writeQAError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "teacher_id、content 或 source 长度或格式无效")
			return
		}
		if errors.Is(err, qathreadapp.ErrForbidden) {
			writeQAError(w, http.StatusForbidden, "FORBIDDEN", "只能向本班教师发起答疑")
			return
		}
		h.logError("create thread failed", err)
		writeQAError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "创建提问失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

type importThreadRequest struct {
	TeacherID   string                         `json:"teacher_id"`
	Source      string                         `json:"source"`
	Content     string                         `json:"content"`
	Attachments []messageattachment.Attachment `json:"attachments"`
}

func (h *Handler) importThread(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireStudent(w, r)
	if !ok {
		return
	}
	if !h.allowWrite(w, r, principal.UserID) {
		return
	}
	var req importThreadRequest
	if !httpjson.DecodeStrictOrBadRequest(w, r, maxJSONBodyBytes, &req) {
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeQAError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "content 不能为空")
		return
	}
	response, err := h.service.CreateThread(r.Context(), principal.UserID, req.TeacherID, req.Content, req.Source, req.Attachments)
	if err != nil {
		if errors.Is(err, qathreadapp.ErrInvalidInput) {
			writeQAError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "teacher_id、content 或 source 长度或格式无效")
			return
		}
		if errors.Is(err, qathreadapp.ErrForbidden) {
			writeQAError(w, http.StatusForbidden, "FORBIDDEN", "只能向本班教师发起答疑")
			return
		}
		h.logError("import thread failed", err)
		writeQAError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "导入提问失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

type createMessageRequest struct {
	Text        string                         `json:"text"`
	Attachments []messageattachment.Attachment `json:"attachments"`
}

func (h *Handler) createMessage(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireMessageUser(w, r)
	if !ok {
		return
	}
	if !h.allowWrite(w, r, principal.UserID) {
		return
	}
	var req createMessageRequest
	if !httpjson.DecodeStrictOrBadRequest(w, r, maxJSONBodyBytes, &req) {
		return
	}
	if strings.TrimSpace(req.Text) == "" && len(req.Attachments) == 0 {
		writeQAError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "消息文字和附件不能同时为空")
		return
	}
	response, err := h.service.CreateThreadMessage(r.Context(), r.PathValue("id"), principal.UserID, string(principal.Role), req.Text, req.Attachments)
	if err != nil {
		if errors.Is(err, qathreadapp.ErrInvalidInput) {
			writeQAError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "消息长度或格式无效")
			return
		}
		if errors.Is(err, qathreadapp.ErrNotFound) {
			writeQAError(w, http.StatusNotFound, "NOT_FOUND", "提问不存在或无权发送消息")
			return
		}
		h.logError("create thread message failed", err)
		writeQAError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "发送消息失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

func (h *Handler) updateStatus(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
		return
	}
	if !h.allowWrite(w, r, principal.UserID) {
		return
	}
	var req updateStatusRequest
	if !httpjson.DecodeStrictOrBadRequest(w, r, maxJSONBodyBytes, &req) {
		return
	}
	status := strings.TrimSpace(req.Status)
	if err := h.service.UpdateThreadStatus(r.Context(), r.PathValue("id"), principal.UserID, status); err != nil {
		if errors.Is(err, qathreadapp.ErrInvalidStatus) || errors.Is(err, qathreadapp.ErrInvalidInput) {
			writeQAError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "status 无效")
			return
		}
		if errors.Is(err, qathreadapp.ErrNotFound) {
			writeQAError(w, http.StatusNotFound, "NOT_FOUND", "提问不存在")
			return
		}
		h.logError("update thread status failed", err)
		writeQAError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "更新状态失败")
		return
	}
	httpjson.Write(w, http.StatusOK, map[string]string{"status": status})
}

func (h *Handler) allowWrite(w http.ResponseWriter, r *http.Request, userID string) bool {
	if h.writeLimiter == nil || h.writeLimiter.Allow(r.Context(), userID) {
		return true
	}
	w.Header().Set("Retry-After", "60")
	writeQAError(w, http.StatusTooManyRequests, "RATE_LIMITED", "消息操作过于频繁，请稍后重试")
	return false
}

func (h *Handler) allowSearch(w http.ResponseWriter, r *http.Request, userID string) bool {
	if h.searchLimiter == nil || h.searchLimiter.Allow(r.Context(), userID) {
		return true
	}
	w.Header().Set("Retry-After", "60")
	writeQAError(w, http.StatusTooManyRequests, "RATE_LIMITED", "消息搜索过于频繁，请稍后重试")
	return false
}

func parseBoundedInt(w http.ResponseWriter, raw string, fallback int, minValue int, maxValue int, name string) (int, bool) {
	value, err := httpquery.BoundedInt(raw, fallback, minValue, maxValue)
	if err != nil {
		writeQAError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", name+" 参数超出范围")
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
		"权限不足，仅学生或教师可以访问消息中心", writeQAError,
		func(err error) { h.logError("validate active access token failed", err) },
	)
}

func (h *Handler) requireStudent(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccessContext(
		w, r, h.auth.DecodeActiveAccessToken, authapp.IsStudent,
		"权限不足，需要学生权限", writeQAError,
		func(err error) { h.logError("validate active access token failed", err) },
	)
}

func (h *Handler) requireTeacher(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccessContext(
		w, r, h.auth.DecodeActiveAccessToken,
		func(principal authapp.Principal) bool { return principal.Role == user.RoleTeacher },
		"权限不足，需要教师权限", writeQAError,
		func(err error) { h.logError("validate active access token failed", err) },
	)
}

func (h *Handler) logError(message string, err error) {
	h.logger.Error(message, "error", redact.String(err.Error()))
}

func writeQAError(w http.ResponseWriter, status int, code, message string) {
	httpjson.WriteDetailError(w, status, code, message)
}
