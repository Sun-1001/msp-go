package sessionhttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	authapp "mathstudy/backend-go/internal/application/auth"
	sessionapp "mathstudy/backend-go/internal/application/session"
)

// Service is the session application surface used by HTTP handlers.
type Service interface {
	CreateSession(context.Context, string, *string, string) (sessionapp.CreateSessionResponse, error)
	ProcessChatFallback(context.Context, string, string, string, []string) (sessionapp.ChatResult, error)
	GetHistory(context.Context, string, string, int, int) (sessionapp.HistoryResponse, error)
	GetSessions(context.Context, string, int, int) (sessionapp.SessionListResponse, error)
	EndSession(context.Context, string, string) (sessionapp.EndResponse, error)
	UpdateSessionMode(context.Context, string, string, string) (sessionapp.UpdateModeResponse, error)
	DeleteSession(context.Context, string, string) (sessionapp.DeleteResponse, error)
	BatchDeleteSessions(context.Context, []string, string) (sessionapp.BatchDeleteResponse, error)
	CancelTask(context.Context, string, string) (sessionapp.CancelTaskResponse, error)
}

// Authenticator decodes Go/Python-compatible access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves /session endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates a session HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("session service is nil")
	}
	if auth == nil {
		return nil, errors.New("session authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches session routes under prefix, for example /api/v1/session.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("POST "+prefix+"/start", h.start)
	mux.HandleFunc("GET "+prefix+"/list", h.list)
	mux.HandleFunc("POST "+prefix+"/batch-delete", h.batchDelete)
	mux.HandleFunc("POST "+prefix+"/task/{task_id}/cancel", h.cancelTask)
	mux.HandleFunc("POST "+prefix+"/{session_id}/chat", h.chat)
	mux.HandleFunc("GET "+prefix+"/{session_id}/history", h.history)
	mux.HandleFunc("POST "+prefix+"/{session_id}/end", h.end)
	mux.HandleFunc("PATCH "+prefix+"/{session_id}/mode", h.updateMode)
	mux.HandleFunc("DELETE "+prefix+"/{session_id}", h.delete)
}

type startRequest struct {
	Topic *string `json:"topic"`
	Mode  string  `json:"mode"`
}

type chatRequest struct {
	Message     string   `json:"message"`
	Attachments []string `json:"attachments"`
}

type updateModeRequest struct {
	Mode string `json:"mode"`
}

type batchDeleteRequest struct {
	SessionIDs []string `json:"session_ids"`
}

type errorResponse struct {
	Detail  string `json:"detail"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	var request startRequest
	if r.Body != nil && r.ContentLength != 0 {
		if !decodeRequest(w, r, &request) {
			return
		}
	}
	if request.Mode == "" {
		request.Mode = "chat"
	}
	response, err := h.service.CreateSession(r.Context(), principal.UserID, request.Topic, request.Mode)
	if err != nil {
		h.logger.Error("create session failed", "error", err)
		writeSessionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "创建会话失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) chat(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	var request chatRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	if strings.TrimSpace(request.Message) == "" {
		writeSessionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "消息内容不能为空")
		return
	}
	result, err := h.service.ProcessChatFallback(r.Context(), r.PathValue("session_id"), principal.UserID, request.Message, request.Attachments)
	if err != nil {
		if errors.Is(err, sessionapp.ErrNotFound) {
			writeSessionSSEError(w, "SESSION_NOT_FOUND", "会话不存在或无权访问")
			return
		}
		h.logger.Error("process chat fallback failed", "error", err)
		writeSessionSSEError(w, "PROCESSING_ERROR", "处理消息时发生错误，请稍后重试")
		return
	}
	writeSSEChatResult(w, result)
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	limit, ok := parseIntQuery(w, r.URL.Query().Get("limit"), 50, 1, 100, "limit")
	if !ok {
		return
	}
	offset, ok := parseIntQuery(w, r.URL.Query().Get("offset"), 0, 0, 1_000_000, "offset")
	if !ok {
		return
	}
	response, err := h.service.GetHistory(r.Context(), r.PathValue("session_id"), principal.UserID, limit, offset)
	if err != nil {
		h.logger.Error("get session history failed", "error", err)
		writeSessionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取会话历史失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	limit, ok := parseIntQuery(w, r.URL.Query().Get("limit"), 20, 1, 50, "limit")
	if !ok {
		return
	}
	offset, ok := parseIntQuery(w, r.URL.Query().Get("offset"), 0, 0, 1_000_000, "offset")
	if !ok {
		return
	}
	response, err := h.service.GetSessions(r.Context(), principal.UserID, limit, offset)
	if err != nil {
		h.logger.Error("get session list failed", "error", err)
		writeSessionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取会话列表失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) end(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	response, err := h.service.EndSession(r.Context(), r.PathValue("session_id"), principal.UserID)
	if err != nil {
		if errors.Is(err, sessionapp.ErrNotFound) {
			writeSessionError(w, http.StatusNotFound, "NOT_FOUND", "会话不存在或无权访问")
			return
		}
		h.logger.Error("end session failed", "error", err)
		writeSessionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "结束会话失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) updateMode(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	var request updateModeRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	response, err := h.service.UpdateSessionMode(r.Context(), r.PathValue("session_id"), principal.UserID, request.Mode)
	if err != nil {
		if errors.Is(err, sessionapp.ErrNotFound) {
			writeSessionError(w, http.StatusNotFound, "NOT_FOUND", "会话不存在或无权访问")
			return
		}
		h.logger.Error("update session mode failed", "error", err)
		writeSessionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "更新会话模式失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	response, err := h.service.DeleteSession(r.Context(), r.PathValue("session_id"), principal.UserID)
	if err != nil {
		h.logger.Error("delete session failed", "error", err)
		writeSessionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "删除会话失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) batchDelete(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	var request batchDeleteRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	response, err := h.service.BatchDeleteSessions(r.Context(), request.SessionIDs, principal.UserID)
	if err != nil {
		h.logger.Error("batch delete sessions failed", "error", err)
		writeSessionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "批量删除会话失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) cancelTask(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	response, err := h.service.CancelTask(r.Context(), r.PathValue("task_id"), principal.UserID)
	if err != nil {
		h.logger.Error("cancel task failed", "error", err)
		writeSessionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "任务取消失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) requirePrincipal(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	fields := strings.Fields(authHeader)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeSessionError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	principal, ok := h.auth.DecodeAccessToken(fields[1])
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeSessionError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	return principal, true
}

func parseIntQuery(w http.ResponseWriter, value string, fallback int, minValue int, maxValue int, name string) (int, bool) {
	if value == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minValue || parsed > maxValue {
		writeSessionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", name+" 参数超出范围")
		return 0, false
	}
	return parsed, true
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := decoder.Decode(target); err != nil {
		writeSessionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "请求体格式错误")
		return false
	}
	return true
}

func writeSSEChatResult(w http.ResponseWriter, result sessionapp.ChatResult) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	writeSSEEvent(w, "task_info", map[string]string{"task_id": result.TaskID})
	writeSSEEvent(w, "message", map[string]any{
		"type":       "chunk",
		"content":    result.Content,
		"agent":      result.Agent,
		"message_id": result.MessageID,
	})
	writeSSEEvent(w, "message", map[string]any{
		"type":       "done",
		"message_id": result.MessageID,
		"agent":      result.Agent,
	})
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func writeSessionSSEError(w http.ResponseWriter, code string, message string) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	writeSSEEvent(w, "error", map[string]string{"type": "error", "code": code, "message": message})
}

func writeSSEEvent(w http.ResponseWriter, event string, payload any) {
	raw, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, raw)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeSessionError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Detail: message, Code: code, Message: message})
}
