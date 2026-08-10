package adminstoragehttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	adminstorageapp "mathstudy/backend/internal/application/adminstorage"
	authapp "mathstudy/backend/internal/application/auth"
	"mathstudy/backend/internal/platform/httpauth"
	"mathstudy/backend/internal/platform/httpjson"
	"mathstudy/backend/internal/platform/redact"
)

// Service is the administrator storage application surface used by HTTP handlers.
type Service interface {
	Settings(context.Context) (adminstorageapp.SettingsResponse, error)
	UpdateSettings(context.Context, adminstorageapp.UpdateInput) (adminstorageapp.SettingsResponse, error)
	TestConnection(context.Context, adminstorageapp.UpdateInput) (adminstorageapp.TestResponse, error)
}

// Authenticator validates access tokens against current account state.
type Authenticator interface {
	DecodeActiveAccessToken(context.Context, string) (authapp.Principal, bool, error)
}

// Handler serves administrator object-storage settings endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates an administrator storage settings handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("admin storage service is nil")
	}
	if auth == nil {
		return nil, errors.New("admin storage authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches routes below /api/v1/admin/settings.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix+"/storage", h.settings)
	mux.HandleFunc("PUT "+prefix+"/storage", h.updateSettings)
	mux.HandleFunc("POST "+prefix+"/storage/test", h.testConnection)
}

func (h *Handler) settings(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.Settings(r.Context())
	if err != nil {
		h.writeServiceError(w, err, "获取存储配置失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var input adminstorageapp.UpdateInput
	if !decodeRequest(w, r, &input) {
		return
	}
	response, err := h.service.UpdateSettings(r.Context(), input)
	if err != nil {
		h.writeServiceError(w, err, "更新存储配置失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) testConnection(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var input adminstorageapp.UpdateInput
	if !decodeRequest(w, r, &input) {
		return
	}
	response, err := h.service.TestConnection(r.Context(), input)
	if err != nil {
		h.writeServiceError(w, err, "存储连接测试失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccessContext(
		w, r, h.auth.DecodeActiveAccessToken, authapp.IsAdmin,
		"权限不足，需要管理员权限", writeAdminStorageError,
	)
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, adminstorageapp.ErrBadRequest):
		h.logger.Warn("administrator storage configuration rejected", "error", loggableError(err))
		writeAdminStorageError(w, http.StatusBadRequest, "BAD_REQUEST", redact.String(err.Error()))
	case errors.Is(err, adminstorageapp.ErrConnection):
		h.logger.Warn("administrator storage connection failed", "error", loggableError(err))
		writeAdminStorageError(w, http.StatusUnprocessableEntity, "STORAGE_CONNECTION_FAILED", redact.String(err.Error()))
	default:
		h.logger.Error("administrator storage request failed", "error", loggableError(err))
		writeAdminStorageError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fallback)
	}
}

func loggableError(err error) string {
	var appError adminstorageapp.Error
	if errors.As(err, &appError) && appError.Cause != nil {
		return redact.String(appError.Cause.Error())
	}
	return redact.String(err.Error())
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	return httpjson.DecodeStrictOrDetailError(w, r, 1<<20, target)
}

func writeAdminStorageError(w http.ResponseWriter, status int, code string, message string) {
	httpjson.WriteDetailError(w, status, code, message)
}
