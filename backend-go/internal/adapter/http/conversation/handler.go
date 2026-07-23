package conversationhttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	authapp "mathstudy/backend-go/internal/application/auth"
	conversationapp "mathstudy/backend-go/internal/application/conversation"
	"mathstudy/backend-go/internal/domain/user"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/redact"
)

// Service is the conversation application surface used by HTTP handlers.
type Service interface {
	ListConversations(ctx context.Context, userID string, role user.Role, search string, status string, className string, page int, pageSize int) (conversationapp.ListResponse, error)
	GetConversation(ctx context.Context, userID string, conversationID string, page int, pageSize int) (conversationapp.ConversationDetail, error)
	CreateConversation(ctx context.Context, creatorID string, creatorRole user.Role, targetID string, subject string, initialMessage string) (conversationapp.ConversationDetail, error)
	SendMessage(ctx context.Context, conversationID string, senderID string, senderRole string, text string) (conversationapp.Message, error)
	ArchiveConversation(ctx context.Context, conversationID string, studentID string) error
	DeleteConversation(ctx context.Context, conversationID string, studentID string) error
	ListTeacherContacts(ctx context.Context, studentID string) ([]conversationapp.Contact, error)
	ListStudentContacts(ctx context.Context, teacherID string) ([]conversationapp.Contact, error)
	SearchContacts(ctx context.Context, query string, role user.Role) ([]conversationapp.Contact, error)
}

