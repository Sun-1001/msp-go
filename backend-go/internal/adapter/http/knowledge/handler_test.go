package knowledgehttp

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
	knowledgeapp "mathstudy/backend-go/internal/application/knowledge"
	"mathstudy/backend-go/internal/domain/user"
)

func TestKnowledgeRoutesRequireAdmin(t *testing.T) {
	handler := newKnowledgeTestHandler(t, &fakeKnowledgeService{}, &fakeAuthenticator{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/knowledge")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/knowledge/stats", nil)
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", got)
	}

	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}}
	handler = newKnowledgeTestHandler(t, &fakeKnowledgeService{}, auth)
	mux = http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/knowledge")
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/knowledge/stats", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestListNodesParsesFilters(t *testing.T) {
	service := &fakeKnowledgeService{nodeListResponse: knowledgeapp.NodeListResponse{Items: []knowledgeapp.KnowledgeNode{}, Page: 2, PageSize: 50}}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "admin-1", Role: user.RoleAdmin}}
	handler := newKnowledgeTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/knowledge")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/knowledge/nodes?page=2&page_size=50&type=concept&chapter=第一章&search=极限", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.lastFilter.Page != 2 || service.lastFilter.PageSize != 50 || service.lastFilter.NodeType != "concept" || service.lastFilter.Chapter != "第一章" {
		t.Fatalf("filter = %#v", service.lastFilter)
	}
}

func TestListNodesRejectsInvalidPagination(t *testing.T) {
	service := &fakeKnowledgeService{}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "admin-1", Role: user.RoleAdmin}}
	handler := newKnowledgeTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/knowledge")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/knowledge/nodes?page=0", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["detail"] != "page 必须大于等于 1" || body["code"] != "VALIDATION_ERROR" {
		t.Fatalf("body = %#v", body)
	}
	if service.lastFilter.Page != 0 {
		t.Fatalf("service was called for invalid pagination: %#v", service.lastFilter)
	}
}

func TestCreateNodeValidatesAndForwards(t *testing.T) {
	service := &fakeKnowledgeService{
		nodeResponse: knowledgeapp.NodeResponse{Success: true, Message: "创建成功", Node: &knowledgeapp.KnowledgeNode{ID: "node-1"}},
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "admin-1", Role: user.RoleAdmin}}
	handler := newKnowledgeTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/knowledge")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/knowledge/nodes", bytes.NewBufferString(`{"name":"","node_type":"concept"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/knowledge/nodes", bytes.NewBufferString(`{"name":"极限","node_type":"concept","difficulty":0.4,"tags":["基础"]}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.lastNodeInput.Name != "极限" || service.lastNodeInput.NodeType != "concept" || len(service.lastNodeInput.Tags) != 1 {
		t.Fatalf("input = %#v", service.lastNodeInput)
	}
}

