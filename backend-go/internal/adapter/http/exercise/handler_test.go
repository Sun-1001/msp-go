package exercisehttp

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
	"time"

	authapp "mathstudy/backend-go/internal/application/auth"
	exerciseapp "mathstudy/backend-go/internal/application/exercise"
	"mathstudy/backend-go/internal/domain/user"
)

func TestNextRequiresBearerToken(t *testing.T) {
	handler := newTestHandler(t, &fakeExerciseService{}, &fakeAuthenticator{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/exercise")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/exercise/next", nil)
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
}

func TestNextForwardsQueryAndWritesNull(t *testing.T) {
	service := &fakeExerciseService{}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/exercise")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/exercise/next?concept_id=limit&difficulty=0.4", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if strings.TrimSpace(recorder.Body.String()) != "null" {
		t.Fatalf("body = %q", recorder.Body.String())
	}
	if service.lastUserID != "student-1" || service.lastNextQuery.ConceptID != "limit" {
		t.Fatalf("service = %#v", service)
	}
	if service.lastNextQuery.Difficulty == nil || *service.lastNextQuery.Difficulty != 0.4 {
		t.Fatalf("difficulty = %#v", service.lastNextQuery.Difficulty)
	}
}

func TestNextRejectsInvalidDifficultyBeforeServiceCall(t *testing.T) {
	service := &fakeExerciseService{}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/exercise")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/exercise/next?difficulty=1.5", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "difficulty 必须在 0 到 1 之间") {
		t.Fatalf("body = %s", recorder.Body.String())
	}
	if service.lastUserID != "" {
		t.Fatalf("service was called for invalid difficulty: %#v", service)
	}
}

func TestGenerateCreatesStudentAIExercise(t *testing.T) {
	service := &fakeExerciseService{generateResponse: &exerciseapp.ExerciseResponse{ID: "generated-1", Source: exerciseapp.ExerciseSourceAIGenerated}}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/exercise")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/exercise/generate", strings.NewReader(`{"concept_id":"limit","difficulty":0.5}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !service.generateCalled || service.lastGenerateRequest.ConceptID != "limit" || service.lastGenerateRequest.Difficulty != 0.5 {
		t.Fatalf("service = %#v", service)
	}
	if !strings.Contains(recorder.Body.String(), `"source":"ai_generated"`) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestGenerateRequiresStudentRole(t *testing.T) {
	service := &fakeExerciseService{}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/exercise")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/exercise/generate", strings.NewReader(`{"concept_id":"limit","difficulty":0.5}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden || service.generateCalled {
		t.Fatalf("status=%d service=%#v", recorder.Code, service)
	}
}

func TestGenerateValidatesRequestBeforeServiceCall(t *testing.T) {
	service := &fakeExerciseService{}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/exercise")

	for _, body := range []string{
		`{"concept_id":"","difficulty":0.5}`,
		`{"concept_id":"limit"}`,
		`{"concept_id":"limit","difficulty":1.1}`,
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/exercise/generate", strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer token")
		mux.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("body=%s status=%d response=%s", body, recorder.Code, recorder.Body.String())
		}
	}
	if service.generateCalled {
		t.Fatal("service was called for invalid request")
	}
}