// Authenticator decodes access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves /conversations endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates a conversation HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("conversation service is nil")
	}
	if auth == nil {
		return nil, errors.New("conversation authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches conversation routes under prefix.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix+"/contacts/teachers", h.listTeacherContacts)
	mux.HandleFunc("GET "+prefix+"/contacts/students", h.listStudentContacts)
	mux.HandleFunc("GET "+prefix+"/search-users", h.searchUsers)
	mux.HandleFunc("POST "+prefix, h.createConversation)
	mux.HandleFunc("GET "+prefix+"/{id}", h.getConversation)
	mux.HandleFunc("GET "+prefix, h.listConversations)
	mux.HandleFunc("POST "+prefix+"/{id}/messages", h.sendMessage)
	mux.HandleFunc("PUT "+prefix+"/{id}/archive", h.archiveConversation)
	mux.HandleFunc("DELETE "+prefix+"/{id}", h.deleteConversation)
}

const maxJSONBodyBytes = 1 << 20

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (h *Handler) listConversations(w http.ResponseWriter, r *http.Request) {
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
		writeConvError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "page_size 必须在 1 到 100 之间")
		return
	}
	response, err := h.service.ListConversations(r.Context(), principal.UserID, principal.Role,
		q.Get("search"), q.Get("status"), q.Get("class_name"), page, pageSize)
	if err != nil {
		h.logError("list conversations failed", err)
		writeConvError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取会话列表失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) getConversation(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("messages_page"))
	if page < 1 { page = 1 }
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("messages_page_size"))
	if pageSize < 1 { pageSize = 50 }
	if pageSize > 100 { writeConvError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "messages_page_size 必须在 1 到 100 之间"); return }
	response, err := h.service.GetConversation(r.Context(), principal.UserID, r.PathValue("id"), page, pageSize)
	if err != nil {
		if errors.Is(err, conversationapp.ErrNotFound) {
			writeConvError(w, http.StatusNotFound, "NOT_FOUND", "会话不存在")
			return
		}
		h.logError("get conversation failed", err)
		writeConvError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取会话失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

type createConversationRequest struct {
	TargetID       string `json:"target_id"`
	Subject        string `json:"subject"`
	InitialMessage string `json:"initial_message"`
}

func (h *Handler) createConversation(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	var req createConversationRequest
	if !httpjson.DecodeStrictOrBadRequest(w, r, maxJSONBodyBytes, &req) {
		return
	}
	if strings.TrimSpace(req.TargetID) == "" {
		writeConvError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "target_id 不能为空")
		return
	}
	response, err := h.service.CreateConversation(r.Context(), principal.UserID, principal.Role, req.TargetID, req.Subject, req.InitialMessage)
	if err != nil {
		if errors.Is(err, conversationapp.ErrForbidden) {
			writeConvError(w, http.StatusForbidden, "FORBIDDEN", "目标用户无效或无权创建会话")
			return
		}
		if errors.Is(err, conversationapp.ErrConflict) {
			writeConvError(w, http.StatusConflict, "CONFLICT", "会话已存在")
			return
		}
		h.logError("create conversation failed", err)
		writeConvError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "创建会话失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

type sendMessageRequest struct {
	Text string `json:"text"`
}

func (h *Handler) sendMessage(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	var req sendMessageRequest
	if !httpjson.DecodeStrictOrBadRequest(w, r, maxJSONBodyBytes, &req) {
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeConvError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "text 不能为空")
		return
	}
	senderRole := string(principal.Role)
	response, err := h.service.SendMessage(r.Context(), r.PathValue("id"), principal.UserID, senderRole, req.Text)
	if err != nil {
		if errors.Is(err, conversationapp.ErrNotFound) {
			writeConvError(w, http.StatusNotFound, "NOT_FOUND", "会话不存在或无权发送消息")
			return
		}
		h.logError("send message failed", err)
		writeConvError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "发送消息失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) archiveConversation(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireStudent(w, r)
	if !ok {
		return
	}
	if err := h.service.ArchiveConversation(r.Context(), r.PathValue("id"), principal.UserID); err != nil {
		if errors.Is(err, conversationapp.ErrNotFound) {
			writeConvError(w, http.StatusNotFound, "NOT_FOUND", "会话不存在")
			return
		}
		h.logError("archive conversation failed", err)
		writeConvError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "归档会话失败")
		return
	}
	httpjson.Write(w, http.StatusOK, map[string]string{"status": "archived"})
}

func (h *Handler) deleteConversation(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireStudent(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteConversation(r.Context(), r.PathValue("id"), principal.UserID); err != nil {
		if errors.Is(err, conversationapp.ErrNotFound) {
			writeConvError(w, http.StatusNotFound, "NOT_FOUND", "会话不存在")
			return
		}
		h.logError("delete conversation failed", err)
		writeConvError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "删除会话失败")
		return
	}
	httpjson.Write(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) listTeacherContacts(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireStudent(w, r)
	if !ok {
		return
	}
	contacts, err := h.service.ListTeacherContacts(r.Context(), principal.UserID)
	if err != nil {
		h.logError("list teacher contacts failed", err)
		writeConvError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取联系人失败")
		return
	}
	if contacts == nil {
		contacts = []conversationapp.Contact{}
	}
	httpjson.Write(w, http.StatusOK, map[string]any{"contacts": contacts})
}

func (h *Handler) listStudentContacts(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
		return
	}
	contacts, err := h.service.ListStudentContacts(r.Context(), principal.UserID)
	if err != nil {
		h.logError("list student contacts failed", err)
		writeConvError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取联系人失败")
		return
	}
	if contacts == nil {
		contacts = []conversationapp.Contact{}
	}
	httpjson.Write(w, http.StatusOK, map[string]any{"contacts": contacts})
}

func (h *Handler) searchUsers(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		httpjson.Write(w, http.StatusOK, map[string]any{"contacts": []conversationapp.Contact{}})
		return
	}
	contacts, err := h.service.SearchContacts(r.Context(), q, principal.Role)
	if err != nil {
		h.logError("search users failed", err)
		writeConvError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "搜索用户失败")
		return
	}
	if contacts == nil {
		contacts = []conversationapp.Contact{}
	}
	httpjson.Write(w, http.StatusOK, map[string]any{"contacts": contacts})
}

// ---------------------------------------------------------------------------
// Auth helpers
// ---------------------------------------------------------------------------

func (h *Handler) requirePrincipal(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccess(w, r, h.auth.DecodeAccessToken, nil, "", writeConvError)
}

func (h *Handler) requireStudent(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccess(
		w, r, h.auth.DecodeAccessToken, authapp.IsStudent,
		"权限不足，需要学生权限", writeConvError,
	)
}

func (h *Handler) requireTeacher(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccess(
		w, r, h.auth.DecodeAccessToken, authapp.IsTeacherOrAdmin,
		"权限不足，需要教师权限", writeConvError,
	)
}

func (h *Handler) logError(message string, err error) {
	h.logger.Error(message, "error", redact.String(err.Error()))
}

func writeConvError(w http.ResponseWriter, status int, code, message string) {
	httpjson.WriteDetailError(w, status, code, message)
}
