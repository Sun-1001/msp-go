package adminstatshttp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	adminstatsapp "mathstudy/backend-go/internal/application/adminstats"
	authapp "mathstudy/backend-go/internal/application/auth"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpquery"
	"mathstudy/backend-go/internal/platform/redact"
)

// Service is the admin stats application surface used by HTTP handlers.
type Service interface {
	OverviewStats(context.Context) (adminstatsapp.OverviewStatsResponse, error)
	UserGrowth(context.Context, string) (adminstatsapp.UserGrowthResponse, error)
	RecentActivities(context.Context, int) (adminstatsapp.RecentActivitiesResponse, error)
	SystemStatus(context.Context) (adminstatsapp.SystemStatusResponse, error)
}

// Authenticator decodes Go/Python-compatible access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves /admin/stats endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates an admin stats HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("admin stats service is nil")
	}
	if auth == nil {
		return nil, errors.New("admin stats authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches admin stats routes under prefix, for example /api/v1/admin/stats.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix+"/overview", h.overview)
	mux.HandleFunc("GET "+prefix+"/user-growth", h.userGrowth)
	mux.HandleFunc("GET "+prefix+"/recent-activities", h.recentActivities)
	mux.HandleFunc("GET "+prefix+"/system-status", h.systemStatus)
}

type errorResponse struct {
	Detail  string `json:"detail"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.OverviewStats(r.Context())
	if err != nil {
		h.writeServiceError(w, err, "获取概览统计失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) userGrowth(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.UserGrowth(r.Context(), r.URL.Query().Get("period"))
	if err != nil {
		h.writeServiceError(w, err, "获取用户增长趋势失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) recentActivities(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	limit, ok := parseIntQuery(w, r.URL.Query().Get("limit"), 10, "limit")
	if !ok {
		return
	}
	if limit < 1 || limit > 50 {
		writeAdminStatsError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "limit 必须在 1 到 50 之间")
		return
	}
	response, err := h.service.RecentActivities(r.Context(), limit)
	if err != nil {
		h.writeServiceError(w, err, "获取最近活动失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) systemStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.SystemStatus(r.Context())
	if err != nil {
		h.writeServiceError(w, err, "获取系统状态失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	token, ok := httpauth.BearerToken(r)
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeAdminStatsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	principal, ok := h.auth.DecodeAccessToken(token)
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeAdminStatsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	if !authapp.IsAdmin(principal) {
		writeAdminStatsError(w, http.StatusForbidden, "FORBIDDEN", "权限不足，需要管理员权限")
		return authapp.Principal{}, false
	}
	return principal, true
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, adminstatsapp.ErrBadRequest):
		writeAdminStatsError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", redact.String(err.Error()))
	default:
		h.logger.Error("admin stats request failed", "error", redact.String(err.Error()))
		writeAdminStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fallback)
	}
}

func parseIntQuery(w http.ResponseWriter, value string, fallback int, name string) (int, bool) {
	parsed, err := httpquery.Int(value, fallback)
	if err != nil {
		writeAdminStatsError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", name+" 必须是整数")
		return 0, false
	}
	return parsed, true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeAdminStatsError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Detail: message, Code: code, Message: message})
}
