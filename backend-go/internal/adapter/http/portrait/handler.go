package portraithttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	authapp "mathstudy/backend-go/internal/application/auth"
	portraitapp "mathstudy/backend-go/internal/application/portrait"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/redact"
)

// Service is the portrait application surface used by HTTP handlers.
type Service interface {
	GetPortrait(context.Context, string) (portraitapp.PortraitResponse, error)
	GeneratePortrait(context.Context, string) (portraitapp.GenerateResponse, error)
	ClearPortrait(context.Context, string) (portraitapp.ClearResponse, error)
}

// Authenticator decodes Go/Python-compatible access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves /portrait endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates a portrait HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("portrait service is nil")
	}
	if auth == nil {
		return nil, errors.New("portrait authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches portrait routes under prefix, for example /api/v1/portrait.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix, h.get)
	mux.HandleFunc("POST "+prefix+"/generate", h.generate)
	mux.HandleFunc("DELETE "+prefix, h.clear)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetPortrait(r.Context(), principal.UserID)
	if err != nil {
		h.logPortraitError("get portrait failed", err)
		writePortraitError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取学生画像失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) generate(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	response, err := h.service.GeneratePortrait(r.Context(), principal.UserID)
	if err != nil {
		h.logPortraitError("generate portrait failed", err)
		writePortraitError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "画像生成失败，请稍后重试")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) clear(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	response, err := h.service.ClearPortrait(r.Context(), principal.UserID)
	if err != nil {
		h.logPortraitError("clear portrait failed", err)
		writePortraitError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "清除学生画像失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) requirePrincipal(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccess(w, r, h.auth.DecodeAccessToken, nil, "", writePortraitError)
}

func (h *Handler) logPortraitError(message string, err error) {
	h.logger.Error(message, "error", redact.String(err.Error()))
}

func writePortraitError(w http.ResponseWriter, status int, code, message string) {
	httpjson.WriteDetailError(w, status, code, message)
}
