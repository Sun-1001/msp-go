package announcementhttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	announcementapp "mathstudy/backend-go/internal/application/announcement"
	authapp "mathstudy/backend-go/internal/application/auth"
	"mathstudy/backend-go/internal/domain/user"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/redact"
)

// Service is the announcement application surface used by HTTP handlers.
type Service interface {
	ListForAdmin(context.Context) (announcementapp.ListResponse, error)
	ListForUser(context.Context, string, user.Role) (announcementapp.ListResponse, error)
	Create(context.Context, string, announcementapp.SaveRequest) (announcementapp.Announcement, error)
	Update(context.Context, string, announcementapp.SaveRequest) (announcementapp.Announcement, error)
	Delete(context.Context, string) (announcementapp.DeleteResponse, error)
	Dismiss(context.Context, string, string, user.Role) (announcementapp.DismissResponse, error)
}

// Authenticator decodes access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves administrator and recipient announcement endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates an announcement HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("announcement service is nil")
	}
	if auth == nil {
		return nil, errors.New("announcement authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// RegisterAdmin attaches administrator CRUD routes under prefix.
func (h *Handler) RegisterAdmin(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix, h.listForAdmin)
	mux.HandleFunc("POST "+prefix, h.create)
	mux.HandleFunc("PUT "+prefix+"/{announcement_id}", h.update)
	mux.HandleFunc("DELETE "+prefix+"/{announcement_id}", h.delete)
}

// RegisterUser attaches student and teacher announcement routes under prefix.
func (h *Handler) RegisterUser(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix, h.listForUser)
	mux.HandleFunc("POST "+prefix+"/{announcement_id}/dismiss", h.dismiss)
}

func (h *Handler) listForAdmin(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.ListForAdmin(r.Context())
	if err != nil {
		h.writeServiceError(w, err, "获取系统公告列表失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	var request announcementapp.SaveRequest
	if !httpjson.DecodeStrictOrDetailError(w, r, 1<<20, &request) {
		return
	}
	response, err := h.service.Create(r.Context(), principal.UserID, request)
	if err != nil {
		h.writeServiceError(w, err, "发布系统公告失败")
		return
	}
	httpjson.Write(w, http.StatusCreated, response)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var request announcementapp.SaveRequest
	if !httpjson.DecodeStrictOrDetailError(w, r, 1<<20, &request) {
		return
	}
	response, err := h.service.Update(r.Context(), r.PathValue("announcement_id"), request)
	if err != nil {
		h.writeServiceError(w, err, "更新系统公告失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.Delete(r.Context(), r.PathValue("announcement_id"))
	if err != nil {
		h.writeServiceError(w, err, "删除系统公告失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) listForUser(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireRecipient(w, r)
	if !ok {
		return
	}
	response, err := h.service.ListForUser(r.Context(), principal.UserID, principal.Role)
	if err != nil {
		h.writeServiceError(w, err, "获取系统公告失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) dismiss(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireRecipient(w, r)
	if !ok {
		return
	}
	response, err := h.service.Dismiss(r.Context(), r.PathValue("announcement_id"), principal.UserID, principal.Role)
	if err != nil {
		h.writeServiceError(w, err, "关闭系统公告失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccess(
		w, r, h.auth.DecodeAccessToken, authapp.IsAdmin,
		"权限不足，需要管理员权限", writeAnnouncementError,
	)
}

func (h *Handler) requireRecipient(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccess(
		w,
		r,
		h.auth.DecodeAccessToken,
		func(principal authapp.Principal) bool {
			return authapp.HasAnyRole(principal, user.RoleStudent, user.RoleTeacher)
		},
		"权限不足，仅学生或教师可以访问系统公告",
		writeAnnouncementError,
	)
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, announcementapp.ErrBadRequest):
		writeAnnouncementError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", redact.String(err.Error()))
	case errors.Is(err, announcementapp.ErrNotFound):
		writeAnnouncementError(w, http.StatusNotFound, "NOT_FOUND", redact.String(err.Error()))
	default:
		h.logger.Error("announcement request failed", "error", redact.String(err.Error()))
		writeAnnouncementError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fallback)
	}
}

func writeAnnouncementError(w http.ResponseWriter, status int, code, message string) {
	httpjson.WriteDetailError(w, status, code, message)
}
