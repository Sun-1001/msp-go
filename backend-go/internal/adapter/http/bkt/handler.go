package bkthttp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	authapp "mathstudy/backend-go/internal/application/auth"
	bktapp "mathstudy/backend-go/internal/application/bkt"
)

// Service is the admin BKT application surface used by HTTP handlers.
type Service interface {
	ListParams(context.Context, int, int) (bktapp.ListResponse, error)
	UpdateParam(context.Context, string, bktapp.Update) (bktapp.Param, error)
	ResetParam(context.Context, string) (bktapp.Param, error)
	SeedDefaultParams(context.Context) (bktapp.SeedResponse, error)
}

// Authenticator decodes Go/Python-compatible access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves /admin/bkt endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates an admin BKT HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("bkt service is nil")
	}
	if auth == nil {
		return nil, errors.New("bkt authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches BKT routes under prefix, for example /api/v1/admin/bkt.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix+"/params", h.listParams)
	mux.HandleFunc("PUT "+prefix+"/params/{concept_id}", h.updateParam)
	mux.HandleFunc("POST "+prefix+"/params/reset/{concept_id}", h.resetParam)
	mux.HandleFunc("POST "+prefix+"/seed", h.seedDefaultParams)
}

type updateRequest struct {
	PL0 *float64 `json:"p_l0"`
	PT  *float64 `json:"p_t"`
	PG  *float64 `json:"p_g"`
	PS  *float64 `json:"p_s"`
}

type errorResponse struct {
	Detail  string `json:"detail"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (h *Handler) listParams(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	offset, ok := parseIntQuery(w, r.URL.Query().Get("offset"), 0, "offset")
	if !ok {
		return
	}
	limit, ok := parseIntQuery(w, r.URL.Query().Get("limit"), 50, "limit")
	if !ok {
		return
	}
	response, err := h.service.ListParams(r.Context(), offset, limit)
	if err != nil {
		h.writeServiceError(w, err, "获取 BKT 参数列表失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) updateParam(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var request updateRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	update, ok := request.toUpdate(w)
	if !ok {
		return
	}
	response, err := h.service.UpdateParam(r.Context(), r.PathValue("concept_id"), update)
	if err != nil {
		h.writeServiceError(w, err, "更新 BKT 参数失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) resetParam(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.ResetParam(r.Context(), r.PathValue("concept_id"))
	if err != nil {
		h.writeServiceError(w, err, "重置 BKT 参数失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) seedDefaultParams(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.SeedDefaultParams(r.Context())
	if err != nil {
		h.writeServiceError(w, err, "种子化 BKT 参数失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	fields := strings.Fields(authHeader)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeBKTError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	principal, ok := h.auth.DecodeAccessToken(fields[1])
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeBKTError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	if !authapp.IsAdmin(principal) {
		writeBKTError(w, http.StatusForbidden, "FORBIDDEN", "权限不足，需要管理员权限")
		return authapp.Principal{}, false
	}
	return principal, true
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, bktapp.ErrBadRequest):
		writeBKTError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
	case errors.Is(err, bktapp.ErrNotFound):
		writeBKTError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	default:
		h.logger.Error("admin bkt request failed", "error", err)
		writeBKTError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fallback)
	}
}

func parseIntQuery(w http.ResponseWriter, value string, fallback int, name string) (int, bool) {
	if value == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		writeBKTError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", name+" 必须是整数")
		return 0, false
	}
	return parsed, true
}

func (r updateRequest) toUpdate(w http.ResponseWriter) (bktapp.Update, bool) {
	update := bktapp.Update{PL0: r.PL0, PT: r.PT, PG: r.PG, PS: r.PS}
	if !validateProbability(w, update.PL0, 0, 1, "p_l0") ||
		!validateProbability(w, update.PT, 0, 1, "p_t") ||
		!validateProbability(w, update.PG, 0, 0.5, "p_g") ||
		!validateProbability(w, update.PS, 0, 0.5, "p_s") {
		return bktapp.Update{}, false
	}
	return update, true
}

func validateProbability(w http.ResponseWriter, value *float64, min float64, max float64, name string) bool {
	if value == nil || (*value >= min && *value <= max) {
		return true
	}
	writeBKTError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", name+" 必须在 "+formatProbability(min)+" 到 "+formatProbability(max)+" 之间")
	return false
}

func formatProbability(value float64) string {
	switch value {
	case 0:
		return "0"
	case 0.5:
		return "0.5"
	case 1:
		return "1"
	default:
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	if err := decoder.Decode(target); err != nil {
		writeBKTError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "请求体格式错误")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeBKTError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Detail: message, Code: code, Message: message})
}
