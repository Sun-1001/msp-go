package messagecenterhttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	authapp "mathstudy/backend/internal/application/auth"
	messagecenterapp "mathstudy/backend/internal/application/messagecenter"
	"mathstudy/backend/internal/domain/user"
	"mathstudy/backend/internal/platform/httpauth"
	"mathstudy/backend/internal/platform/httpjson"
	"mathstudy/backend/internal/platform/redact"
)

// Service is the message center application surface used by HTTP handlers.
type Service interface {
	Summary(context.Context, string, user.Role) (messagecenterapp.Summary, error)
}

// Authenticator decodes access tokens.
type Authenticator interface {
	DecodeActiveAccessToken(context.Context, string) (authapp.Principal, bool, error)
}

// Handler serves message center summary endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates a message center HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("message center service is nil")
	}
	if auth == nil {
		return nil, errors.New("message center authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches message center routes under prefix.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix+"/summary", h.summary)
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireMessageUser(w, r)
	if !ok {
		return
	}

	response, err := h.service.Summary(r.Context(), principal.UserID, principal.Role)
	if err != nil {
		if errors.Is(err, messagecenterapp.ErrForbidden) {
			writeMessageCenterError(w, http.StatusForbidden, "FORBIDDEN", "权限不足，仅学生或教师可以访问消息中心")
			return
		}
		h.logger.Error("get message center summary failed", "error", redact.String(err.Error()))
		writeMessageCenterError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取消息预览失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) requireMessageUser(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccessContext(
		w, r, h.auth.DecodeActiveAccessToken,
		func(principal authapp.Principal) bool {
			return authapp.HasAnyRole(principal, user.RoleStudent, user.RoleTeacher)
		},
		"权限不足，仅学生或教师可以访问消息中心", writeMessageCenterError,
		func(err error) {
			h.logger.Error("validate active access token failed", "error", redact.String(err.Error()))
		},
	)
}

func writeMessageCenterError(w http.ResponseWriter, status int, code, message string) {
	httpjson.WriteDetailError(w, status, code, message)
}