func TestRelationRoutesForwardIDs(t *testing.T) {
	service := &fakeKnowledgeService{
		relationResponse: knowledgeapp.RelationResponse{Success: true, Message: "更新成功", Relation: &knowledgeapp.KnowledgeRelation{ID: "rel-1"}},
		deleteResponse:   knowledgeapp.DeleteResponse{Success: true, Message: "删除成功"},
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "admin-1", Role: user.RoleAdmin}}
	handler := newKnowledgeTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/knowledge")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/knowledge/relations/rel-1", bytes.NewBufferString(`{"relation_type":"used_in","weight":0.7}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastRelationID != "rel-1" {
		t.Fatalf("update status=%d relation=%q", recorder.Code, service.lastRelationID)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/knowledge/relations/rel-2", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastRelationID != "rel-2" {
		t.Fatalf("delete status=%d relation=%q", recorder.Code, service.lastRelationID)
	}
}

func TestJSONRoutesRejectTrailingJSON(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		assert func(*testing.T, *fakeKnowledgeService)
	}{
		{
			name:   "create node",
			method: http.MethodPost,
			path:   "/api/v1/admin/knowledge/nodes",
			body:   `{"name":"极限","node_type":"concept","difficulty":0.4} {"name":"extra"}`,
			assert: func(t *testing.T, service *fakeKnowledgeService) {
				t.Helper()
				if service.lastNodeInput.Name != "" {
					t.Fatalf("service was called for create node trailing JSON: %#v", service.lastNodeInput)
				}
			},
		},
		{
			name:   "update node",
			method: http.MethodPut,
			path:   "/api/v1/admin/knowledge/nodes/node-1",
			body:   `{"name":"极限"} {"name":"extra"}`,
			assert: func(t *testing.T, service *fakeKnowledgeService) {
				t.Helper()
				if service.lastNodeID != "" {
					t.Fatalf("service was called for update node trailing JSON: id=%q update=%#v", service.lastNodeID, service.lastNodeUpdate)
				}
			},
		},
		{
			name:   "create relation",
			method: http.MethodPost,
			path:   "/api/v1/admin/knowledge/relations",
			body:   `{"source_id":"node-1","target_id":"node-2","relation_type":"used_in"} {"source_id":"extra"}`,
			assert: func(t *testing.T, service *fakeKnowledgeService) {
				t.Helper()
				if service.lastRelationInput.SourceID != "" {
					t.Fatalf("service was called for create relation trailing JSON: %#v", service.lastRelationInput)
				}
			},
		},
		{
			name:   "update relation",
			method: http.MethodPut,
			path:   "/api/v1/admin/knowledge/relations/relation-1",
			body:   `{"relation_type":"used_in"} {"relation_type":"related_to"}`,
			assert: func(t *testing.T, service *fakeKnowledgeService) {
				t.Helper()
				if service.lastRelationID != "" {
					t.Fatalf("service was called for update relation trailing JSON: id=%q update=%#v", service.lastRelationID, service.lastRelationUpdate)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeKnowledgeService{}
			handler := newKnowledgeTestHandler(t, service, adminAuthenticator())
			mux := http.NewServeMux()
			handler.Register(mux, "/api/v1/admin/knowledge")

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			request.Header.Set("Authorization", "Bearer token")
			mux.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
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

func TestDeleteNodeMapsBadRequest(t *testing.T) {
	service := &fakeKnowledgeService{err: knowledgeapp.ErrBadRequest}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "admin-1", Role: user.RoleAdmin}}
	handler := newKnowledgeTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/knowledge")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/knowledge/nodes/node-1", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest || service.lastNodeID != "node-1" {
		t.Fatalf("status=%d node=%q body=%s", recorder.Code, service.lastNodeID, recorder.Body.String())
	}
}

func TestServiceErrorsRedactPublicMessages(t *testing.T) {
	badRequestErr := knowledgeapp.Error{Kind: knowledgeapp.ErrBadRequest, Message: "bad input Authorization: Bearer knowledge-secret token=query-token api_key=plain password=letmein"}
	notFoundErr := knowledgeapp.Error{Kind: knowledgeapp.ErrNotFound, Message: "missing Authorization: Bearer knowledge-secret token=query-token api_key=plain password=letmein"}
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		err        error
		wantStatus int
	}{
		{name: "list nodes bad request", method: http.MethodGet, path: "/api/v1/admin/knowledge/nodes", err: badRequestErr, wantStatus: http.StatusBadRequest},
		{name: "get node not found", method: http.MethodGet, path: "/api/v1/admin/knowledge/nodes/node-1", err: notFoundErr, wantStatus: http.StatusNotFound},
		{name: "create node bad request", method: http.MethodPost, path: "/api/v1/admin/knowledge/nodes", body: `{"name":"极限","node_type":"concept"}`, err: badRequestErr, wantStatus: http.StatusBadRequest},
		{name: "update node bad request", method: http.MethodPut, path: "/api/v1/admin/knowledge/nodes/node-1", body: `{"name":"极限"}`, err: badRequestErr, wantStatus: http.StatusBadRequest},
		{name: "update node not found", method: http.MethodPut, path: "/api/v1/admin/knowledge/nodes/node-1", body: `{"name":"极限"}`, err: notFoundErr, wantStatus: http.StatusBadRequest},
		{name: "delete node bad request", method: http.MethodDelete, path: "/api/v1/admin/knowledge/nodes/node-1", err: badRequestErr, wantStatus: http.StatusBadRequest},
		{name: "delete node not found", method: http.MethodDelete, path: "/api/v1/admin/knowledge/nodes/node-1", err: notFoundErr, wantStatus: http.StatusNotFound},
		{name: "create relation bad request", method: http.MethodPost, path: "/api/v1/admin/knowledge/relations", body: `{"source_id":"node-1","target_id":"node-2","relation_type":"used_in"}`, err: badRequestErr, wantStatus: http.StatusBadRequest},
		{name: "update relation bad request", method: http.MethodPut, path: "/api/v1/admin/knowledge/relations/relation-1", body: `{"relation_type":"used_in"}`, err: badRequestErr, wantStatus: http.StatusBadRequest},
		{name: "update relation not found", method: http.MethodPut, path: "/api/v1/admin/knowledge/relations/relation-1", body: `{"relation_type":"used_in"}`, err: notFoundErr, wantStatus: http.StatusBadRequest},
		{name: "delete relation not found", method: http.MethodDelete, path: "/api/v1/admin/knowledge/relations/relation-1", err: notFoundErr, wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeKnowledgeService{err: tt.err}
			handler := newKnowledgeTestHandler(t, service, adminAuthenticator())
			mux := http.NewServeMux()
			handler.Register(mux, "/api/v1/admin/knowledge")

			var body *bytes.Reader
			if tt.body == "" {
				body = bytes.NewReader(nil)
			} else {
				body = bytes.NewReader([]byte(tt.body))
			}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, body)
			request.Header.Set("Authorization", "Bearer token")
			mux.ServeHTTP(recorder, request)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
			}
			assertNoKnowledgeCredentialLeak(t, recorder.Body.String())
		})
	}
}

func TestInternalErrorsRedactLogs(t *testing.T) {
	var logBuffer bytes.Buffer
	service := &fakeKnowledgeService{err: errors.New("knowledge repo failed Authorization: Bearer knowledge-secret token=query-token api_key=plain password=letmein")}
	handler, err := NewHandler(slog.New(slog.NewTextHandler(&logBuffer, nil)), service, adminAuthenticator())
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/knowledge")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/knowledge/stats", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertNoKnowledgeCredentialLeak(t, recorder.Body.String())
	assertNoKnowledgeCredentialLeak(t, logBuffer.String())
}

func TestNewHandlerRejectsMissingDependencies(t *testing.T) {
	if _, err := NewHandler(nil, nil, &fakeAuthenticator{}); err == nil {
		t.Fatal("NewHandler(nil service) error = nil, want error")
	}
	if _, err := NewHandler(nil, &fakeKnowledgeService{}, nil); err == nil {
		t.Fatal("NewHandler(nil auth) error = nil, want error")
	}
}

func newKnowledgeTestHandler(t *testing.T, service Service, auth Authenticator) *Handler {
	t.Helper()
	handler, err := NewHandler(slog.New(slog.NewTextHandler(os.Stdout, nil)), service, auth)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return handler
}

func adminAuthenticator() *fakeAuthenticator {
	return &fakeAuthenticator{principal: authapp.Principal{UserID: "admin-1", Role: user.RoleAdmin}}
}

func assertNoKnowledgeCredentialLeak(t *testing.T, value string) {
	t.Helper()
	for _, leaked := range []string{"knowledge-secret", "token=query-token", "api_key=plain", "password=letmein", "Bearer knowledge-secret"} {
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

type fakeKnowledgeService struct {
	statsResponse        knowledgeapp.Stats
	chaptersResponse     []string
	nodeListResponse     knowledgeapp.NodeListResponse
	simpleNodesResponse  []knowledgeapp.SimpleNode
	node                 knowledgeapp.KnowledgeNode
	nodeResponse         knowledgeapp.NodeResponse
	deleteResponse       knowledgeapp.DeleteResponse
	relationListResponse knowledgeapp.RelationListResponse
	relationResponse     knowledgeapp.RelationResponse
	err                  error
	lastFilter           knowledgeapp.NodeFilter
	lastNodeID           string
	lastNodeInput        knowledgeapp.NodeInput
	lastNodeUpdate       knowledgeapp.NodeUpdate
	lastRelationID       string
	lastRelationInput    knowledgeapp.RelationInput
	lastRelationUpdate   knowledgeapp.RelationUpdate
}

func (s *fakeKnowledgeService) GetStats(context.Context) (knowledgeapp.Stats, error) {
	return s.statsResponse, s.err
}

func (s *fakeKnowledgeService) GetChapters(context.Context) ([]string, error) {
	return s.chaptersResponse, s.err
}

func (s *fakeKnowledgeService) ListNodes(_ context.Context, filter knowledgeapp.NodeFilter) (knowledgeapp.NodeListResponse, error) {
	s.lastFilter = filter
	return s.nodeListResponse, s.err
}

func (s *fakeKnowledgeService) GetAllNodesSimple(context.Context) ([]knowledgeapp.SimpleNode, error) {
	return s.simpleNodesResponse, s.err
}

func (s *fakeKnowledgeService) GetNode(_ context.Context, nodeID string) (knowledgeapp.KnowledgeNode, error) {
	s.lastNodeID = nodeID
	return s.node, s.err
}

func (s *fakeKnowledgeService) CreateNode(_ context.Context, input knowledgeapp.NodeInput) (knowledgeapp.NodeResponse, error) {
	s.lastNodeInput = input
	return s.nodeResponse, s.err
}

func (s *fakeKnowledgeService) UpdateNode(_ context.Context, nodeID string, update knowledgeapp.NodeUpdate) (knowledgeapp.NodeResponse, error) {
	s.lastNodeID = nodeID
	s.lastNodeUpdate = update
	return s.nodeResponse, s.err
}

func (s *fakeKnowledgeService) DeleteNode(_ context.Context, nodeID string) (knowledgeapp.DeleteResponse, error) {
	s.lastNodeID = nodeID
	return s.deleteResponse, s.err
}

func (s *fakeKnowledgeService) ListRelations(context.Context, string) (knowledgeapp.RelationListResponse, error) {
	return s.relationListResponse, s.err
}

func (s *fakeKnowledgeService) CreateRelation(_ context.Context, input knowledgeapp.RelationInput) (knowledgeapp.RelationResponse, error) {
	s.lastRelationInput = input
	return s.relationResponse, s.err
}

func (s *fakeKnowledgeService) UpdateRelation(_ context.Context, relationID string, update knowledgeapp.RelationUpdate) (knowledgeapp.RelationResponse, error) {
	s.lastRelationID = relationID
	s.lastRelationUpdate = update
	return s.relationResponse, s.err
}

func (s *fakeKnowledgeService) DeleteRelation(_ context.Context, relationID string) (knowledgeapp.DeleteResponse, error) {
	s.lastRelationID = relationID
	return s.deleteResponse, s.err
}
