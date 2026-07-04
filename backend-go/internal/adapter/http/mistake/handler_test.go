package mistakehttp

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
	mistakeapp "mathstudy/backend-go/internal/application/mistake"
	"mathstudy/backend-go/internal/domain/user"
)

func TestListRequiresBearerToken(t *testing.T) {
	handler := newTestHandler(t, &fakeMistakeService{}, &fakeAuthenticator{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/mistakes")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/mistakes", nil)
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["detail"] != "未认证，请先登录" || body["code"] != "UNAUTHORIZED" {
		t.Fatalf("body = %#v", body)
	}
}

func TestListForwardsQueryParameters(t *testing.T) {
	service := &fakeMistakeService{
		listResponse: mistakeapp.MistakeListResponse{
			Items: []mistakeapp.MistakeItem{{ID: "attempt-1"}},
			Pagination: mistakeapp.PaginationInfo{
				Page: 2, PageSize: 10, Total: 1, TotalPages: 1,
			},
		},
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/mistakes")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/mistakes?page=2&page_size=10&error_type=conceptual&concept_id=limit&difficulty_min=0.2&difficulty_max=0.8&date_from=2026-04-01T00:00:00&date_to=2026-04-25T23:00:00&mastery_status=weak&sort_by=mastery&sort_order=asc", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if service.lastUserID != "student-1" {
		t.Fatalf("userID = %q", service.lastUserID)
	}
	query := service.lastListQuery
	if query.Page != 2 || query.PageSize != 10 || query.ErrorType != "conceptual" || query.ConceptID != "limit" {
		t.Fatalf("query = %#v", query)
	}
	if query.DifficultyMin != 0.2 || query.DifficultyMax != 0.8 || query.MasteryStatus != "weak" || query.SortBy != "mastery" || query.SortOrder != "asc" {
		t.Fatalf("query = %#v", query)
	}
	if query.DateFrom == nil || query.DateTo == nil {
		t.Fatalf("dates = %#v %#v", query.DateFrom, query.DateTo)
	}
}

func TestListRejectsInvalidPagination(t *testing.T) {
	service := &fakeMistakeService{}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/mistakes")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/mistakes?page_size=bad", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["detail"] != "page_size 必须是整数" || body["code"] != "VALIDATION_ERROR" {
		t.Fatalf("body = %#v", body)
	}
	if service.lastUserID != "" {
		t.Fatalf("service was called for invalid pagination: user=%q query=%#v", service.lastUserID, service.lastListQuery)
	}
}

func TestListRejectsInvalidDate(t *testing.T) {
	service := &fakeMistakeService{}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/mistakes")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/mistakes?date_from=not-a-date", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["detail"] != "开始时间格式错误，请使用 ISO 8601 格式" {
		t.Fatalf("body = %#v", body)
	}
}

func TestDetailMapsNotFound(t *testing.T) {
	service := &fakeMistakeService{detailErr: mistakeapp.ErrNotFound}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/mistakes")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/mistakes/attempt-1", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if service.lastAttemptID != "attempt-1" {
		t.Fatalf("attemptID = %q", service.lastAttemptID)
	}
}

func TestReviewNextUsesLiteralRoute(t *testing.T) {
	service := &fakeMistakeService{
		reviewResponse: mistakeapp.ReviewExerciseResponse{
			Exercise: mistakeapp.ReviewExercise{ID: "content-1"},
			Context:  mistakeapp.ReviewContext{IsReview: true},
		},
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/mistakes")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/mistakes/review/next?focus_concept=limit&focus_error_type=logical", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !service.reviewCalled || service.lastFocusConcept != "limit" || service.lastFocusErrorType != "logical" {
		t.Fatalf("review call = %#v", service)
	}
	if service.detailCalled {
		t.Fatal("detail handler was called for /review/next")
	}
}

func TestMasterAndDeleteMapNotFound(t *testing.T) {
	service := &fakeMistakeService{masterErr: mistakeapp.ErrNotFound, deleteErr: mistakeapp.ErrNotFound}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/mistakes")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/mistakes/attempt-1/master", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("master status = %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodDelete, "/api/v1/mistakes/attempt-1", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("delete status = %d", recorder.Code)
	}
}

func TestMasterMapsMissingProfile(t *testing.T) {
	service := &fakeMistakeService{masterErr: mistakeapp.ErrProfileNotFound}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/mistakes")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/mistakes/attempt-1/master", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["message"] != "学生画像不存在" {
		t.Fatalf("message = %q", body["message"])
	}
}

func TestInternalErrorsRedactLogs(t *testing.T) {
	credentialErr := errors.New("mistake repo failed Authorization: Bearer mistake-secret token=query-token api_key=plain password=letmein")
	tests := []struct {
		name    string
		method  string
		path    string
		service fakeMistakeService
	}{
		{
			name:    "list",
			method:  http.MethodGet,
			path:    "/api/v1/mistakes",
			service: fakeMistakeService{listErr: credentialErr},
		},
		{
			name:    "statistics",
			method:  http.MethodGet,
			path:    "/api/v1/mistakes/statistics",
			service: fakeMistakeService{statisticsErr: credentialErr},
		},
		{
			name:    "detail",
			method:  http.MethodGet,
			path:    "/api/v1/mistakes/attempt-1",
			service: fakeMistakeService{detailErr: credentialErr},
		},
		{
			name:    "mark mastered",
			method:  http.MethodPost,
			path:    "/api/v1/mistakes/attempt-1/master",
			service: fakeMistakeService{masterErr: credentialErr},
		},
		{
			name:    "delete",
			method:  http.MethodDelete,
			path:    "/api/v1/mistakes/attempt-1",
			service: fakeMistakeService{deleteErr: credentialErr},
		},
		{
			name:    "review next",
			method:  http.MethodGet,
			path:    "/api/v1/mistakes/review/next?focus_concept=limit&focus_error_type=logical",
			service: fakeMistakeService{reviewErr: credentialErr},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logBuffer bytes.Buffer
			service := tt.service
			handler, err := NewHandler(slog.New(slog.NewTextHandler(&logBuffer, nil)), &service, &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}})
			if err != nil {
				t.Fatalf("NewHandler() error = %v", err)
			}
			mux := http.NewServeMux()
			handler.Register(mux, "/api/v1/mistakes")

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)
			request.Header.Set("Authorization", "Bearer token")
			mux.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if logBuffer.Len() == 0 {
				t.Fatal("expected internal error log")
			}
			assertNoMistakeCredentialLeak(t, recorder.Body.String())
			assertNoMistakeCredentialLeak(t, logBuffer.String())
		})
	}
}

