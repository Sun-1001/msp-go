package sessionhttp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	authapp "mathstudy/backend-go/internal/application/auth"
	sessionapp "mathstudy/backend-go/internal/application/session"
	"mathstudy/backend-go/internal/domain/user"
)

func TestStartRequiresBearerToken(t *testing.T) {
	handler := newTestHandler(t, &fakeSessionService{}, &fakeAuthenticator{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/session")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/session/start", strings.NewReader(`{}`))
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
}

func TestStartForwardsRequest(t *testing.T) {
	service := &fakeSessionService{
		createResponse: sessionapp.CreateSessionResponse{SessionID: "session-1", UserID: "student-1", Mode: "study", Status: "active"},
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/session")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/session/start", strings.NewReader(`{"topic":"极限","mode":"study"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if service.lastUserID != "student-1" || service.lastTopic == nil || *service.lastTopic != "极限" || service.lastMode != "study" {
		t.Fatalf("service = %#v", service)
	}
}

func TestChatWritesSSEEvents(t *testing.T) {
	service := &fakeSessionService{
		chatResult: sessionapp.ChatResult{TaskID: "task-1", MessageID: "msg-1", Agent: "tutor", Content: "hello"},
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/session")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/session/session-1/chat", strings.NewReader(`{"message":"你好","attachments":["/uploads/a.png"]}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/event-stream") {
		t.Fatalf("content type = %q", contentType)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "event: task_info") || !strings.Contains(body, "event: message") || !strings.Contains(body, `"type":"done"`) {
		t.Fatalf("body = %s", body)
	}
	if service.lastSessionID != "session-1" || service.lastChatMessage != "你好" || len(service.lastAttachments) != 1 {
		t.Fatalf("service = %#v", service)
	}
}

func TestChatNotFoundWritesSSEError(t *testing.T) {
	service := &fakeSessionService{chatErr: sessionapp.ErrNotFound}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/session")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/session/session-1/chat", strings.NewReader(`{"message":"你好"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "SESSION_NOT_FOUND") {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestHistoryListAndCancelTaskForwardInputs(t *testing.T) {
	service := &fakeSessionService{
		historyResponse: sessionapp.HistoryResponse{Messages: []sessionapp.MessageResponse{}, Total: 0, HasMore: false},
		listResponse:    sessionapp.SessionListResponse{Sessions: []sessionapp.SessionResponse{}, Total: 0},
		cancelResponse:  sessionapp.CancelTaskResponse{Success: false, Message: "任务不存在或已完成"},
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/session")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/session/session-1/history?limit=10&offset=5", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastLimit != 10 || service.lastOffset != 5 {
		t.Fatalf("history status=%d service=%#v", recorder.Code, service)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/v1/session/list?limit=7&offset=3", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastLimit != 7 || service.lastOffset != 3 {
		t.Fatalf("list status=%d service=%#v", recorder.Code, service)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/session/task/task-1/cancel", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastTaskID != "task-1" {
		t.Fatalf("cancel status=%d service=%#v", recorder.Code, service)
	}
}

func TestEndModeDeleteAndBatchDelete(t *testing.T) {
	service := &fakeSessionService{
		endResponse:         sessionapp.EndResponse{Status: "ended", Message: "会话已成功结束"},
		updateModeResponse:  sessionapp.UpdateModeResponse{SessionID: "session-1", Mode: "explain"},
		deleteResponse:      sessionapp.DeleteResponse{Success: true, Message: "会话已删除"},
		batchDeleteResponse: sessionapp.BatchDeleteResponse{Success: true, DeletedCount: 2, Message: "成功删除 2 个会话"},
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/session")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/session/session-1/end", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastSessionID != "session-1" {
		t.Fatalf("end status=%d service=%#v", recorder.Code, service)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPatch, "/api/v1/session/session-1/mode", strings.NewReader(`{"mode":"explain"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastMode != "explain" {
		t.Fatalf("mode status=%d service=%#v", recorder.Code, service)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodDelete, "/api/v1/session/session-1", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("delete status=%d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/session/batch-delete", strings.NewReader(`{"session_ids":["a","b"]}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || len(service.lastSessionIDs) != 2 {
		t.Fatalf("batch status=%d service=%#v", recorder.Code, service)
	}
}

func TestNewHandlerRejectsMissingDependencies(t *testing.T) {
	if _, err := NewHandler(nil, nil, &fakeAuthenticator{}); err == nil {
		t.Fatal("NewHandler(nil service) error = nil, want error")
	}
	if _, err := NewHandler(nil, &fakeSessionService{}, nil); err == nil {
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

type fakeAuthenticator struct {
	principal authapp.Principal
}

func (a *fakeAuthenticator) DecodeAccessToken(string) (authapp.Principal, bool) {
	if a.principal.UserID == "" {
		return authapp.Principal{}, false
	}
	return a.principal, true
}

type fakeSessionService struct {
	createResponse      sessionapp.CreateSessionResponse
	chatResult          sessionapp.ChatResult
	historyResponse     sessionapp.HistoryResponse
	listResponse        sessionapp.SessionListResponse
	endResponse         sessionapp.EndResponse
	updateModeResponse  sessionapp.UpdateModeResponse
	deleteResponse      sessionapp.DeleteResponse
	batchDeleteResponse sessionapp.BatchDeleteResponse
	cancelResponse      sessionapp.CancelTaskResponse
	createErr           error
	chatErr             error
	historyErr          error
	listErr             error
	endErr              error
	modeErr             error
	deleteErr           error
	batchErr            error
	cancelErr           error
	lastUserID          string
	lastTopic           *string
	lastMode            string
	lastSessionID       string
	lastSessionIDs      []string
	lastChatMessage     string
	lastAttachments     []string
	lastLimit           int
	lastOffset          int
	lastTaskID          string
}

func (s *fakeSessionService) CreateSession(_ context.Context, userID string, topic *string, mode string) (sessionapp.CreateSessionResponse, error) {
	s.lastUserID = userID
	s.lastTopic = topic
	s.lastMode = mode
	return s.createResponse, s.createErr
}

func (s *fakeSessionService) ProcessChat(_ context.Context, sessionID string, userID string, message string, attachments []string) (sessionapp.ChatResult, error) {
	s.lastSessionID = sessionID
	s.lastUserID = userID
	s.lastChatMessage = message
	s.lastAttachments = attachments
	return s.chatResult, s.chatErr
}

func (s *fakeSessionService) GetHistory(_ context.Context, sessionID string, userID string, limit int, offset int) (sessionapp.HistoryResponse, error) {
	s.lastSessionID = sessionID
	s.lastUserID = userID
	s.lastLimit = limit
	s.lastOffset = offset
	return s.historyResponse, s.historyErr
}

func (s *fakeSessionService) GetSessions(_ context.Context, userID string, limit int, offset int) (sessionapp.SessionListResponse, error) {
	s.lastUserID = userID
	s.lastLimit = limit
	s.lastOffset = offset
	return s.listResponse, s.listErr
}

func (s *fakeSessionService) EndSession(_ context.Context, sessionID string, userID string) (sessionapp.EndResponse, error) {
	s.lastSessionID = sessionID
	s.lastUserID = userID
	return s.endResponse, s.endErr
}

func (s *fakeSessionService) UpdateSessionMode(_ context.Context, sessionID string, userID string, mode string) (sessionapp.UpdateModeResponse, error) {
	s.lastSessionID = sessionID
	s.lastUserID = userID
	s.lastMode = mode
	return s.updateModeResponse, s.modeErr
}

func (s *fakeSessionService) DeleteSession(_ context.Context, sessionID string, userID string) (sessionapp.DeleteResponse, error) {
	s.lastSessionID = sessionID
	s.lastUserID = userID
	return s.deleteResponse, s.deleteErr
}

func (s *fakeSessionService) BatchDeleteSessions(_ context.Context, sessionIDs []string, userID string) (sessionapp.BatchDeleteResponse, error) {
	s.lastSessionIDs = sessionIDs
	s.lastUserID = userID
	return s.batchDeleteResponse, s.batchErr
}

func (s *fakeSessionService) CancelTask(_ context.Context, taskID string, userID string) (sessionapp.CancelTaskResponse, error) {
	s.lastTaskID = taskID
	s.lastUserID = userID
	return s.cancelResponse, s.cancelErr
}

func decodeJSONBody(t *testing.T, body string) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return decoded
}
