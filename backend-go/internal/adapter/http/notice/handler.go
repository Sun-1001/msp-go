package noticehttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	authapp "mathstudy/backend-go/internal/application/auth"
	noticeapp "mathstudy/backend-go/internal/application/notice"
	"mathstudy/backend-go/internal/domain/user"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/redact"
)

// Service is the notice application surface used by HTTP handlers.
type Service interface {
	ListNotices(ctx context.Context, userID string, role user.Role, search string, status string, className string, page int, pageSize int) (noticeapp.ListResponse, error)
	GetNotice(ctx context.Context, userID string, noticeID string, role user.Role) (any, error)
	CreateNotice(ctx context.Context, teacherID string, classID string, title string, body string) (noticeapp.TeacherNoticeItem, error)
	ConfirmNotice(ctx context.Context, noticeID string, studentID string) error
	RemindUnconfirmed(ctx context.Context, noticeID string, teacherID string) ([]string, error)
}

// Authenticator decodes access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves /notices endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates a notice HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("notice service is nil")
	}
	if auth == nil {
		return nil, errors.New("notice authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches notice routes under prefix.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("POST "+prefix, h.createNotice)
	mux.HandleFunc("GET "+prefix+"/{id}", h.getNotice)
	mux.HandleFunc("GET "+prefix, h.listNotices)
	mux.HandleFunc("POST "+prefix+"/{id}/confirm", h.confirmNotice)
	mux.HandleFunc("POST "+prefix+"/{id}/remind", h.remindUnconfirmed)
}

const maxJSONBodyBytes = 1 << 20

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (h *Handler) listNotices(w http.ResponseWriter, r *http.Request) {
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
		writeNoticeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "page_size 必须在 1 到 100 之间")
		return
	}
	response, err := h.service.ListNotices(r.Context(), principal.UserID, principal.Role,
		q.Get("search"), q.Get("status"), q.Get("class_name"), page, pageSize)
	if err != nil {
		h.logError("list notices failed", err)
		writeNoticeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取通知列表失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) getNotice(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetNotice(r.Context(), principal.UserID, r.PathValue("id"), principal.Role)
	if err != nil {
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
	if err := h.service.ConfirmNotice(r.Context(), r.PathValue("id"), principal.UserID); err != nil {
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

func (h *Handler) remindUnconfirmed(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
		return
	}
	names, err := h.service.RemindUnconfirmed(r.Context(), r.PathValue("id"), principal.UserID)
	if err != nil {
		if errors.Is(err, noticeapp.ErrNotFound) {
			writeNoticeError(w, http.StatusNotFound, "NOT_FOUND", "通知不存在")
			return
		}
		h.logError("remind unconfirmed failed", err)
		writeNoticeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "提醒失败")
		return
	}
	httpjson.Write(w, http.StatusOK, map[string]any{
		"unconfirmed_students": names,
		"count":                len(names),
	})
}

// ---------------------------------------------------------------------------
// Auth helpers
// ---------------------------------------------------------------------------

func (h *Handler) requirePrincipal(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccess(w, r, h.auth.DecodeAccessToken, nil, "", writeNoticeError)
}

func (h *Handler) requireStudent(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccess(
		w, r, h.auth.DecodeAccessToken, authapp.IsStudent,
		"权限不足，需要学生权限", writeNoticeError,
	)
}

func (h *Handler) requireTeacher(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccess(
		w, r, h.auth.DecodeAccessToken, authapp.IsTeacherOrAdmin,
		"权限不足，需要教师权限", writeNoticeError,
	)
}

func (h *Handler) logError(message string, err error) {
	h.logger.Error(message, "error", redact.String(err.Error()))
}

func writeNoticeError(w http.ResponseWriter, status int, code, message string) {
	httpjson.WriteDetailError(w, status, code, message)
}
