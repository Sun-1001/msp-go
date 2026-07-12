package exercisehttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	authapp "mathstudy/backend-go/internal/application/auth"
	exerciseapp "mathstudy/backend-go/internal/application/exercise"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/httpquery"
	"mathstudy/backend-go/internal/platform/ptrutil"
	"mathstudy/backend-go/internal/platform/redact"
)

// Service is the exercise application surface used by HTTP handlers.
type Service interface {
	GetNextExercise(context.Context, string, exerciseapp.NextQuery) (*exerciseapp.ExerciseResponse, error)
	GenerateExercise(context.Context, string, exerciseapp.GenerateExerciseRequest) (*exerciseapp.ExerciseResponse, error)
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
	limiter *generationRateLimiter
}

const (
	generationRateLimitMax    = 10
	generationRateLimitWindow = time.Minute
)

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
	return &Handler{
		service: service,
		auth:    auth,
		logger:  logger,
		limiter: newGenerationRateLimiter(generationRateLimitMax, generationRateLimitWindow),
	}, nil
}

// Register attaches exercise routes under prefix, for example /api/v1/exercise.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix+"/next", h.next)
	mux.HandleFunc("POST "+prefix+"/generate", h.generate)
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

type generateRequest struct {
	ConceptID  string   `json:"concept_id"`
	Difficulty *float64 `json:"difficulty"`
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

func (h *Handler) generate(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireStudent(w, r)
	if !ok {
		return
	}
	var request generateRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	request.ConceptID = strings.TrimSpace(request.ConceptID)
	if request.ConceptID == "" || len(request.ConceptID) > 36 {
		writeExerciseError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "请选择有效的知识点")
		return
	}
	if request.Difficulty == nil || *request.Difficulty < 0 || *request.Difficulty > 1 {
		writeExerciseError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "difficulty 必须在 0 到 1 之间")
		return
	}
	if h.limiter != nil && !h.limiter.Allow(principal.UserID) {
		writeExerciseError(w, http.StatusTooManyRequests, "RATE_LIMITED", "AI 出题过于频繁，请稍后重试")
		return
	}
	response, err := h.service.GenerateExercise(r.Context(), principal.UserID, exerciseapp.GenerateExerciseRequest{
		ConceptID:  request.ConceptID,
		Difficulty: *request.Difficulty,
	})
	if err != nil {
		switch {
		case errors.Is(err, exerciseapp.ErrNotFound):
			writeExerciseError(w, http.StatusNotFound, "NOT_FOUND", "所选知识点不存在")
		case errors.Is(err, exerciseapp.ErrBadRequest):
			writeExerciseError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "生成参数无效")
		default:
			h.writeExerciseError(w, err, "生成练习题失败")
		}
		return
	}
	httpjson.Write(w, http.StatusCreated, response)
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
	answerText := ptrutil.ValueOrZero(request.AnswerText)
	answerImageURL := ptrutil.ValueOrZero(request.AnswerImageURL)
	if strings.TrimSpace(answerText) == "" && strings.TrimSpace(answerImageURL) == "" {
		writeExerciseError(w, http.StatusBadRequest, "BAD_REQUEST", "请提供文本答案")
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

func (h *Handler) requireStudent(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return authapp.Principal{}, false
	}
	if !authapp.IsStudent(principal) {
		writeExerciseError(w, http.StatusForbidden, "FORBIDDEN", "权限不足，需要学生身份")
		return authapp.Principal{}, false
	}
	return principal, true
}

func (h *Handler) writeExerciseError(w http.ResponseWriter, err error, fallback string) {
	if errors.Is(err, exerciseapp.ErrAIGenerationUnavailable) {
		writeExerciseError(w, http.StatusServiceUnavailable, "AI_GENERATION_UNAVAILABLE", "AI 出题服务暂不可用，请稍后重试")
		return
	}
	if errors.Is(err, exerciseapp.ErrOCRUnavailable) {
		writeExerciseError(w, http.StatusNotImplemented, "OCR_UNAVAILABLE", "图片答案自动判题尚未开放，请改用文本答案")
		return
	}
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
		parsed, err := httpquery.BoundedFloat(raw, 0, 0, 1)
		if err != nil {
			writeExerciseError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "difficulty 必须在 0 到 1 之间")
			return exerciseapp.NextQuery{}, false
		}
		difficulty = &parsed
	}
	return exerciseapp.NextQuery{ConceptID: query.Get("concept_id"), Difficulty: difficulty}, true
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	return httpjson.DecodeStrictOrDetailError(w, r, 1<<20, target)
}

func writeExerciseError(w http.ResponseWriter, status int, code, message string) {
	httpjson.WriteDetailError(w, status, code, message)
}

type generationRateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	now    func() time.Time
	hits   map[string][]time.Time
}

func newGenerationRateLimiter(limit int, window time.Duration) *generationRateLimiter {
	if limit <= 0 {
		limit = generationRateLimitMax
	}
	if window <= 0 {
		window = generationRateLimitWindow
	}
	return &generationRateLimiter{
		limit:  limit,
		window: window,
		now:    func() time.Time { return time.Now().UTC() },
		hits:   map[string][]time.Time{},
	}
}

func (l *generationRateLimiter) Allow(key string) bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.limit <= 0 {
		l.limit = generationRateLimitMax
	}
	if l.window <= 0 {
		l.window = generationRateLimitWindow
	}
	if l.now == nil {
		l.now = func() time.Time { return time.Now().UTC() }
	}
	if l.hits == nil {
		l.hits = map[string][]time.Time{}
	}
	now := l.now()
	cutoff := now.Add(-l.window)
	recent := l.hits[key][:0]
	for _, hit := range l.hits[key] {
		if hit.After(cutoff) {
			recent = append(recent, hit)
		}
	}
	if len(recent) >= l.limit {
		l.hits[key] = recent
		return false
	}
	l.hits[key] = append(recent, now)
	return true
}
