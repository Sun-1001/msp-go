package xidianhttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	authapp "mathstudy/backend-go/internal/application/auth"
	xidianapp "mathstudy/backend-go/internal/application/xidian"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/redact"
)

// Service is the Xidian application surface used by HTTP handlers.
type Service interface {
	GetBindingStatus(context.Context, string) (xidianapp.BindingStatus, error)
	StartBinding(context.Context) (xidianapp.BindStartResponse, error)
	CompleteBinding(context.Context, string, xidianapp.CompleteBindingInput) (xidianapp.BindCompleteResponse, error)
	Unbind(context.Context, string) error
}

// Authenticator decodes Go/Python-compatible access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves /xidian endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates a Xidian HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("xidian service is nil")
	}
	if auth == nil {
		return nil, errors.New("xidian authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches Xidian routes under prefix, for example /api/v1/xidian.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix+"/binding", h.bindingStatus)
	mux.HandleFunc("POST "+prefix+"/binding/start", h.startBinding)
	mux.HandleFunc("POST "+prefix+"/binding/complete", h.completeBinding)
	mux.HandleFunc("POST "+prefix+"/binding/unbind", h.unbind)
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const maxJSONBodyBytes = 1 << 20

func (h *Handler) bindingStatus(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetBindingStatus(r.Context(), principal.UserID)
	if err != nil {
		h.writeServiceError(w, err, "获取绑定状态失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) startBinding(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requirePrincipal(w, r); !ok {
		return
	}
	response, err := h.service.StartBinding(r.Context())
	if err != nil {
		h.writeServiceError(w, err, "获取验证码失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) completeBinding(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	var request xidianapp.CompleteBindingInput
	if !decodeRequest(w, r, &request) {
		return
	}
	response, err := h.service.CompleteBinding(r.Context(), principal.UserID, request)
	if err != nil {
		h.writeServiceError(w, err, "绑定失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) unbind(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	if err := h.service.Unbind(r.Context(), principal.UserID); err != nil {
		h.writeServiceError(w, err, "解绑失败")
		return
	}
	httpjson.Write(w, http.StatusOK, xidianapp.UnbindResponse{Success: true})
}

func (h *Handler) requirePrincipal(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccess(w, r, h.auth.DecodeAccessToken, nil, "", writeXidianError)
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error, fallback string) {
	var serviceErr xidianapp.ServiceError
	if errors.As(err, &serviceErr) {
		status := serviceErr.Status
		if status == 0 {
			status = http.StatusBadRequest
		}
		writeXidianError(w, status, redact.String(serviceErr.Code), redact.String(serviceErr.Message))
		return
	}
	h.logger.Error("xidian request failed", "error", redact.String(err.Error()))
	writeXidianError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fallback)
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := httpjson.DecodeStrict(w, r, maxJSONBodyBytes, target); err != nil {
		writeXidianError(w, http.StatusBadRequest, "BAD_REQUEST", "请求体不是有效 JSON")
		return false
	}
	return true
}

func writeXidianError(w http.ResponseWriter, status int, code, message string) {
	httpjson.Write(w, status, errorResponse{Code: code, Message: message})
}
