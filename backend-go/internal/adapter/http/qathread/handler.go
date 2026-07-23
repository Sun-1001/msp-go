package qathreadhttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	authapp "mathstudy/backend-go/internal/application/auth"
	qathreadapp "mathstudy/backend-go/internal/application/qathread"
	"mathstudy/backend-go/internal/domain/user"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/redact"
)

// Service is the Q&A thread application surface used by HTTP handlers.
type Service interface {
	ListThreads(ctx context.Context, userID string, role user.Role, search string, status string, className string, teacherID string, page int, pageSize int) (qathreadapp.ListResponse, error)
	GetThread(ctx context.Context, userID string, threadID string, role user.Role, page int, pageSize int) (any, error)
	CreateThread(ctx context.Context, studentID string, teacherID string, content string, source string) (qathreadapp.ThreadDetail, error)
	CreateThreadMessage(ctx context.Context, threadID string, senderID string, senderRole string, text string) (qathreadapp.Message, error)
	UpdateThreadStatus(ctx context.Context, threadID string, teacherID string, status string) error
	DeleteThread(ctx context.Context, threadID string, studentID string) error
}

// Authenticator decodes access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves /qa-threads endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates a Q&A thread HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("qathread service is nil")
	}
	if auth == nil {
		return nil, errors.New("qathread authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches Q&A thread routes under prefix.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("POST "+prefix+"/import", h.importThread)
	mux.HandleFunc("POST "+prefix, h.createThread)
	mux.HandleFunc("GET "+prefix+"/{id}", h.getThread)
	mux.HandleFunc("GET "+prefix, h.listThreads)
	mux.HandleFunc("POST "+prefix+"/{id}/messages", h.createMessage)
	mux.HandleFunc("PUT "+prefix+"/{id}/status", h.updateStatus)
	mux.HandleFunc("DELETE "+prefix+"/{id}", h.deleteThread)
}

const maxJSONBodyBytes = 1 << 20

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (h *Handler) listThreads(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(q.Get("page_size"))
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		writeQAError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "page_size 必须在 1 到 100 之间")
		return
	}
	response, err := h.service.ListThreads(r.Context(), principal.UserID, principal.Role,
		q.Get("search"), q.Get("status"), q.Get("class_name"), q.Get("teacher_id"), page, pageSize)
	if err != nil {
		h.logError("list threads failed", err)
		writeQAError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取提问列表失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) getThread(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("messages_page"))
	if page < 1 { page = 1 }
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("messages_page_size"))
	if pageSize < 1 { pageSize = 50 }
	if pageSize > 100 { writeQAError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "messages_page_size 必须在 1 到 100 之间"); return }
	response, err := h.service.GetThread(r.Context(), principal.UserID, r.PathValue("id"), principal.Role, page, pageSize)
	if err != nil {
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

type createThreadRequest struct {
	TeacherID string `json:"teacher_id"`
	Content   string `json:"content"`
	Source    string `json:"source"`
}

func (h *Handler) createThread(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireStudent(w, r)
	if !ok {
		return
	}
	var req createThreadRequest
	if !httpjson.DecodeStrictOrBadRequest(w, r, maxJSONBodyBytes, &req) {
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeQAError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "content 不能为空")
		return
	}
	if req.Source == "" {
		req.Source = "消息中心"
	}
	response, err := h.service.CreateThread(r.Context(), principal.UserID, req.TeacherID, req.Content, req.Source)
	if err != nil {
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
	TeacherID string `json:"teacher_id"`
	Source    string `json:"source"`
	Content   string `json:"content"`
}

func (h *Handler) importThread(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireStudent(w, r)
	if !ok {
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
	response, err := h.service.CreateThread(r.Context(), principal.UserID, req.TeacherID, req.Content, req.Source)
	if err != nil {
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
	Text string `json:"text"`
}

func (h *Handler) createMessage(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	var req createMessageRequest
	if !httpjson.DecodeStrictOrBadRequest(w, r, maxJSONBodyBytes, &req) {
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeQAError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "text 不能为空")
		return
	}
	response, err := h.service.CreateThreadMessage(r.Context(), r.PathValue("id"), principal.UserID, string(principal.Role), req.Text)
	if err != nil {
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
	var req updateStatusRequest
	if !httpjson.DecodeStrictOrBadRequest(w, r, maxJSONBodyBytes, &req) {
		return
	}
	if err := h.service.UpdateThreadStatus(r.Context(), r.PathValue("id"), principal.UserID, req.Status); err != nil {
		if errors.Is(err, qathreadapp.ErrInvalidStatus) {
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
	httpjson.Write(w, http.StatusOK, map[string]string{"status": req.Status})
}

func (h *Handler) deleteThread(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireStudent(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteThread(r.Context(), r.PathValue("id"), principal.UserID); err != nil {
		if errors.Is(err, qathreadapp.ErrNotFound) {
			writeQAError(w, http.StatusNotFound, "NOT_FOUND", "提问不存在")
			return
		}
		h.logError("delete thread failed", err)
		writeQAError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "删除提问失败")
		return
	}
	httpjson.Write(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ---------------------------------------------------------------------------
// Auth helpers
// ---------------------------------------------------------------------------

func (h *Handler) requirePrincipal(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccess(w, r, h.auth.DecodeAccessToken, nil, "", writeQAError)
}

func (h *Handler) requireStudent(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccess(
		w, r, h.auth.DecodeAccessToken, authapp.IsStudent,
		"权限不足，需要学生权限", writeQAError,
	)
}

func (h *Handler) requireTeacher(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccess(
		w, r, h.auth.DecodeAccessToken, authapp.IsTeacherOrAdmin,
		"权限不足，需要教师权限", writeQAError,
	)
}

func (h *Handler) logError(message string, err error) {
	h.logger.Error(message, "error", redact.String(err.Error()))
}

func writeQAError(w http.ResponseWriter, status int, code, message string) {
	httpjson.WriteDetailError(w, status, code, message)
}