func TestGenerateMapsUnavailableAndRateLimit(t *testing.T) {
	service := &fakeExerciseService{generateErr: exerciseapp.ErrAIGenerationUnavailable}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	handler.limiter = newGenerationRateLimiter(1, time.Minute)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/exercise")
	body := `{"concept_id":"limit","difficulty":0.5}`

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/exercise/generate", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "AI_GENERATION_UNAVAILABLE") {
		t.Fatalf("first status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/exercise/generate", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusTooManyRequests || !strings.Contains(recorder.Body.String(), "RATE_LIMITED") {
		t.Fatalf("second status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSubmitRejectsMissingAnswerBeforeServiceCall(t *testing.T) {
	service := &fakeExerciseService{}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/exercise")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/exercise/submit", strings.NewReader(`{"exercise_id":"exercise-1"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
	if service.submitCalled {
		t.Fatal("service was called")
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["detail"] != "请提供文本答案" {
		t.Fatalf("body = %#v", body)
	}
}

func TestSubmitForwardsPayloadAndMapsBadRequest(t *testing.T) {
	service := &fakeExerciseService{submitErr: exerciseapp.ErrBadRequest}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/exercise")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/exercise/submit", strings.NewReader(`{"exercise_id":"exercise-1","answer_text":"42","answer_steps":["s"],"time_spent_seconds":12}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !service.submitCalled || service.lastSubmitRequest.ExerciseID != "exercise-1" || service.lastSubmitRequest.AnswerText != "42" || service.lastSubmitRequest.TimeSpentSeconds != 12 {
		t.Fatalf("request = %#v", service.lastSubmitRequest)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["detail"] != "提交失败，请检查输入后重试" {
		t.Fatalf("body = %#v", body)
	}
}

func TestSubmitMapsImageOnlyOCRUnavailable(t *testing.T) {
	service := &fakeExerciseService{submitErr: exerciseapp.ErrOCRUnavailable}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/exercise")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/exercise/submit", strings.NewReader(`{"exercise_id":"exercise-1","answer_image_url":"/uploads/images/answer.png"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !service.submitCalled || service.lastSubmitRequest.AnswerImageURL != "/uploads/images/answer.png" {
		t.Fatalf("request = %#v", service.lastSubmitRequest)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["code"] != "OCR_UNAVAILABLE" || body["detail"] != "图片答案自动判题尚未开放，请改用文本答案" {
		t.Fatalf("body = %#v", body)
	}
}

func TestSubmitRejectsTrailingJSON(t *testing.T) {
	service := &fakeExerciseService{}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/exercise")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/exercise/submit", strings.NewReader(`{"exercise_id":"exercise-1","answer_text":"42"} {"answer_text":"extra"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if service.submitCalled {
		t.Fatalf("service was called for trailing JSON body: %#v", service.lastSubmitRequest)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["detail"] != "请求体格式错误" || body["code"] != "VALIDATION_ERROR" {
		t.Fatalf("body = %#v", body)
	}
}

func TestSolutionRouteDoesNotHitDetailRoute(t *testing.T) {
	service := &fakeExerciseService{
		solutionResponse: exerciseapp.SolutionResponse{ExerciseID: "exercise-1", Answer: "42", Steps: []string{"step"}, Source: "cached"},
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/exercise")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/exercise/exercise-1/solution", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !service.solutionCalled || service.detailCalled {
		t.Fatalf("service calls = %#v", service)
	}
}

func TestDetailMapsNotFound(t *testing.T) {
	service := &fakeExerciseService{detailErr: exerciseapp.ErrNotFound}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/exercise")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/exercise/exercise-1", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestInternalErrorsRedactLogs(t *testing.T) {
	var logBuffer bytes.Buffer
	service := &fakeExerciseService{nextErr: errors.New("exercise repo failed Authorization: Bearer exercise-secret token=query-token api_key=plain password=letmein")}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler, err := NewHandler(slog.New(slog.NewTextHandler(&logBuffer, nil)), service, auth)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/exercise")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/exercise/next", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	assertNoExerciseCredentialLeak(t, recorder.Body.String())
	assertNoExerciseCredentialLeak(t, logBuffer.String())
}

func TestNewHandlerRejectsMissingDependencies(t *testing.T) {
	if _, err := NewHandler(nil, nil, &fakeAuthenticator{}); err == nil {
		t.Fatal("NewHandler(nil service) error = nil, want error")
	}
	if _, err := NewHandler(nil, &fakeExerciseService{}, nil); err == nil {
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

func assertNoExerciseCredentialLeak(t *testing.T, value string) {
	t.Helper()
	for _, leaked := range []string{"exercise-secret", "token=query-token", "api_key=plain", "password=letmein", "Bearer exercise-secret"} {
		if strings.Contains(value, leaked) {
			t.Fatalf("value leaked %q in %q", leaked, value)
		}
	}
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

type fakeExerciseService struct {
	nextResponse        *exerciseapp.ExerciseResponse
	generateResponse    *exerciseapp.ExerciseResponse
	submitResponse      exerciseapp.SubmitResponse
	detailResponse      exerciseapp.ExerciseDetailResponse
	solutionResponse    exerciseapp.SolutionResponse
	nextErr             error
	generateErr         error
	submitErr           error
	detailErr           error
	solutionErr         error
	lastUserID          string
	lastNextQuery       exerciseapp.NextQuery
	lastGenerateRequest exerciseapp.GenerateExerciseRequest
	lastSubmitRequest   exerciseapp.SubmitRequest
	lastExerciseID      string
	submitCalled        bool
	generateCalled      bool
	detailCalled        bool
	solutionCalled      bool
}

func (s *fakeExerciseService) GetNextExercise(_ context.Context, userID string, query exerciseapp.NextQuery) (*exerciseapp.ExerciseResponse, error) {
	s.lastUserID = userID
	s.lastNextQuery = query
	return s.nextResponse, s.nextErr
}

func (s *fakeExerciseService) GenerateExercise(_ context.Context, userID string, request exerciseapp.GenerateExerciseRequest) (*exerciseapp.ExerciseResponse, error) {
	s.lastUserID = userID
	s.lastGenerateRequest = request
	s.generateCalled = true
	return s.generateResponse, s.generateErr
}

func (s *fakeExerciseService) SubmitAnswer(_ context.Context, userID string, request exerciseapp.SubmitRequest) (exerciseapp.SubmitResponse, error) {
	s.lastUserID = userID
	s.lastSubmitRequest = request
	s.submitCalled = true
	return s.submitResponse, s.submitErr
}

func (s *fakeExerciseService) GetExercise(_ context.Context, userID string, exerciseID string) (exerciseapp.ExerciseDetailResponse, error) {
	s.lastUserID = userID
	s.lastExerciseID = exerciseID
	s.detailCalled = true
	return s.detailResponse, s.detailErr
}

func (s *fakeExerciseService) GetSolution(_ context.Context, userID string, exerciseID string) (exerciseapp.SolutionResponse, error) {
	s.lastUserID = userID
	s.lastExerciseID = exerciseID
	s.solutionCalled = true
	return s.solutionResponse, s.solutionErr
}
