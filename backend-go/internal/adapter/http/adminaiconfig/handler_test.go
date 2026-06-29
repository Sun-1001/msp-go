package adminaiconfighthttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	adminaiconfigapp "mathstudy/backend-go/internal/application/adminaiconfig"
	authapp "mathstudy/backend-go/internal/application/auth"
	"mathstudy/backend-go/internal/domain/user"
)

func TestRequiresAdmin(t *testing.T) {
	handler := newTestHandler(t, &fakeAIConfigService{}, &fakeAuthenticator{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/ai-config")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ai-config/providers", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}

	handler = newTestHandler(t, &fakeAIConfigService{}, &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}})
	mux = http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/ai-config")
	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/ai-config/providers", nil)
	request.Header.Set("Authorization", "Bearer token")
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestProviderAndModelRoutes(t *testing.T) {
	service := &fakeAIConfigService{
		provider: adminaiconfigapp.LLMProvider{ID: "provider-1", Name: "DeepSeek", Code: "deepseek", BaseURL: "https://api.deepseek.com", IsActive: true},
		model:    adminaiconfigapp.LLMModel{ID: "model-1", ProviderID: "provider-1", Name: "deepseek-chat", ModelID: "deepseek-chat", IsActive: true},
	}
	handler := newTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/ai-config")

	request := authedRequest(http.MethodPost, "/api/v1/admin/ai-config/providers/with-models", `{"name":"DeepSeek","code":"deepseek","base_url":"https://api.deepseek.com","api_key":"secret","models":[{"model_id":"deepseek-chat"}]}`)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated || service.lastProviderWithModels.Name != "DeepSeek" {
		t.Fatalf("status=%d request=%#v body=%s", recorder.Code, service.lastProviderWithModels, recorder.Body.String())
	}

	request = authedRequest(http.MethodPut, "/api/v1/admin/ai-config/providers/provider-1/models", `{"models":[{"model_id":"deepseek-chat"},{"model_id":"deepseek-reasoner"}]}`)
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastProviderID != "provider-1" || len(service.lastModels.Models) != 2 {
		t.Fatalf("status=%d provider=%q models=%#v", recorder.Code, service.lastProviderID, service.lastModels)
	}

	request = authedRequest(http.MethodPost, "/api/v1/admin/ai-config/models/model-1/set-default", ``)
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastModelID != "model-1" {
		t.Fatalf("status=%d model=%q", recorder.Code, service.lastModelID)
	}
}

