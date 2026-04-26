package resourcehttp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	authapp "mathstudy/backend-go/internal/application/auth"
	resourceapp "mathstudy/backend-go/internal/application/resource"
	"mathstudy/backend-go/internal/domain/user"
)

func TestListRequiresBearerToken(t *testing.T) {
	handler := newTestHandler(t, &fakeResourceService{}, &fakeAuthenticator{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/resources")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/resources", nil)
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
	service := &fakeResourceService{
		listResponse: resourceapp.ListResponse{Items: []resourceapp.Resource{{ID: "resource-1"}}, Total: 1, Page: 2, PageSize: 10},
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/resources")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/resources?type=video&chapter=chapter-1&topic=topic-1&search=limit&favorites_only=true&page=2&page_size=10", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if service.lastUserID != "student-1" {
		t.Fatalf("userID = %q", service.lastUserID)
	}
	filter := service.lastFilter
	if filter.Type != "video" || filter.Chapter != "chapter-1" || filter.Topic != "topic-1" || filter.Search != "limit" || !filter.FavoritesOnly {
		t.Fatalf("filter = %#v", filter)
	}
	if filter.Page != 2 || filter.PageSize != 10 {
		t.Fatalf("pagination = %#v", filter)
	}
}

func TestCreateRequiresTeacherRole(t *testing.T) {
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, &fakeResourceService{}, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/resources")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/resources", bytes.NewBufferString(`{"title":"资源","type":"document"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestCreateValidatesDefaultsAndReturnsCreated(t *testing.T) {
	service := &fakeResourceService{createResponse: resourceapp.Resource{ID: "resource-1", Title: "资源"}}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/resources")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/resources", bytes.NewBufferString(`{"title":"资源","type":"document"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if service.lastOwnerID != "teacher-1" || service.lastInput.Difficulty != 0.5 || service.lastInput.StorageType != "external" {
		t.Fatalf("input = %#v owner = %q", service.lastInput, service.lastOwnerID)
	}
	if service.lastInput.Tags == nil {
		t.Fatalf("tags = nil, want empty slice")
	}
}

func TestStatsAndFavoritesUseLiteralRoutes(t *testing.T) {
	service := &fakeResourceService{
		statsResponse:     resourceapp.Stats{Total: 3, Videos: 1, Documents: 2, Favorites: 1},
		favoritesResponse: resourceapp.ListResponse{Items: []resourceapp.Resource{{ID: "favorite-1"}}, Page: 1, PageSize: 20},
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "user-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/resources")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/resources/stats", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !service.statsCalled || service.detailCalled {
		t.Fatalf("stats status = %d statsCalled=%t detailCalled=%t", recorder.Code, service.statsCalled, service.detailCalled)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/v1/resources/favorites?page=2&page_size=5", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !service.favoritesCalled || service.detailCalled {
		t.Fatalf("favorites status = %d favoritesCalled=%t detailCalled=%t", recorder.Code, service.favoritesCalled, service.detailCalled)
	}
	if service.lastPage != 2 || service.lastPageSize != 5 {
		t.Fatalf("page = %d size = %d", service.lastPage, service.lastPageSize)
	}
}

func TestDetailAndFavoriteMapNotFound(t *testing.T) {
	service := &fakeResourceService{detailErr: resourceapp.ErrNotFound, favoriteErr: resourceapp.ErrNotFound}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "user-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/resources")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/resources/missing", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("detail status = %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/resources/missing/favorite", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("favorite status = %d", recorder.Code)
	}
}

func TestDeleteReturnsNoContent(t *testing.T) {
	service := &fakeResourceService{}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/resources")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/resources/resource-1", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if service.lastResourceID != "resource-1" || service.lastOwnerID != "teacher-1" {
		t.Fatalf("delete call resource=%q owner=%q", service.lastResourceID, service.lastOwnerID)
	}
}

func TestNewHandlerRejectsMissingDependencies(t *testing.T) {
	if _, err := NewHandler(nil, nil, &fakeAuthenticator{}); err == nil {
		t.Fatal("NewHandler(nil service) error = nil, want error")
	}
	if _, err := NewHandler(nil, &fakeResourceService{}, nil); err == nil {
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

type fakeResourceService struct {
	listResponse      resourceapp.ListResponse
	favoritesResponse resourceapp.ListResponse
	detailResponse    resourceapp.Resource
	createResponse    resourceapp.Resource
	updateResponse    resourceapp.Resource
	favoriteResponse  resourceapp.FavoriteToggleResponse
	statsResponse     resourceapp.Stats
	listErr           error
	favoritesErr      error
	detailErr         error
	createErr         error
	updateErr         error
	deleteErr         error
	favoriteErr       error
	statsErr          error
	lastUserID        string
	lastOwnerID       string
	lastResourceID    string
	lastFilter        resourceapp.ListFilter
	lastInput         resourceapp.ResourceInput
	lastUpdate        resourceapp.ResourceUpdate
	lastPage          int
	lastPageSize      int
	statsCalled       bool
	favoritesCalled   bool
	detailCalled      bool
}

func (s *fakeResourceService) GetResources(_ context.Context, userID string, filter resourceapp.ListFilter) (resourceapp.ListResponse, error) {
	s.lastUserID = userID
	s.lastFilter = filter
	return s.listResponse, s.listErr
}

func (s *fakeResourceService) GetFavorites(_ context.Context, userID string, page int, pageSize int) (resourceapp.ListResponse, error) {
	s.lastUserID = userID
	s.lastPage = page
	s.lastPageSize = pageSize
	s.favoritesCalled = true
	return s.favoritesResponse, s.favoritesErr
}

func (s *fakeResourceService) GetResource(_ context.Context, userID string, resourceID string) (resourceapp.Resource, error) {
	s.lastUserID = userID
	s.lastResourceID = resourceID
	s.detailCalled = true
	return s.detailResponse, s.detailErr
}

func (s *fakeResourceService) CreateResource(_ context.Context, ownerID string, input resourceapp.ResourceInput) (resourceapp.Resource, error) {
	s.lastOwnerID = ownerID
	s.lastInput = input
	return s.createResponse, s.createErr
}

func (s *fakeResourceService) UpdateResource(_ context.Context, resourceID string, ownerID string, input resourceapp.ResourceUpdate) (resourceapp.Resource, error) {
	s.lastResourceID = resourceID
	s.lastOwnerID = ownerID
	s.lastUpdate = input
	return s.updateResponse, s.updateErr
}

func (s *fakeResourceService) DeleteResource(_ context.Context, resourceID string, ownerID string) error {
	s.lastResourceID = resourceID
	s.lastOwnerID = ownerID
	return s.deleteErr
}

func (s *fakeResourceService) ToggleFavorite(_ context.Context, userID string, resourceID string) (resourceapp.FavoriteToggleResponse, error) {
	s.lastUserID = userID
	s.lastResourceID = resourceID
	return s.favoriteResponse, s.favoriteErr
}

func (s *fakeResourceService) GetStats(_ context.Context, userID string) (resourceapp.Stats, error) {
	s.lastUserID = userID
	s.statsCalled = true
	return s.statsResponse, s.statsErr
}