func TestNewHandlerRejectsMissingDependencies(t *testing.T) {
	if _, err := NewHandler(nil, nil, &fakeAuthenticator{}); err == nil {
		t.Fatal("NewHandler(nil service) error = nil, want error")
	}
	if _, err := NewHandler(nil, &fakeMistakeService{}, nil); err == nil {
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

func assertNoMistakeCredentialLeak(t *testing.T, value string) {
	t.Helper()
	for _, leaked := range []string{"mistake-secret", "token=query-token", "api_key=plain", "password=letmein", "Bearer mistake-secret"} {
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

type fakeMistakeService struct {
	listResponse       mistakeapp.MistakeListResponse
	statisticsResponse mistakeapp.StatisticsResponse
	detailResponse     mistakeapp.DetailResponse
	masterResponse     mistakeapp.MarkAsMasteredResponse
	deleteResponse     mistakeapp.DeleteResponse
	reviewResponse     mistakeapp.ReviewExerciseResponse
	listErr            error
	statisticsErr      error
	detailErr          error
	masterErr          error
	deleteErr          error
	reviewErr          error
	lastUserID         string
	lastListQuery      mistakeapp.ListQuery
	lastTimeRange      string
	lastAttemptID      string
	lastFocusConcept   string
	lastFocusErrorType string
	reviewCalled       bool
	detailCalled       bool
}

func (s *fakeMistakeService) GetMistakes(_ context.Context, userID string, query mistakeapp.ListQuery) (mistakeapp.MistakeListResponse, error) {
	s.lastUserID = userID
	s.lastListQuery = query
	return s.listResponse, s.listErr
}

func (s *fakeMistakeService) GetStatistics(_ context.Context, userID string, timeRange string) (mistakeapp.StatisticsResponse, error) {
	s.lastUserID = userID
	s.lastTimeRange = timeRange
	return s.statisticsResponse, s.statisticsErr
}

func (s *fakeMistakeService) GetMistakeDetail(_ context.Context, userID string, attemptID string) (mistakeapp.DetailResponse, error) {
	s.lastUserID = userID
	s.lastAttemptID = attemptID
	s.detailCalled = true
	return s.detailResponse, s.detailErr
}

func (s *fakeMistakeService) MarkAsMastered(_ context.Context, userID string, attemptID string) (mistakeapp.MarkAsMasteredResponse, error) {
	s.lastUserID = userID
	s.lastAttemptID = attemptID
	return s.masterResponse, s.masterErr
}

func (s *fakeMistakeService) DeleteMistake(_ context.Context, userID string, attemptID string) (mistakeapp.DeleteResponse, error) {
	s.lastUserID = userID
	s.lastAttemptID = attemptID
	return s.deleteResponse, s.deleteErr
}

func (s *fakeMistakeService) GetReviewExercise(_ context.Context, userID string, focusConcept string, focusErrorType string) (mistakeapp.ReviewExerciseResponse, error) {
	s.lastUserID = userID
	s.lastFocusConcept = focusConcept
	s.lastFocusErrorType = focusErrorType
	s.reviewCalled = true
	return s.reviewResponse, s.reviewErr
}