func TestAgentRoutesAndFetchModels(t *testing.T) {
	service := &fakeAIConfigService{
		fetch: adminaiconfigapp.FetchModelsResponse{Success: true, Models: []string{"deepseek-chat"}, Message: "ok"},
		agent: adminaiconfigapp.AgentModelConfig{ID: "agent-1", AgentType: "tutor", IsActive: true},
	}
	handler := newTestHandler(t, service, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/ai-config")

	request := authedRequest(http.MethodPost, "/api/v1/admin/ai-config/channels/fetch-models", `{"base_url":"https://api.deepseek.com","api_key":"secret"}`)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastCredentials.BaseURL == "" {
		t.Fatalf("status=%d credentials=%#v", recorder.Code, service.lastCredentials)
	}

	request = authedRequest(http.MethodPut, "/api/v1/admin/ai-config/agents/tutor", `{"model_id":"model-1","temperature_override":0.2}`)
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastAgentType != "tutor" || service.lastAgentUpdate.ModelID != "model-1" {
		t.Fatalf("status=%d agent=%q update=%#v", recorder.Code, service.lastAgentType, service.lastAgentUpdate)
	}

	request = authedRequest(http.MethodGet, "/api/v1/admin/ai-config/agents/types", ``)
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestServiceErrors(t *testing.T) {
	handler := newTestHandler(t, &fakeAIConfigService{err: adminaiconfigapp.Error{Kind: adminaiconfigapp.ErrBadRequest, Message: "bad input"}}, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/ai-config")

	request := authedRequest(http.MethodGet, "/api/v1/admin/ai-config/providers", ``)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestNewHandlerRejectsMissingDependencies(t *testing.T) {
	if _, err := NewHandler(nil, nil, &fakeAuthenticator{}); err == nil {
		t.Fatal("NewHandler(nil service) error = nil, want error")
	}
	if _, err := NewHandler(nil, &fakeAIConfigService{}, nil); err == nil {
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

func authedRequest(method string, path string, body string) *http.Request {
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Authorization", "Bearer token")
	return request
}

func adminAuthenticator() *fakeAuthenticator {
	return &fakeAuthenticator{principal: authapp.Principal{UserID: "admin-1", Role: user.RoleAdmin}}
}

type fakeAuthenticator struct {
	principal authapp.Principal
}

func (a *fakeAuthenticator) DecodeAccessToken(string) (authapp.Principal, bool) {
	return a.principal, a.principal.UserID != ""
}

type fakeAIConfigService struct {
	provider               adminaiconfigapp.LLMProvider
	model                  adminaiconfigapp.LLMModel
	agent                  adminaiconfigapp.AgentModelConfig
	fetch                  adminaiconfigapp.FetchModelsResponse
	err                    error
	lastProviderID         string
	lastModelID            string
	lastAgentType          string
	lastProviderWithModels adminaiconfigapp.CreateProviderWithModelsRequest
	lastModels             adminaiconfigapp.ModelsUpdateRequest
	lastAgentUpdate        adminaiconfigapp.UpdateAgentConfigRequest
	lastCredentials        adminaiconfigapp.FetchModelsByCredentialsRequest
}

func (s *fakeAIConfigService) maybeErr() error {
	if s.err != nil {
		return s.err
	}
	return nil
}

func (s *fakeAIConfigService) ListProviders(context.Context, bool) (adminaiconfigapp.ListResponse[adminaiconfigapp.LLMProvider], error) {
	if err := s.maybeErr(); err != nil {
		return adminaiconfigapp.ListResponse[adminaiconfigapp.LLMProvider]{}, err
	}
	return adminaiconfigapp.ListResponse[adminaiconfigapp.LLMProvider]{Items: []adminaiconfigapp.LLMProvider{s.provider}, Total: 1}, nil
}

func (s *fakeAIConfigService) GetProvider(context.Context, string) (adminaiconfigapp.LLMProvider, error) {
	return s.provider, s.maybeErr()
}

func (s *fakeAIConfigService) CreateProvider(context.Context, adminaiconfigapp.CreateProviderRequest) (adminaiconfigapp.LLMProvider, error) {
	return s.provider, s.maybeErr()
}

func (s *fakeAIConfigService) CreateProviderWithModels(_ context.Context, request adminaiconfigapp.CreateProviderWithModelsRequest) (adminaiconfigapp.ProviderWithModelsResponse, error) {
	s.lastProviderWithModels = request
	return adminaiconfigapp.ProviderWithModelsResponse{Provider: s.provider, Models: []adminaiconfigapp.LLMModel{s.model}, ModelsCount: 1}, s.maybeErr()
}

func (s *fakeAIConfigService) UpdateProvider(context.Context, string, adminaiconfigapp.UpdateProviderRequest) (adminaiconfigapp.LLMProvider, error) {
	return s.provider, s.maybeErr()
}

func (s *fakeAIConfigService) DeleteProvider(context.Context, string) (adminaiconfigapp.SuccessResponse, error) {
	return adminaiconfigapp.SuccessResponse{Success: true}, s.maybeErr()
}

func (s *fakeAIConfigService) TestProvider(context.Context, string, string) (adminaiconfigapp.ProviderTestResult, error) {
	return adminaiconfigapp.ProviderTestResult{Success: true, Message: "ok"}, s.maybeErr()
}

func (s *fakeAIConfigService) FetchAvailableModels(context.Context, string) (adminaiconfigapp.FetchModelsResponse, error) {
	return s.fetch, s.maybeErr()
}

func (s *fakeAIConfigService) FetchModelsByCredentials(_ context.Context, request adminaiconfigapp.FetchModelsByCredentialsRequest) (adminaiconfigapp.FetchModelsResponse, error) {
	s.lastCredentials = request
	return s.fetch, s.maybeErr()
}

func (s *fakeAIConfigService) UpdateProviderModels(_ context.Context, providerID string, request adminaiconfigapp.ModelsUpdateRequest) (adminaiconfigapp.ModelsUpdateResult, error) {
	s.lastProviderID = providerID
	s.lastModels = request
	return adminaiconfigapp.ModelsUpdateResult{Models: []adminaiconfigapp.LLMModel{s.model}}, s.maybeErr()
}

func (s *fakeAIConfigService) ListModels(context.Context, string, bool) (adminaiconfigapp.ListResponse[adminaiconfigapp.LLMModel], error) {
	return adminaiconfigapp.ListResponse[adminaiconfigapp.LLMModel]{Items: []adminaiconfigapp.LLMModel{s.model}, Total: 1}, s.maybeErr()
}

func (s *fakeAIConfigService) GetModel(context.Context, string) (adminaiconfigapp.LLMModel, error) {
	return s.model, s.maybeErr()
}

func (s *fakeAIConfigService) CreateModel(context.Context, adminaiconfigapp.CreateModelRequest) (adminaiconfigapp.LLMModel, error) {
	return s.model, s.maybeErr()
}

func (s *fakeAIConfigService) UpdateModel(context.Context, string, adminaiconfigapp.UpdateModelRequest) (adminaiconfigapp.LLMModel, error) {
	return s.model, s.maybeErr()
}

func (s *fakeAIConfigService) DeleteModel(context.Context, string) (adminaiconfigapp.SuccessResponse, error) {
	return adminaiconfigapp.SuccessResponse{Success: true}, s.maybeErr()
}

func (s *fakeAIConfigService) SetDefaultModel(_ context.Context, modelID string) (adminaiconfigapp.SuccessResponse, error) {
	s.lastModelID = modelID
	return adminaiconfigapp.SuccessResponse{Success: true}, s.maybeErr()
}

func (s *fakeAIConfigService) ListAgentConfigs(context.Context) (adminaiconfigapp.ListResponse[adminaiconfigapp.AgentModelConfig], error) {
	return adminaiconfigapp.ListResponse[adminaiconfigapp.AgentModelConfig]{Items: []adminaiconfigapp.AgentModelConfig{s.agent}, Total: 1}, s.maybeErr()
}

func (s *fakeAIConfigService) ListAgentTypes(context.Context) (adminaiconfigapp.AgentTypesResponse, error) {
	return adminaiconfigapp.AgentTypesResponse{Items: []adminaiconfigapp.AgentTypeInfo{{Type: "tutor", Name: "导师智能体", Configured: true}}}, s.maybeErr()
}

func (s *fakeAIConfigService) GetAgentConfig(context.Context, string) (adminaiconfigapp.AgentModelConfig, error) {
	return s.agent, s.maybeErr()
}

func (s *fakeAIConfigService) UpdateAgentConfig(_ context.Context, agentType string, request adminaiconfigapp.UpdateAgentConfigRequest) (adminaiconfigapp.AgentModelConfig, error) {
	s.lastAgentType = agentType
	s.lastAgentUpdate = request
	return s.agent, s.maybeErr()
}

func (s *fakeAIConfigService) DeleteAgentConfig(context.Context, string) (adminaiconfigapp.SuccessResponse, error) {
	return adminaiconfigapp.SuccessResponse{Success: true}, s.maybeErr()
}

func TestErrorCodesSerialize(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeAIConfigError(recorder, http.StatusConflict, "CONFLICT", "冲突")
	var response map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if response["code"] != "CONFLICT" {
		t.Fatalf("response = %#v", response)
	}
}

func TestErrorMappingNotFound(t *testing.T) {
	handler := newTestHandler(t, &fakeAIConfigService{err: adminaiconfigapp.ErrNotFound}, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/ai-config")
	request := authedRequest(http.MethodGet, "/api/v1/admin/ai-config/models/model-1", ``)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestErrorMappingInternal(t *testing.T) {
	handler := newTestHandler(t, &fakeAIConfigService{err: errors.New("db down")}, adminAuthenticator())
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/admin/ai-config")
	request := authedRequest(http.MethodGet, "/api/v1/admin/ai-config/models", ``)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
