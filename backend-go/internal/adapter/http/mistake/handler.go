package mistakehttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	authapp "mathstudy/backend-go/internal/application/auth"
	mistakeapp "mathstudy/backend-go/internal/application/mistake"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/httpquery"
	"mathstudy/backend-go/internal/platform/redact"
)

// Service is the mistake application surface used by HTTP handlers.
type Service interface {
	GetMistakes(context.Context, string, mistakeapp.ListQuery) (mistakeapp.MistakeListResponse, error)
	GetStatistics(context.Context, string, string) (mistakeapp.StatisticsResponse, error)
	GetMistakeDetail(context.Context, string, string) (mistakeapp.DetailResponse, error)
	MarkAsMastered(context.Context, string, string) (mistakeapp.MarkAsMasteredResponse, error)
	DeleteMistake(context.Context, string, string) (mistakeapp.DeleteResponse, error)
	GetReviewExercise(context.Context, string, string, string) (mistakeapp.ReviewExerciseResponse, error)
}

// Authenticator decodes Go/Python-compatible access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves /mistakes endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates a mistake HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("mistake service is nil")
	}
	if auth == nil {
		return nil, errors.New("mistake authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches mistake routes under prefix, for example /api/v1/mistakes.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix, h.list)
	mux.HandleFunc("GET "+prefix+"/statistics", h.statistics)
	mux.HandleFunc("GET "+prefix+"/review/next", h.reviewNext)
	mux.HandleFunc("GET "+prefix+"/{attempt_id}", h.detail)
	mux.HandleFunc("POST "+prefix+"/{attempt_id}/master", h.markAsMastered)
	mux.HandleFunc("DELETE "+prefix+"/{attempt_id}", h.delete)
}

type errorResponse struct {
	Detail  string `json:"detail"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	query, ok := parseListQuery(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetMistakes(r.Context(), principal.UserID, query)
	if err != nil {
		h.logMistakeError("get mistake list failed", err)
		writeMistakeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "查询错题列表失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) statistics(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	timeRange := r.URL.Query().Get("time_range")
	if timeRange == "" {
		timeRange = "month"
	}
	response, err := h.service.GetStatistics(r.Context(), principal.UserID, timeRange)
	if err != nil {
		h.logMistakeError("get mistake statistics failed", err)
		writeMistakeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "查询错题统计失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) detail(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetMistakeDetail(r.Context(), principal.UserID, r.PathValue("attempt_id"))
	if err != nil {
		if errors.Is(err, mistakeapp.ErrNotFound) {
			writeMistakeError(w, http.StatusNotFound, "NOT_FOUND", "错题记录不存在")
			return
		}
		h.logMistakeError("get mistake detail failed", err)
		writeMistakeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "查询错题详情失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) markAsMastered(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	response, err := h.service.MarkAsMastered(r.Context(), principal.UserID, r.PathValue("attempt_id"))
	if err != nil {
		if errors.Is(err, mistakeapp.ErrNotFound) {
			writeMistakeError(w, http.StatusNotFound, "NOT_FOUND", "错题记录不存在")
			return
		}
		if errors.Is(err, mistakeapp.ErrProfileNotFound) {
			writeMistakeError(w, http.StatusNotFound, "NOT_FOUND", "学生画像不存在")
			return
		}
		h.logMistakeError("mark mistake as mastered failed", err)
		writeMistakeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "标记已掌握失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	response, err := h.service.DeleteMistake(r.Context(), principal.UserID, r.PathValue("attempt_id"))
	if err != nil {
		if errors.Is(err, mistakeapp.ErrNotFound) {
			writeMistakeError(w, http.StatusNotFound, "NOT_FOUND", "错题记录不存在")
			return
		}
		h.logMistakeError("delete mistake failed", err)
		writeMistakeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "删除错题失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) reviewNext(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	response, err := h.service.GetReviewExercise(r.Context(), principal.UserID, query.Get("focus_concept"), query.Get("focus_error_type"))
	if err != nil {
		if errors.Is(err, mistakeapp.ErrNotFound) {
			writeMistakeError(w, http.StatusNotFound, "NOT_FOUND", "没有可复习的错题")
			return
		}
		h.logMistakeError("get review exercise failed", err)
		writeMistakeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取复习题目失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) requirePrincipal(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	token, ok := httpauth.BearerToken(r)
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeMistakeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	principal, ok := h.auth.DecodeAccessToken(token)
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeMistakeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	return principal, true
}

func (h *Handler) logMistakeError(message string, err error) {
	h.logger.Error(message, "error", redact.String(err.Error()))
}

func parseListQuery(w http.ResponseWriter, r *http.Request) (mistakeapp.ListQuery, bool) {
	query := r.URL.Query()
	pagination, err := httpquery.Pagination(query, 20, 100)
	if err != nil {
		writeMistakePaginationError(w, err)
		return mistakeapp.ListQuery{}, false
	}
	difficultyMin, ok := parseFloatQuery(w, query.Get("difficulty_min"), 0.0, "difficulty_min")
	if !ok {
		return mistakeapp.ListQuery{}, false
	}
	difficultyMax, ok := parseFloatQuery(w, query.Get("difficulty_max"), 1.0, "difficulty_max")
	if !ok {
		return mistakeapp.ListQuery{}, false
	}
	dateFrom, ok := parseOptionalTimeQuery(w, query.Get("date_from"), "开始时间格式错误，请使用 ISO 8601 格式")
	if !ok {
		return mistakeapp.ListQuery{}, false
	}
	dateTo, ok := parseOptionalTimeQuery(w, query.Get("date_to"), "结束时间格式错误，请使用 ISO 8601 格式")
	if !ok {
		return mistakeapp.ListQuery{}, false
	}
	if difficultyMin < 0 || difficultyMin > 1 || difficultyMax < 0 || difficultyMax > 1 {
		writeMistakeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "difficulty 必须在 0 到 1 之间")
		return mistakeapp.ListQuery{}, false
	}
	masteryStatus := query.Get("mastery_status")
	if masteryStatus == "" {
		masteryStatus = "all"
	}
	sortBy := query.Get("sort_by")
	if sortBy == "" {
		sortBy = "time"
	}
	sortOrder := query.Get("sort_order")
	if sortOrder == "" {
		sortOrder = "desc"
	}
	return mistakeapp.ListQuery{
		Page:          pagination.Page,
		PageSize:      pagination.PageSize,
		ErrorType:     query.Get("error_type"),
		ConceptID:     query.Get("concept_id"),
		DifficultyMin: difficultyMin,
		DifficultyMax: difficultyMax,
		DateFrom:      dateFrom,
		DateTo:        dateTo,
		MasteryStatus: masteryStatus,
		SortBy:        sortBy,
		SortOrder:     sortOrder,
	}, true
}

func writeMistakePaginationError(w http.ResponseWriter, err error) {
	writeMistakeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", paginationErrorMessage(err))
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

func parseFloatQuery(w http.ResponseWriter, value string, fallback float64, name string) (float64, bool) {
	parsed, err := httpquery.Float(value, fallback)
	if err != nil {
		writeMistakeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", name+" 必须是数字")
		return 0, false
	}
	return parsed, true
}

func parseOptionalTimeQuery(w http.ResponseWriter, value string, message string) (*time.Time, bool) {
	parsed, err := httpquery.OptionalTime(
		value,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05",
		"2006-01-02",
	)
	if err != nil {
		writeMistakeError(w, http.StatusBadRequest, "BAD_REQUEST", message)
		return nil, false
	}
	return parsed, true
}

func writeMistakeError(w http.ResponseWriter, status int, code, message string) {
	httpjson.Write(w, status, errorResponse{Detail: message, Code: code, Message: message})
}
