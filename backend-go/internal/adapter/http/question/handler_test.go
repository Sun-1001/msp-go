package questionhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	authapp "mathstudy/backend-go/internal/application/auth"
	questionapp "mathstudy/backend-go/internal/application/question"
	"mathstudy/backend-go/internal/domain/user"
)

func TestListRequiresTeacherBearerToken(t *testing.T) {
	handler := newTestHandler(t, &fakeQuestionService{}, &fakeAuthenticator{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/questions")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/questions", nil)
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
}

func TestListForwardsFilters(t *testing.T) {
	service := &fakeQuestionService{listResponse: questionapp.ListResponse{Items: []questionapp.Question{{ID: "question-1"}}, Total: 1, Page: 2, PageSize: 10}}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/questions")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/questions?page=2&page_size=10&search=limit&difficulty=easy&type=proof&status=published&tags=calculus&tags[]=exam&group=导数&sort_by=usage_count&sort_order=asc", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	filter := service.lastFilter
	if service.lastOwnerID != "teacher-1" || filter.Page != 2 || filter.PageSize != 10 || filter.Search != "limit" || filter.Type != "proof" {
		t.Fatalf("owner/filter = %q %#v", service.lastOwnerID, filter)
	}
	if len(filter.Tags) != 2 || filter.Group != "导数" || filter.SortBy != "usage_count" || filter.SortOrder != "asc" {
		t.Fatalf("filter = %#v", filter)
	}
}

func TestCreateRequiresTeacherRole(t *testing.T) {
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, &fakeQuestionService{}, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/questions")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/questions", bytes.NewBufferString(`{"title":"导数","body":"题目","answer":"1"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestCreateValidatesDefaultsAndReturnsCreated(t *testing.T) {
	service := &fakeQuestionService{createResponse: questionapp.Question{ID: "question-1", Title: "导数"}}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/questions")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/questions", bytes.NewBufferString(`{"title":"导数","body":"题目","answer":"1"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if service.lastOwnerID != "teacher-1" || service.lastInput.Difficulty != 0.5 || service.lastInput.EstimatedTimeSeconds != 300 {
		t.Fatalf("input = %#v owner = %q", service.lastInput, service.lastOwnerID)
	}
}

func TestStatsAndGroupsUseLiteralRoutes(t *testing.T) {
	service := &fakeQuestionService{
		statsResponse:  questionapp.Stats{Total: 3, ByDifficulty: map[string]int{}, ByType: map[string]int{}, ByStatus: map[string]int{}},
		groupsResponse: questionapp.GroupsResponse{Groups: []string{"导数"}},
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/questions")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/questions/stats", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !service.statsCalled || service.detailCalled {
		t.Fatalf("stats status=%d statsCalled=%t detailCalled=%t", recorder.Code, service.statsCalled, service.detailCalled)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/v1/questions/groups", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !service.groupsCalled || service.detailCalled {
		t.Fatalf("groups status=%d groupsCalled=%t detailCalled=%t", recorder.Code, service.groupsCalled, service.detailCalled)
	}
}

func TestUpdateRejectsInvalidStatus(t *testing.T) {
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}}
	handler := newTestHandler(t, &fakeQuestionService{}, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/questions")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/questions/question-1", bytes.NewBufferString(`{"status":"bad"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	service := &fakeQuestionService{updateErr: questionapp.ErrBadRequest}
	handler = newTestHandler(t, service, auth)
	mux = http.NewServeMux()
	handler.Register(mux, "/api/v1/questions")

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPut, "/api/v1/questions/question-1", bytes.NewBufferString(`{"title":"导数"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("service bad request status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestDetailDeleteAndBatchMapErrors(t *testing.T) {
	service := &fakeQuestionService{detailErr: questionapp.ErrNotFound, deleteErr: questionapp.ErrNotFound, batchErr: questionapp.ErrBadRequest}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/questions")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/questions/missing", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("detail status = %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodDelete, "/api/v1/questions/missing", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("delete status = %d", recorder.Code)
	}

	service.detailErr = questionapp.ErrBadRequest
	service.deleteErr = questionapp.ErrBadRequest
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/v1/questions/bad", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("detail bad request status = %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodDelete, "/api/v1/questions/bad", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("delete bad request status = %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/questions/batch/publish", bytes.NewBufferString(`{"question_ids":["q1"]}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("batch status = %d", recorder.Code)
	}
}

func TestBatchImportAndAIParse(t *testing.T) {
	service := &fakeQuestionService{
		batchResponse: questionapp.BatchOperationResponse{Success: 1, Failed: 0, FailedIDs: []string{}, Errors: []string{}},
		parseResponse: questionapp.AIParseResponse{Questions: []questionapp.AIParseQuestionItem{{Title: "导数"}}},
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/questions")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/questions/batch/import", bytes.NewBufferString(`{"questions":[{"title":"导数","body":"题目","answer":"1"}]}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || len(service.lastInputs) != 1 {
		t.Fatalf("import status=%d body=%s inputs=%#v", recorder.Code, recorder.Body.String(), service.lastInputs)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/questions/ai-parse", bytes.NewBufferString(`{"raw_texts":["导数题"]}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || len(service.lastRawTexts) != 1 {
		t.Fatalf("ai parse status=%d body=%s raw=%#v", recorder.Code, recorder.Body.String(), service.lastRawTexts)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := body["questions"]; !ok {
		t.Fatalf("body = %#v", body)
	}
}

func TestQuestionBoundaryValidationRejectsBeforeService(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		assert     func(*testing.T, *fakeQuestionService)
	}{
		{
			name:       "create oversized body",
			method:     http.MethodPost,
			path:       "/api/v1/questions",
			body:       `{"title":"导数","body":"` + strings.Repeat("x", questionapp.MaxQuestionBodyBytes+1) + `","answer":"1"}`,
			wantStatus: http.StatusUnprocessableEntity,
			assert: func(t *testing.T, service *fakeQuestionService) {
				t.Helper()
				if service.lastOwnerID != "" {
					t.Fatalf("service was called: %#v", service.lastInput)
				}
			},
		},
		{
			name:       "batch publish too many IDs",
			method:     http.MethodPost,
			path:       "/api/v1/questions/batch/publish",
			body:       `{"question_ids":` + stringListJSON(questionapp.MaxBatchOperationIDs+1, "question-1") + `}`,
			wantStatus: http.StatusUnprocessableEntity,
			assert: func(t *testing.T, service *fakeQuestionService) {
				t.Helper()
				if len(service.lastIDs) != 0 {
					t.Fatalf("service was called: %#v", service.lastIDs)
				}
			},
		},
		{
			name:       "batch import too many questions",
			method:     http.MethodPost,
			path:       "/api/v1/questions/batch/import",
			body:       questionImportJSON(questionapp.MaxBatchImportQuestions + 1),
			wantStatus: http.StatusUnprocessableEntity,
			assert: func(t *testing.T, service *fakeQuestionService) {
				t.Helper()
				if len(service.lastInputs) != 0 {
					t.Fatalf("service was called: %#v", service.lastInputs)
				}
			},
		},
		{
			name:       "batch import too many options",
			method:     http.MethodPost,
			path:       "/api/v1/questions/batch/import",
			body:       `{"questions":[{"title":"导数","body":"题目","answer":"1","options":` + stringListJSON(questionapp.MaxQuestionListItems+1, "A") + `}]}`,
			wantStatus: http.StatusUnprocessableEntity,
			assert: func(t *testing.T, service *fakeQuestionService) {
				t.Helper()
				if len(service.lastInputs) != 0 {
					t.Fatalf("service was called: %#v", service.lastInputs)
				}
			},
		},
		{
			name:       "ai parse too many raw texts",
			method:     http.MethodPost,
			path:       "/api/v1/questions/ai-parse",
			body:       `{"raw_texts":` + stringListJSON(questionapp.MaxAIParseRawTexts+1, "导数题") + `}`,
			wantStatus: http.StatusUnprocessableEntity,
			assert: func(t *testing.T, service *fakeQuestionService) {
				t.Helper()
				if len(service.lastRawTexts) != 0 {
					t.Fatalf("service was called: %#v", service.lastRawTexts)
				}
			},
		},
		{
			name:       "ai parse oversized raw text",
			method:     http.MethodPost,
			path:       "/api/v1/questions/ai-parse",
			body:       `{"raw_texts":["` + strings.Repeat("x", questionapp.MaxAIParseRawTextBytes+1) + `"]}`,
			wantStatus: http.StatusBadRequest,
			assert: func(t *testing.T, service *fakeQuestionService) {
				t.Helper()
				if len(service.lastRawTexts) != 0 {
					t.Fatalf("service was called: %#v", service.lastRawTexts)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeQuestionService{}
			handler := newTestHandler(t, service, &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}})
			mux := http.NewServeMux()
			handler.Register(mux, "/api/v1/questions")

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			request.Header.Set("Authorization", "Bearer token")
			mux.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			tt.assert(t, service)
		})
	}
}

func TestGenerateIsomorphicProblemForwardsRequest(t *testing.T) {
	service := &fakeQuestionService{
		generateResponse: questionapp.GeneratedQuestion{
			Title:      "变式题",
			Template:   "integral_power_exp",
			Parameters: map[string]int{"n": 2, "a": 3},
			Validation: questionapp.GenerationValidation{HasClosedForm: true, InSyllabus: true},
		},
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/questions")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/questions/generate-isomorphic", bytes.NewBufferString(`{"template":"integral_power_exp","ability":0.7,"concept_ids":["integral"]}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.lastGenerate.Ability != 0.7 || len(service.lastGenerate.ConceptIDs) != 1 || service.lastGenerate.ConceptIDs[0] != "integral" {
		t.Fatalf("generate request = %#v", service.lastGenerate)
	}
}

func TestJSONRoutesRejectTrailingJSON(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		assert func(*testing.T, *fakeQuestionService)
	}{
		{
			name:   "create",
			method: http.MethodPost,
			path:   "/api/v1/questions",
			body:   `{"title":"导数","body":"题目","answer":"1"} {"title":"extra"}`,
			assert: func(t *testing.T, service *fakeQuestionService) {
				t.Helper()
				if service.lastOwnerID != "" {
					t.Fatalf("service was called for create trailing JSON: %#v", service.lastInput)
				}
			},
		},
		{
			name:   "update",
			method: http.MethodPut,
			path:   "/api/v1/questions/question-1",
			body:   `{"title":"导数"} {"title":"extra"}`,
			assert: func(t *testing.T, service *fakeQuestionService) {
				t.Helper()
				if service.lastOwnerID != "" {
					t.Fatalf("service was called for update trailing JSON: %#v", service.lastUpdate)
				}
			},
		},
		{
			name:   "batch publish",
			method: http.MethodPost,
			path:   "/api/v1/questions/batch/publish",
			body:   `{"question_ids":["q1"]} {"question_ids":["q2"]}`,
			assert: func(t *testing.T, service *fakeQuestionService) {
				t.Helper()
				if service.lastOwnerID != "" || len(service.lastIDs) != 0 {
					t.Fatalf("service was called for batch trailing JSON: owner=%q ids=%#v", service.lastOwnerID, service.lastIDs)
				}
			},
		},
		{
			name:   "batch import",
			method: http.MethodPost,
			path:   "/api/v1/questions/batch/import",
			body:   `{"questions":[{"title":"导数","body":"题目","answer":"1"}]} {"questions":[]}`,
			assert: func(t *testing.T, service *fakeQuestionService) {
				t.Helper()
				if service.lastOwnerID != "" || len(service.lastInputs) != 0 {
					t.Fatalf("service was called for import trailing JSON: owner=%q inputs=%#v", service.lastOwnerID, service.lastInputs)
				}
			},
		},
		{
			name:   "ai parse",
			method: http.MethodPost,
			path:   "/api/v1/questions/ai-parse",
			body:   `{"raw_texts":["导数题"]} {"raw_texts":["extra"]}`,
			assert: func(t *testing.T, service *fakeQuestionService) {
				t.Helper()
				if len(service.lastRawTexts) != 0 {
					t.Fatalf("service was called for ai parse trailing JSON: %#v", service.lastRawTexts)
				}
			},
		},
		{
			name:   "generate isomorphic",
			method: http.MethodPost,
			path:   "/api/v1/questions/generate-isomorphic",
			body:   `{"template":"integral_power_exp","ability":0.7,"concept_ids":["integral"]} {"template":"extra"}`,
			assert: func(t *testing.T, service *fakeQuestionService) {
				t.Helper()
				if service.lastGenerate.Template != "" {
					t.Fatalf("service was called for generate trailing JSON: %#v", service.lastGenerate)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeQuestionService{}
			handler := newTestHandler(t, service, &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}})
			mux := http.NewServeMux()
			handler.Register(mux, "/api/v1/questions")

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			request.Header.Set("Authorization", "Bearer token")
			mux.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			var body map[string]string
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if body["detail"] != "请求体格式错误" || body["code"] != "VALIDATION_ERROR" {
				t.Fatalf("body = %#v", body)
			}
			tt.assert(t, service)
		})
	}
}

func TestInternalErrorsRedactLogs(t *testing.T) {
	credentialErr := errors.New("question repo failed Authorization: Bearer question-secret token=query-token api_key=plain password=letmein")
	tests := []struct {
		name    string
		method  string
		path    string
		body    string
		service fakeQuestionService
	}{
		{
			name:    "list",
			method:  http.MethodGet,
			path:    "/api/v1/questions",
			service: fakeQuestionService{listErr: credentialErr},
		},
		{
			name:    "groups",
			method:  http.MethodGet,
			path:    "/api/v1/questions/groups",
			service: fakeQuestionService{groupsErr: credentialErr},
		},
		{
			name:    "stats",
			method:  http.MethodGet,
			path:    "/api/v1/questions/stats",
			service: fakeQuestionService{statsErr: credentialErr},
		},
		{
			name:    "detail",
			method:  http.MethodGet,
			path:    "/api/v1/questions/question-1",
			service: fakeQuestionService{detailErr: credentialErr},
		},
		{
			name:    "create",
			method:  http.MethodPost,
			path:    "/api/v1/questions",
			body:    `{"title":"导数","body":"题目","answer":"1"}`,
			service: fakeQuestionService{createErr: credentialErr},
		},
		{
			name:    "update",
			method:  http.MethodPut,
			path:    "/api/v1/questions/question-1",
			body:    `{"title":"导数"}`,
			service: fakeQuestionService{updateErr: credentialErr},
		},
		{
			name:    "delete",
			method:  http.MethodDelete,
			path:    "/api/v1/questions/question-1",
			service: fakeQuestionService{deleteErr: credentialErr},
		},
		{
			name:    "batch publish",
			method:  http.MethodPost,
			path:    "/api/v1/questions/batch/publish",
			body:    `{"question_ids":["q1"]}`,
			service: fakeQuestionService{batchErr: credentialErr},
		},
		{
			name:    "batch import",
			method:  http.MethodPost,
			path:    "/api/v1/questions/batch/import",
			body:    `{"questions":[{"title":"导数","body":"题目","answer":"1"}]}`,
			service: fakeQuestionService{importErr: credentialErr},
		},
		{
			name:    "ai parse",
			method:  http.MethodPost,
			path:    "/api/v1/questions/ai-parse",
			body:    `{"raw_texts":["导数题"]}`,
			service: fakeQuestionService{parseErr: credentialErr},
		},
		{
			name:    "generate isomorphic",
			method:  http.MethodPost,
			path:    "/api/v1/questions/generate-isomorphic",
			body:    `{"template":"integral_power_exp","ability":0.7,"concept_ids":["integral"]}`,
			service: fakeQuestionService{generateErr: credentialErr},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logBuffer bytes.Buffer
			service := tt.service
			handler, err := NewHandler(slog.New(slog.NewTextHandler(&logBuffer, nil)), &service, &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}})
			if err != nil {
				t.Fatalf("NewHandler() error = %v", err)
			}
			mux := http.NewServeMux()
			handler.Register(mux, "/api/v1/questions")

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			request.Header.Set("Authorization", "Bearer token")
			mux.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if logBuffer.Len() == 0 {
				t.Fatal("expected internal error log")
			}
			assertNoQuestionCredentialLeak(t, recorder.Body.String())
			assertNoQuestionCredentialLeak(t, logBuffer.String())
		})
	}
}

func TestNewHandlerRejectsMissingDependencies(t *testing.T) {
	if _, err := NewHandler(nil, nil, &fakeAuthenticator{}); err == nil {
		t.Fatal("NewHandler(nil service) error = nil, want error")
	}
	if _, err := NewHandler(nil, &fakeQuestionService{}, nil); err == nil {
		t.Fatal("NewHandler(nil auth) error = nil, want error")
	}
}

func newTestHandler(t *testing.T, service Service, auth Authenticator) *Handler {
	t.Helper()
	handler, err := NewHandler(slog.New(slog.NewTextHandler(os.Stdout, nil)), service, auth)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return handler
}

func assertNoQuestionCredentialLeak(t *testing.T, value string) {
	t.Helper()
	for _, leaked := range []string{"question-secret", "token=query-token", "api_key=plain", "password=letmein", "Bearer question-secret"} {
		if strings.Contains(value, leaked) {
			t.Fatalf("value leaked %q in %q", leaked, value)
		}
	}
}

func questionImportJSON(count int) string {
	var builder strings.Builder
	builder.WriteString(`{"questions":[`)
	for index := 0; index < count; index++ {
		if index > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(`{"title":"导数","body":"题目","answer":"1"}`)
	}
	builder.WriteString(`]}`)
	return builder.String()
}

func stringListJSON(count int, value string) string {
	var builder strings.Builder
	builder.WriteByte('[')
	for index := 0; index < count; index++ {
		if index > 0 {
			builder.WriteByte(',')
		}
		encoded, _ := json.Marshal(value)
		builder.Write(encoded)
	}
	builder.WriteByte(']')
	return builder.String()
}

type fakeAuthenticator struct {
	principal authapp.Principal
}

func (a *fakeAuthenticator) DecodeAccessToken(string) (authapp.Principal, bool) {
	if a.principal.UserID == "" {
		return authapp.Principal{}, false
	}
	return a.principal, true
}

type fakeQuestionService struct {
	listResponse     questionapp.ListResponse
	detailResponse   questionapp.Question
	createResponse   questionapp.Question
	updateResponse   questionapp.Question
	groupsResponse   questionapp.GroupsResponse
	statsResponse    questionapp.Stats
	batchResponse    questionapp.BatchOperationResponse
	parseResponse    questionapp.AIParseResponse
	generateResponse questionapp.GeneratedQuestion
	listErr          error
	detailErr        error
	createErr        error
	updateErr        error
	deleteErr        error
	groupsErr        error
	statsErr         error
	batchErr         error
	importErr        error
	parseErr         error
	generateErr      error
	lastOwnerID      string
	lastQuestionID   string
	lastFilter       questionapp.ListFilter
	lastInput        questionapp.QuestionInput
	lastUpdate       questionapp.QuestionUpdate
	lastIDs          []string
	lastInputs       []questionapp.QuestionInput
	lastRawTexts     []string
	lastGenerate     questionapp.GenerateRequest
	statsCalled      bool
	groupsCalled     bool
	detailCalled     bool
}

func (s *fakeQuestionService) ListQuestions(_ context.Context, ownerID string, filter questionapp.ListFilter) (questionapp.ListResponse, error) {
	s.lastOwnerID = ownerID
	s.lastFilter = filter
	return s.listResponse, s.listErr
}

func (s *fakeQuestionService) GetQuestion(_ context.Context, ownerID string, questionID string) (questionapp.Question, error) {
	s.lastOwnerID = ownerID
	s.lastQuestionID = questionID
	s.detailCalled = true
	return s.detailResponse, s.detailErr
}

func (s *fakeQuestionService) CreateQuestion(_ context.Context, ownerID string, input questionapp.QuestionInput) (questionapp.Question, error) {
	s.lastOwnerID = ownerID
	s.lastInput = input
	return s.createResponse, s.createErr
}

func (s *fakeQuestionService) UpdateQuestion(_ context.Context, ownerID string, questionID string, update questionapp.QuestionUpdate) (questionapp.Question, error) {
	s.lastOwnerID = ownerID
	s.lastQuestionID = questionID
	s.lastUpdate = update
	return s.updateResponse, s.updateErr
}

func (s *fakeQuestionService) DeleteQuestion(_ context.Context, ownerID string, questionID string) error {
	s.lastOwnerID = ownerID
	s.lastQuestionID = questionID
	return s.deleteErr
}

func (s *fakeQuestionService) GetGroups(_ context.Context, ownerID string) (questionapp.GroupsResponse, error) {
	s.lastOwnerID = ownerID
	s.groupsCalled = true
	return s.groupsResponse, s.groupsErr
}

func (s *fakeQuestionService) GetStats(_ context.Context, ownerID string) (questionapp.Stats, error) {
	s.lastOwnerID = ownerID
	s.statsCalled = true
	return s.statsResponse, s.statsErr
}

func (s *fakeQuestionService) BatchPublish(_ context.Context, ownerID string, ids []string) (questionapp.BatchOperationResponse, error) {
	s.lastOwnerID = ownerID
	s.lastIDs = ids
	return s.batchResponse, s.batchErr
}

func (s *fakeQuestionService) BatchDelete(_ context.Context, ownerID string, ids []string) (questionapp.BatchOperationResponse, error) {
	s.lastOwnerID = ownerID
	s.lastIDs = ids
	return s.batchResponse, s.batchErr
}

func (s *fakeQuestionService) BatchDuplicate(_ context.Context, ownerID string, ids []string) (questionapp.BatchOperationResponse, error) {
	s.lastOwnerID = ownerID
	s.lastIDs = ids
	return s.batchResponse, s.batchErr
}

func (s *fakeQuestionService) BatchImport(_ context.Context, ownerID string, inputs []questionapp.QuestionInput) (questionapp.BatchOperationResponse, error) {
	s.lastOwnerID = ownerID
	s.lastInputs = inputs
	return s.batchResponse, s.importErr
}

func (s *fakeQuestionService) ParseQuestions(_ context.Context, rawTexts []string) (questionapp.AIParseResponse, error) {
	s.lastRawTexts = rawTexts
	return s.parseResponse, s.parseErr
}

func (s *fakeQuestionService) GenerateIsomorphicProblem(_ context.Context, request questionapp.GenerateRequest) (questionapp.GeneratedQuestion, error) {
	s.lastGenerate = request
	return s.generateResponse, s.generateErr
}
