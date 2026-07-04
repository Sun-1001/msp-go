package admininboxhttp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	admininboxapp "mathstudy/backend-go/internal/application/admininbox"
	authapp "mathstudy/backend-go/internal/application/auth"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/httpquery"
	"mathstudy/backend-go/internal/platform/redact"
)

// Service is the admin inbox application surface used by HTTP handlers.
type Service interface {
	ListRequests(context.Context, admininboxapp.ListFilter) (admininboxapp.ListResponse, error)
	PendingCount(context.Context) (int, error)
	ReviewRequest(context.Context, string, string, string, *string) (admininboxapp.ReviewResponse, error)
}

// Authenticator decodes Go/Python-compatible access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves /admin/inbox endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates an admin inbox HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("admin inbox service is nil")
	}
	if auth == nil {
		return nil, errors.New("admin inbox authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches admin inbox routes under prefix, for example /api/v1/admin/inbox.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix+"/pending-count", h.pendingCount)
	mux.HandleFunc("GET "+prefix, h.listRequests)
	mux.HandleFunc("POST "+prefix+"/{request_id}/review", h.reviewRequest)
}

type reviewRequestBody struct {
	Action       string  `json:"action"`
	RejectReason *string `json:"reject_reason"`
}

type pendingCountResponse struct {
	PendingCount int `json:"pending_count"`
}

type errorResponse struct {
	Detail  string `json:"detail"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (h *Handler) listRequests(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	filter, ok := parseListFilter(w, r)
	if !ok {
		return
	}
	response, err := h.service.ListRequests(r.Context(), filter)
	if err != nil {
		h.writeServiceError(w, err, "获取密码重置申请列表失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) pendingCount(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	count, err := h.service.PendingCount(r.Context())
	if err != nil {
		h.writeServiceError(w, err, "获取待处理申请数量失败")
		return
	}
	writeJSON(w, http.StatusOK, pendingCountResponse{PendingCount: count})
}

func (h *Handler) reviewRequest(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	var request reviewRequestBody
	if !decodeRequest(w, r, &request) {
		return
	}
	action := strings.ToLower(strings.TrimSpace(request.Action))
	if action != "approve" && action != "reject" {
		writeAdminInboxError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "action 必须是 approve 或 reject")
		return
	}
	response, err := h.service.ReviewRequest(r.Context(), r.PathValue("request_id"), principal.UserID, action, request.RejectReason)
	if err != nil {
		h.writeServiceError(w, err, "审批密码重置申请失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	token, ok := httpauth.BearerToken(r)
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeAdminInboxError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	principal, ok := h.auth.DecodeAccessToken(token)
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeAdminInboxError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	if !authapp.IsAdmin(principal) {
		writeAdminInboxError(w, http.StatusForbidden, "FORBIDDEN", "权限不足，需要管理员权限")
		return authapp.Principal{}, false
	}
	return principal, true
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, admininboxapp.ErrBadRequest):
		writeAdminInboxError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", redact.String(err.Error()))
	default:
		h.logger.Error("admin inbox request failed", "error", redact.String(err.Error()))
		writeAdminInboxError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fallback)
	}
}

func parseListFilter(w http.ResponseWriter, r *http.Request) (admininboxapp.ListFilter, bool) {
	query := r.URL.Query()
	pagination, err := httpquery.Pagination(query, 20, 100)
	if err != nil {
		writeAdminInboxPaginationError(w, err)
		return admininboxapp.ListFilter{}, false
	}
	return admininboxapp.ListFilter{
		Status:   query.Get("status"),
		Page:     pagination.Page,
		PageSize: pagination.PageSize,
	}, true
}

func writeAdminInboxPaginationError(w http.ResponseWriter, err error) {
	writeAdminInboxError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", paginationErrorMessage(err))
}

func paginationErrorMessage(err error) string {
	var paginationErr httpquery.PaginationError
	if !errors.As(err, &paginationErr) {
		return "分页参数无效"
	}
	if errors.Is(err, httpquery.ErrInvalidInt) {
		return paginationErr.Field + " 必须是整数"
	}
	if paginationErr.Field == httpquery.PageField {
		return "page 必须大于等于 1"
	}
	return "page_size 必须在 1 到 100 之间"
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := httpjson.DecodeStrict(w, r, 1<<20, target); err != nil {
		writeAdminInboxError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "请求体格式错误")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeAdminInboxError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Detail: message, Code: code, Message: message})
}
