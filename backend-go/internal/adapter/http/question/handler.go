package questionhttp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	authapp "mathstudy/backend-go/internal/application/auth"
	questionapp "mathstudy/backend-go/internal/application/question"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/httpquery"
	"mathstudy/backend-go/internal/platform/redact"
)

const maxJSONBodyBytes = 2 << 20

// Service is the question application surface used by HTTP handlers.
type Service interface {
	ListQuestions(context.Context, string, questionapp.ListFilter) (questionapp.ListResponse, error)
	GetQuestion(context.Context, string, string) (questionapp.Question, error)
	CreateQuestion(context.Context, string, questionapp.QuestionInput) (questionapp.Question, error)
	UpdateQuestion(context.Context, string, string, questionapp.QuestionUpdate) (questionapp.Question, error)
	DeleteQuestion(context.Context, string, string) error
	GetGroups(context.Context, string) (questionapp.GroupsResponse, error)
	GetStats(context.Context, string) (questionapp.Stats, error)
	BatchPublish(context.Context, string, []string) (questionapp.BatchOperationResponse, error)
	BatchDelete(context.Context, string, []string) (questionapp.BatchOperationResponse, error)
	BatchDuplicate(context.Context, string, []string) (questionapp.BatchOperationResponse, error)
	BatchImport(context.Context, string, []questionapp.QuestionInput) (questionapp.BatchOperationResponse, error)
	ParseQuestions(context.Context, []string) (questionapp.AIParseResponse, error)
	GenerateIsomorphicProblem(context.Context, questionapp.GenerateRequest) (questionapp.GeneratedQuestion, error)
}

