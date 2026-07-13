package securityloghttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	authapp "mathstudy/backend-go/internal/application/auth"
	securitylogapp "mathstudy/backend-go/internal/application/securitylog"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/httpquery"
	"mathstudy/backend-go/internal/platform/redact"
)

// Service is the security log application surface used by HTTP handlers.
type Service interface {
	ListLogs(context.Context, securitylogapp.QueryFilter) (securitylogapp.ListResponse, error)
	Stats(context.Context) (securitylogapp.StatsResponse, error)
	DeleteLogs(context.Context, securitylogapp.DeleteRequest) (map[string]int, error)
	ExportLogs(context.Context, securitylogapp.ExportRequest) (securitylogapp.ExportResponse, error)
	ArchiveLogs(context.Context, time.Time) (securitylogapp.ArchiveResponse, error)
	GenerateDailyReport(context.Context) (securitylogapp.DailyReportResponse, error)
	Cleanup(context.Context) (securitylogapp.CleanupResponse, error)
	Volume(context.Context) (securitylogapp.VolumeResponse, error)
}

// Authenticator decodes Go/Python-compatible access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves /admin/security-logs endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates a security log HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("security log service is nil")
	}
	if auth == nil {
		return nil, errors.New("security log authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches security log routes under prefix, for example /api/v1/admin/security-logs.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix+"/stats", h.stats)
	mux.HandleFunc("POST "+prefix+"/export", h.exportLogs)
	mux.HandleFunc("POST "+prefix+"/archive", h.archiveLogs)
	mux.HandleFunc("POST "+prefix+"/generate-daily-report", h.generateDailyReport)
	mux.HandleFunc("POST "+prefix+"/cleanup", h.cleanup)
	mux.HandleFunc("GET "+prefix+"/volume", h.volume)
	mux.HandleFunc("GET "+prefix, h.listLogs)
	mux.HandleFunc("DELETE "+prefix, h.deleteLogs)
}

type archiveRequest struct {
	BeforeDate time.Time `json:"before_date"`
}

func (h *Handler) listLogs(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	filter, ok := parseQueryFilter(w, r)
	if !ok {
		return
	}
	response, err := h.service.ListLogs(r.Context(), filter)
	if err != nil {
		h.writeServiceError(w, err, "获取安全日志列表失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.Stats(r.Context())
	if err != nil {
		h.writeServiceError(w, err, "获取安全日志统计失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) deleteLogs(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var request securitylogapp.DeleteRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	response, err := h.service.DeleteLogs(r.Context(), request)
	if err != nil {
		h.writeServiceError(w, err, "删除安全日志失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) exportLogs(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var request securitylogapp.ExportRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	response, err := h.service.ExportLogs(r.Context(), request)
	if err != nil {
		h.writeServiceError(w, err, "导出安全日志失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) archiveLogs(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var request archiveRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	response, err := h.service.ArchiveLogs(r.Context(), request.BeforeDate)
	if err != nil {
		h.writeServiceError(w, err, "归档安全日志失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) generateDailyReport(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.GenerateDailyReport(r.Context())
	if err != nil {
		h.writeServiceError(w, err, "生成每日安全报告失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) cleanup(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.Cleanup(r.Context())
	if err != nil {
		h.writeServiceError(w, err, "清理安全日志失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) volume(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.Volume(r.Context())
	if err != nil {
		h.writeServiceError(w, err, "获取日志总量失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccess(
		w, r, h.auth.DecodeAccessToken, authapp.IsAdmin,
		"权限不足，需要管理员权限", writeSecurityLogError,
	)
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, securitylogapp.ErrBadRequest):
		writeSecurityLogError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", redact.String(err.Error()))
	default:
		h.logger.Error("security log request failed", "error", redact.String(err.Error()))
		writeSecurityLogError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fallback)
	}
}

func parseQueryFilter(w http.ResponseWriter, r *http.Request) (securitylogapp.QueryFilter, bool) {
	query := r.URL.Query()
	pagination, err := httpquery.Pagination(query, 50, 100)
	if err != nil {
		writeSecurityLogPaginationError(w, err)
		return securitylogapp.QueryFilter{}, false
	}
	startDate, ok := parseTimeQuery(w, query.Get("start_date"), "start_date")
	if !ok {
		return securitylogapp.QueryFilter{}, false
	}
	endDate, ok := parseTimeQuery(w, query.Get("end_date"), "end_date")
	if !ok {
		return securitylogapp.QueryFilter{}, false
	}
	return securitylogapp.QueryFilter{
		EventTypes:      parseEventTypes(httpquery.StringList(query["event_types"])),
		Severities:      parseSeverities(httpquery.StringList(query["severities"])),
		StartDate:       startDate,
		EndDate:         endDate,
		IncludeArchived: strings.EqualFold(query.Get("include_archived"), "true"),
		Page:            pagination.Page,
		PageSize:        pagination.PageSize,
	}, true
}

func writeSecurityLogPaginationError(w http.ResponseWriter, err error) {
	writeSecurityLogError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", httpquery.PaginationErrorMessage(err, 100))
}

func parseTimeQuery(w http.ResponseWriter, value string, name string) (*time.Time, bool) {
	parsed, err := httpquery.OptionalTime(value, time.RFC3339, "2006-01-02")
	if err != nil {
		writeSecurityLogError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", name+" 时间格式错误")
		return nil, false
	}
	return parsed, true
}

func parseEventTypes(values []string) []securitylogapp.EventType {
	result := make([]securitylogapp.EventType, 0, len(values))
	for _, value := range values {
		result = append(result, securitylogapp.EventType(value))
	}
	return result
}

func parseSeverities(values []string) []securitylogapp.Severity {
	result := make([]securitylogapp.Severity, 0, len(values))
	for _, value := range values {
		result = append(result, securitylogapp.Severity(value))
	}
	return result
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	return httpjson.DecodeStrictOrDetailError(w, r, 1<<20, target)
}

func writeSecurityLogError(w http.ResponseWriter, status int, code, message string) {
	httpjson.WriteDetailError(w, status, code, message)
}
