package exercisehttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	authapp "mathstudy/backend-go/internal/application/auth"
	exerciseapp "mathstudy/backend-go/internal/application/exercise"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/redact"
)

// Service is the exercise application surface used by HTTP handlers.
type Service interface {
	GetNextExercise(context.Context, string, exerciseapp.NextQuery) (*exerciseapp.ExerciseResponse, error)
	SubmitAnswer(context.Context, string, exerciseapp.SubmitRequest) (exerciseapp.SubmitResponse, error)
	GetExercise(context.Context, string, string) (exerciseapp.ExerciseDetailResponse, error)
	GetSolution(context.Context, string, string) (exerciseapp.SolutionResponse, error)
}

// Authenticator decodes Go/Python-compatible access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves /exercise endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates an exercise HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("exercise service is nil")
	}
	if auth == nil {
		return nil, errors.New("exercise authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches exercise routes under prefix, for example /api/v1/exercise.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix+"/next", h.next)
	mux.HandleFunc("POST "+prefix+"/submit", h.submit)
	mux.HandleFunc("GET "+prefix+"/{exercise_id}/solution", h.solution)
	mux.HandleFunc("GET "+prefix+"/{exercise_id}", h.detail)
}

type submitRequest struct {
	ExerciseID       string   `json:"exercise_id"`
	AnswerText       *string  `json:"answer_text"`
	AnswerImageURL   *string  `json:"answer_image_url"`
	AnswerSteps      []string `json:"answer_steps"`
	TimeSpentSeconds int      `json:"time_spent_seconds"`
}

type errorResponse struct {
	Detail  string `json:"detail"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (h *Handler) next(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	query, ok := parseNextQuery(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetNextExercise(r.Context(), principal.UserID, query)
	if err != nil {
		h.writeExerciseError(w, err, "获取练习题失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	var request submitRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	answerText := valueOrEmpty(request.AnswerText)
	answerImageURL := valueOrEmpty(request.AnswerImageURL)
	if strings.TrimSpace(answerText) == "" && strings.TrimSpace(answerImageURL) == "" {
		writeExerciseError(w, http.StatusBadRequest, "BAD_REQUEST", "请提供文本答案或图片答案")
		return
	}
	response, err := h.service.SubmitAnswer(r.Context(), principal.UserID, exerciseapp.SubmitRequest{
		ExerciseID:       request.ExerciseID,
		AnswerText:       answerText,
		AnswerImageURL:   answerImageURL,
		AnswerSteps:      request.AnswerSteps,
		TimeSpentSeconds: request.TimeSpentSeconds,
	})
	if err != nil {
		if errors.Is(err, exerciseapp.ErrBadRequest) {
			writeExerciseError(w, http.StatusBadRequest, "BAD_REQUEST", "提交失败，请检查输入后重试")
			return
		}
		h.writeExerciseError(w, err, "提交答案失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) detail(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetExercise(r.Context(), principal.UserID, r.PathValue("exercise_id"))
	if err != nil {
		if errors.Is(err, exerciseapp.ErrNotFound) {
			writeExerciseError(w, http.StatusNotFound, "NOT_FOUND", "题目不存在或无权访问")
			return
		}
		h.writeExerciseError(w, err, "获取题目详情失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) solution(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetSolution(r.Context(), principal.UserID, r.PathValue("exercise_id"))
	if err != nil {
		if errors.Is(err, exerciseapp.ErrNotFound) {
			writeExerciseError(w, http.StatusNotFound, "NOT_FOUND", "题目不存在或无权访问")
			return
		}
		h.writeExerciseError(w, err, "获取题目解析失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) requirePrincipal(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	token, ok := httpauth.BearerToken(r)
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeExerciseError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	principal, ok := h.auth.DecodeAccessToken(token)
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeExerciseError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	return principal, true
}

func (h *Handler) writeExerciseError(w http.ResponseWriter, err error, fallback string) {
	if errors.Is(err, exerciseapp.ErrForbidden) {
		writeExerciseError(w, http.StatusForbidden, "FORBIDDEN", "请先加入班级后再开始练习")
		return
	}
	if errors.Is(err, exerciseapp.ErrBadRequest) {
		writeExerciseError(w, http.StatusBadRequest, "BAD_REQUEST", fallback)
		return
	}
	h.logger.Error("exercise request failed", "error", redact.String(err.Error()))
	writeExerciseError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fallback)
}

func parseNextQuery(w http.ResponseWriter, r *http.Request) (exerciseapp.NextQuery, bool) {
	query := r.URL.Query()
	var difficulty *float64
	if raw := query.Get("difficulty"); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || parsed < 0 || parsed > 1 {
			writeExerciseError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "difficulty 必须在 0 到 1 之间")
			return exerciseapp.NextQuery{}, false
		}
		difficulty = &parsed
	}
	return exerciseapp.NextQuery{ConceptID: query.Get("concept_id"), Difficulty: difficulty}, true
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := httpjson.DecodeStrict(w, r, 1<<20, target); err != nil {
		writeExerciseError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "请求体格式错误")
		return false
	}
	return true
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func writeExerciseError(w http.ResponseWriter, status int, code, message string) {
	httpjson.Write(w, status, errorResponse{Detail: message, Code: code, Message: message})
}