// Authenticator decodes Go/Python-compatible access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves /questions endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates a question HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("question service is nil")
	}
	if auth == nil {
		return nil, errors.New("question authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches question routes under prefix, for example /api/v1/questions.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("POST "+prefix, h.create)
	mux.HandleFunc("GET "+prefix, h.list)
	mux.HandleFunc("GET "+prefix+"/groups", h.groups)
	mux.HandleFunc("GET "+prefix+"/stats", h.stats)
	mux.HandleFunc("POST "+prefix+"/batch/publish", h.batchPublish)
	mux.HandleFunc("POST "+prefix+"/batch/delete", h.batchDelete)
	mux.HandleFunc("POST "+prefix+"/batch/duplicate", h.batchDuplicate)
	mux.HandleFunc("POST "+prefix+"/batch/import", h.batchImport)
	mux.HandleFunc("POST "+prefix+"/ai-parse", h.aiParse)
	mux.HandleFunc("POST "+prefix+"/generate-isomorphic", h.generateIsomorphic)
	mux.HandleFunc("GET "+prefix+"/{question_id}", h.detail)
	mux.HandleFunc("PUT "+prefix+"/{question_id}", h.update)
	mux.HandleFunc("DELETE "+prefix+"/{question_id}", h.delete)
}

type createRequest struct {
	Title                string    `json:"title"`
	Body                 string    `json:"body"`
	Type                 string    `json:"type"`
	Difficulty           *float64  `json:"difficulty"`
	ConceptIDs           []string  `json:"concept_ids"`
	Tags                 []string  `json:"tags"`
	Answer               *string   `json:"answer"`
	AnswerType           string    `json:"answer_type"`
	Hints                []string  `json:"hints"`
	SolutionSteps        []string  `json:"solution_steps"`
	Options              *[]string `json:"options"`
	EstimatedTimeSeconds *int      `json:"estimated_time_seconds"`
}

type updateRequest struct {
	Title                *string   `json:"title"`
	Body                 *string   `json:"body"`
	Type                 *string   `json:"type"`
	Difficulty           *float64  `json:"difficulty"`
	ConceptIDs           *[]string `json:"concept_ids"`
	Tags                 *[]string `json:"tags"`
	Answer               *string   `json:"answer"`
	AnswerType           *string   `json:"answer_type"`
	Hints                *[]string `json:"hints"`
	SolutionSteps        *[]string `json:"solution_steps"`
	Options              *[]string `json:"options"`
	EstimatedTimeSeconds *int      `json:"estimated_time_seconds"`
	Status               *string   `json:"status"`
}

type batchRequest struct {
	QuestionIDs []string `json:"question_ids"`
}

type batchImportRequest struct {
	Questions []createRequest `json:"questions"`
}

type aiParseRequest struct {
	RawTexts []string `json:"raw_texts"`
}

type generateRequest struct {
	Template   string   `json:"template"`
	Ability    *float64 `json:"ability"`
	Difficulty *float64 `json:"difficulty"`
	ConceptIDs []string `json:"concept_ids"`
	Tags       []string `json:"tags"`
}

type errorResponse struct {
	Detail  string `json:"detail"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
		return
	}
	filter, ok := parseListFilter(w, r)
	if !ok {
		return
	}
	response, err := h.service.ListQuestions(r.Context(), principal.UserID, filter)
	if err != nil {
		h.logQuestionError("list questions failed", err)
		writeQuestionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取题目列表失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) groups(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetGroups(r.Context(), principal.UserID)
	if err != nil {
		h.logQuestionError("get question groups failed", err)
		writeQuestionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取题目分组失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetStats(r.Context(), principal.UserID)
	if err != nil {
		h.logQuestionError("get question stats failed", err)
		writeQuestionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取题目统计失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) detail(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetQuestion(r.Context(), principal.UserID, r.PathValue("question_id"))
	if err != nil {
		if errors.Is(err, questionapp.ErrBadRequest) {
			writeQuestionError(w, http.StatusBadRequest, "BAD_REQUEST", "该内容不是题目类型")
			return
		}
		if errors.Is(err, questionapp.ErrForbidden) {
			writeQuestionError(w, http.StatusForbidden, "FORBIDDEN", "无权访问此题目")
			return
		}
		if errors.Is(err, questionapp.ErrNotFound) {
			writeQuestionError(w, http.StatusNotFound, "NOT_FOUND", "题目不存在或无权访问")
			return
		}
		h.logQuestionError("get question failed", err)
		writeQuestionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取题目详情失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
		return
	}
	var request createRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	input, ok := request.toInput(w)
	if !ok {
		return
	}
	response, err := h.service.CreateQuestion(r.Context(), principal.UserID, input)
	if err != nil {
		if errors.Is(err, questionapp.ErrBadRequest) {
			writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "题目内容不合法")
			return
		}
		h.logQuestionError("create question failed", err)
		writeQuestionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "创建题目失败")
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
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
	response, err := h.service.UpdateQuestion(r.Context(), principal.UserID, r.PathValue("question_id"), update)
	if err != nil {
		if errors.Is(err, questionapp.ErrBadRequest) {
			writeQuestionError(w, http.StatusBadRequest, "BAD_REQUEST", "更新请求不合法")
			return
		}
		if errors.Is(err, questionapp.ErrForbidden) {
			writeQuestionError(w, http.StatusForbidden, "FORBIDDEN", "无权修改此题目")
			return
		}
		if errors.Is(err, questionapp.ErrNotFound) {
			writeQuestionError(w, http.StatusNotFound, "NOT_FOUND", "题目不存在或无权访问")
			return
		}
		h.logQuestionError("update question failed", err)
		writeQuestionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "更新失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
		return
	}
	err := h.service.DeleteQuestion(r.Context(), principal.UserID, r.PathValue("question_id"))
	if err != nil {
		if errors.Is(err, questionapp.ErrBadRequest) {
			writeQuestionError(w, http.StatusBadRequest, "BAD_REQUEST", "删除题目请求不合法")
			return
		}
		if errors.Is(err, questionapp.ErrForbidden) {
			writeQuestionError(w, http.StatusForbidden, "FORBIDDEN", "无权删除此题目")
			return
		}
		if errors.Is(err, questionapp.ErrNotFound) {
			writeQuestionError(w, http.StatusNotFound, "NOT_FOUND", "题目不存在或无权删除")
			return
		}
		h.logQuestionError("delete question failed", err)
		writeQuestionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "删除题目失败")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) batchPublish(w http.ResponseWriter, r *http.Request) {
	h.batchOperation(w, r, h.service.BatchPublish, "批量发布失败")
}

func (h *Handler) batchDelete(w http.ResponseWriter, r *http.Request) {
	h.batchOperation(w, r, h.service.BatchDelete, "批量删除失败")
}

func (h *Handler) batchDuplicate(w http.ResponseWriter, r *http.Request) {
	h.batchOperation(w, r, h.service.BatchDuplicate, "批量复制失败")
}

func (h *Handler) batchOperation(w http.ResponseWriter, r *http.Request, fn func(context.Context, string, []string) (questionapp.BatchOperationResponse, error), fallback string) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
		return
	}
	var request batchRequest
	if !decodeRequest(w, r, &request) || !validateBatchIDs(w, request.QuestionIDs) {
		return
	}
	response, err := fn(r.Context(), principal.UserID, request.QuestionIDs)
	if err != nil {
		if errors.Is(err, questionapp.ErrBadRequest) {
			writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "question_ids 长度必须在 1 到 100 之间")
			return
		}
		h.logQuestionError("question batch operation failed", err)
		writeQuestionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fallback)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) batchImport(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
		return
	}
	var request batchImportRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	if len(request.Questions) < 1 || len(request.Questions) > questionapp.MaxBatchImportQuestions {
		writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "questions 长度必须在 1 到 200 之间")
		return
	}
	inputs := make([]questionapp.QuestionInput, 0, len(request.Questions))
	for _, item := range request.Questions {
		input, ok := item.toInput(w)
		if !ok {
			return
		}
		inputs = append(inputs, input)
	}
	response, err := h.service.BatchImport(r.Context(), principal.UserID, inputs)
	if err != nil {
		if errors.Is(err, questionapp.ErrBadRequest) {
			writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "questions 不合法")
			return
		}
		h.logQuestionError("batch import questions failed", err)
		writeQuestionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "批量导入失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) aiParse(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
		return
	}
	_ = principal
	var request aiParseRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	if len(request.RawTexts) < 1 || len(request.RawTexts) > questionapp.MaxAIParseRawTexts {
		writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "raw_texts 长度必须在 1 到 10 之间")
		return
	}
	for index, text := range request.RawTexts {
		if len(text) > questionapp.MaxAIParseRawTextBytes {
			writeQuestionError(w, http.StatusBadRequest, "BAD_REQUEST", "第 "+strconv.Itoa(index+1)+" 段文本超过 3000 字符限制")
			return
		}
		if strings.TrimSpace(text) == "" {
			writeQuestionError(w, http.StatusBadRequest, "BAD_REQUEST", "第 "+strconv.Itoa(index+1)+" 段文本不能为空")
			return
		}
	}
	response, err := h.service.ParseQuestions(r.Context(), request.RawTexts)
	if err != nil {
		if errors.Is(err, questionapp.ErrBadRequest) {
			writeQuestionError(w, http.StatusBadRequest, "BAD_REQUEST", "AI 题目识别输入不合法")
			return
		}
		h.logQuestionError("parse questions failed", err)
		writeQuestionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "AI 题目识别失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) generateIsomorphic(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireTeacher(w, r); !ok {
		return
	}
	var request generateRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	ability := 0.5
	if request.Ability != nil {
		if *request.Ability < 0 || *request.Ability > 1 {
			writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "ability 必须在 0 到 1 之间")
			return
		}
		ability = *request.Ability
	}
	if !validateDifficulty(w, request.Difficulty) {
		return
	}
	response, err := h.service.GenerateIsomorphicProblem(r.Context(), questionapp.GenerateRequest{
		Template:   request.Template,
		Ability:    ability,
		Difficulty: request.Difficulty,
		ConceptIDs: request.ConceptIDs,
		Tags:       request.Tags,
	})
	if err != nil {
		if errors.Is(err, questionapp.ErrBadRequest) {
			writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "不支持的题目模板")
			return
		}
		h.logQuestionError("generate isomorphic question failed", err)
		writeQuestionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "生成变式题失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) requireTeacher(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	fields := strings.Fields(authHeader)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeQuestionError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	principal, ok := h.auth.DecodeAccessToken(fields[1])
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeQuestionError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	if !authapp.IsTeacherOrAdmin(principal) {
		writeQuestionError(w, http.StatusForbidden, "FORBIDDEN", "权限不足，需要教师权限")
		return authapp.Principal{}, false
	}
	return principal, true
}

func (h *Handler) logQuestionError(message string, err error) {
	h.logger.Error(message, "error", redact.String(err.Error()))
}

func parseListFilter(w http.ResponseWriter, r *http.Request) (questionapp.ListFilter, bool) {
	page, ok := parseIntQuery(w, r.URL.Query().Get("page"), 1, "page")
	if !ok {
		return questionapp.ListFilter{}, false
	}
	pageSize, ok := parseIntQuery(w, r.URL.Query().Get("page_size"), 20, "page_size")
	if !ok {
		return questionapp.ListFilter{}, false
	}
	if page < 1 {
		writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "page 必须大于等于 1")
		return questionapp.ListFilter{}, false
	}
	if pageSize < 1 || pageSize > 100 {
		writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "page_size 必须在 1 到 100 之间")
		return questionapp.ListFilter{}, false
	}
	query := r.URL.Query()
	return questionapp.ListFilter{
		Page:       page,
		PageSize:   pageSize,
		Search:     query.Get("search"),
		Difficulty: query.Get("difficulty"),
		Type:       query.Get("type"),
		Status:     query.Get("status"),
		Tags:       httpquery.NamedStringList(query, "tags"),
		Group:      query.Get("group"),
		SortBy:     query.Get("sort_by"),
		SortOrder:  query.Get("sort_order"),
	}, true
}

func parseIntQuery(w http.ResponseWriter, value string, fallback int, name string) (int, bool) {
	parsed, err := httpquery.Int(value, fallback)
	if err != nil {
		writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", name+" 必须是整数")
		return 0, false
	}
	return parsed, true
}

func (r createRequest) toInput(w http.ResponseWriter) (questionapp.QuestionInput, bool) {
	if !validateRequiredString(w, r.Title, 1, questionapp.MaxQuestionTitleBytes, "title") ||
		!validateRequiredString(w, r.Body, 1, questionapp.MaxQuestionBodyBytes, "body") ||
		!validateRequiredField(w, r.Answer, "answer") ||
		!validateOptionalString(w, r.Answer, questionapp.MaxQuestionAnswerBytes, "answer") ||
		!validateStringSlice(w, r.ConceptIDs, "concept_ids") ||
		!validateStringSlice(w, r.Tags, "tags") ||
		!validateStringSlice(w, r.Hints, "hints") ||
		!validateStringSlice(w, r.SolutionSteps, "solution_steps") ||
		!validateOptionalStringSlice(w, r.Options, "options") ||
		!validateDifficulty(w, r.Difficulty) ||
		!validateEstimatedSeconds(w, r.EstimatedTimeSeconds) {
		return questionapp.QuestionInput{}, false
	}
	difficulty := 0.5
	if r.Difficulty != nil {
		difficulty = *r.Difficulty
	}
	estimated := 300
	if r.EstimatedTimeSeconds != nil {
		estimated = *r.EstimatedTimeSeconds
	}
	return questionapp.QuestionInput{
		Title:                r.Title,
		Body:                 r.Body,
		Type:                 r.Type,
		Difficulty:           difficulty,
		ConceptIDs:           r.ConceptIDs,
		Tags:                 r.Tags,
		Answer:               *r.Answer,
		AnswerType:           r.AnswerType,
		Hints:                r.Hints,
		SolutionSteps:        r.SolutionSteps,
		Options:              r.Options,
		EstimatedTimeSeconds: estimated,
	}, true
}

func (r updateRequest) toUpdate(w http.ResponseWriter) (questionapp.QuestionUpdate, bool) {
	if r.Title != nil && !validateRequiredString(w, *r.Title, 1, questionapp.MaxQuestionTitleBytes, "title") {
		return questionapp.QuestionUpdate{}, false
	}
	if r.Body != nil && !validateRequiredString(w, *r.Body, 1, questionapp.MaxQuestionBodyBytes, "body") {
		return questionapp.QuestionUpdate{}, false
	}
	if !validateOptionalString(w, r.Answer, questionapp.MaxQuestionAnswerBytes, "answer") ||
		!validateOptionalStringSlice(w, r.ConceptIDs, "concept_ids") ||
		!validateOptionalStringSlice(w, r.Tags, "tags") ||
		!validateOptionalStringSlice(w, r.Hints, "hints") ||
		!validateOptionalStringSlice(w, r.SolutionSteps, "solution_steps") ||
		!validateOptionalStringSlice(w, r.Options, "options") ||
		!validateDifficulty(w, r.Difficulty) ||
		!validateEstimatedSeconds(w, r.EstimatedTimeSeconds) {
		return questionapp.QuestionUpdate{}, false
	}
	if r.Status != nil && !validStatus(*r.Status) {
		writeQuestionError(w, http.StatusBadRequest, "BAD_REQUEST", "无效的状态值: "+*r.Status)
		return questionapp.QuestionUpdate{}, false
	}
	return questionapp.QuestionUpdate{
		Title:                r.Title,
		Body:                 r.Body,
		Type:                 r.Type,
		Difficulty:           r.Difficulty,
		ConceptIDs:           r.ConceptIDs,
		Tags:                 r.Tags,
		Answer:               r.Answer,
		AnswerType:           r.AnswerType,
		Hints:                r.Hints,
		SolutionSteps:        r.SolutionSteps,
		Options:              r.Options,
		EstimatedTimeSeconds: r.EstimatedTimeSeconds,
		Status:               r.Status,
	}, true
}

func validateRequiredString(w http.ResponseWriter, value string, min int, max int, name string) bool {
	length := len(strings.TrimSpace(value))
	if length < min {
		writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", name+" 不能为空")
		return false
	}
	if max > 0 && length > max {
		writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", name+" 长度超出限制")
		return false
	}
	return true
}

func validateRequiredField(w http.ResponseWriter, value *string, name string) bool {
	if value != nil {
		return true
	}
	writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", name+" 为必填字段")
	return false
}

func validateOptionalString(w http.ResponseWriter, value *string, max int, name string) bool {
	if value == nil || len(*value) <= max {
		return true
	}
	writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", name+" 长度超出限制")
	return false
}

func validateStringSlice(w http.ResponseWriter, values []string, name string) bool {
	if len(values) > questionapp.MaxQuestionListItems {
		writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", name+" 数量超出限制")
		return false
	}
	for _, value := range values {
		if len(strings.TrimSpace(value)) > questionapp.MaxQuestionListItemBytes {
			writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", name+" 单项长度超出限制")
			return false
		}
	}
	return true
}

func validateOptionalStringSlice(w http.ResponseWriter, values *[]string, name string) bool {
	if values == nil {
		return true
	}
	return validateStringSlice(w, *values, name)
}

func validateDifficulty(w http.ResponseWriter, value *float64) bool {
	if value == nil {
		return true
	}
	if *value < 0 || *value > 1 {
		writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "difficulty 必须在 0 到 1 之间")
		return false
	}
	return true
}

func validateEstimatedSeconds(w http.ResponseWriter, value *int) bool {
	if value == nil || (*value >= 0 && *value <= questionapp.MaxQuestionEstimatedTimeSeconds) {
		return true
	}
	writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "estimated_time_seconds 必须在 0 到 86400 之间")
	return false
}

func validateBatchIDs(w http.ResponseWriter, ids []string) bool {
	if len(ids) >= 1 && len(ids) <= questionapp.MaxBatchOperationIDs {
		return true
	}
	writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "question_ids 长度必须在 1 到 100 之间")
	return false
}

func validStatus(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "draft", "published", "archived":
		return true
	default:
		return false
	}
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := httpjson.DecodeStrict(w, r, maxJSONBodyBytes, target); err != nil {
		writeQuestionError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "请求体格式错误")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeQuestionError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Detail: message, Code: code, Message: message})
}
