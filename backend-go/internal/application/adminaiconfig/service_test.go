package adminaiconfig

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCreateProviderWithModelsEncryptsKeyAndCreatesDefaults(t *testing.T) {
	repo := &fakeRepo{}
	service := newTestService(t, repo)

	response, err := service.CreateProviderWithModels(context.Background(), CreateProviderWithModelsRequest{
		Name:    " DeepSeek ",
		Code:    "DeepSeek",
		BaseURL: "https://api.deepseek.com/v1/",
		APIKey:  "secret-key",
		Models: []ModelCreateSimple{
			{ModelID: "deepseek-chat"},
			{ModelID: "deepseek-chat"},
			{ModelID: "deepseek-reasoner"},
		},
	})
	if err != nil {
		t.Fatalf("CreateProviderWithModels() error = %v", err)
	}
	if response.Provider.ID != "id-1" || response.Provider.Code != "deepseek" || response.Provider.BaseURL != "https://api.deepseek.com/v1" {
		t.Fatalf("provider = %#v", response.Provider)
	}
	if repo.createdProvider.EncryptedAPIKey != "enc:secret-key" {
		t.Fatalf("encrypted key = %q", repo.createdProvider.EncryptedAPIKey)
	}
	if len(response.Models) != 2 || !response.Models[0].IsDefault || response.Models[0].DefaultTimeout != defaultTimeout {
		t.Fatalf("models = %#v", response.Models)
	}
}

func TestUpdateAgentConfigValidatesModelAndBuildsRuntimeConfig(t *testing.T) {
	temp := 0.2
	maxTokens := 800
	timeout := 30
	retries := 3
	repo := &fakeRepo{
		providers: map[string]StoredProvider{
			"provider-1": {LLMProvider: LLMProvider{ID: "provider-1", Name: "DeepSeek", Code: "deepseek", BaseURL: "https://api.deepseek.com", IsActive: true}, EncryptedAPIKey: "enc:secret"},
		},
		models: map[string]LLMModel{
			"model-1": {ID: "model-1", ProviderID: "provider-1", ModelID: "deepseek-chat", Name: "DeepSeek Chat", DefaultTemperature: 0.7, DefaultTimeout: 60, DefaultMaxRetries: 2, IsActive: true},
		},
	}
	service := newTestService(t, repo)

	config, err := service.UpdateAgentConfig(context.Background(), "tutor", UpdateAgentConfigRequest{
		ModelID:             "model-1",
		TemperatureOverride: &temp,
		MaxTokensOverride:   &maxTokens,
		TimeoutOverride:     &timeout,
		MaxRetriesOverride:  &retries,
	})
	if err != nil {
		t.Fatalf("UpdateAgentConfig() error = %v", err)
	}
	if config.AgentType != "tutor" || config.ModelID == nil || *config.ModelID != "model-1" {
		t.Fatalf("config = %#v", config)
	}
	runtime, ok, err := service.RuntimeConfig(context.Background(), "tutor")
	if err != nil || !ok {
		t.Fatalf("RuntimeConfig() ok=%v error=%v", ok, err)
	}
	if runtime.Model != "deepseek-chat" || runtime.APIKey != "secret" || runtime.Temperature != 0.2 || runtime.MaxTokens != 800 || runtime.Timeout != 30*time.Second || runtime.MaxIterations != 4 {
		t.Fatalf("runtime = %#v", runtime)
	}
}

func TestFetchModelsByCredentialsReadsOpenAICompatibleList(t *testing.T) {
	service := newTestService(t, &fakeRepo{})
	service.httpClient = fakeHTTPDoer(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.example.com/v1/models" {
			t.Fatalf("url = %s", req.URL.String())
		}
		if req.Header.Get("Authorization") != "Bearer key" {
			t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
		}
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"b"},{"id":"a"},{"id":"a"}]}`)),
		}, nil
	})

	response, err := service.FetchModelsByCredentials(context.Background(), FetchModelsByCredentialsRequest{BaseURL: "https://api.example.com", APIKey: "key"})
	if err != nil {
		t.Fatalf("FetchModelsByCredentials() error = %v", err)
	}
	if !response.Success || strings.Join(response.Models, ",") != "a,b" {
		t.Fatalf("response = %#v", response)
	}
}

func TestFetchModelsByCredentialsRejectsTrailingResponseData(t *testing.T) {
	service := newTestService(t, &fakeRepo{})
	service.httpClient = fakeHTTPDoer(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"a"}]} {"data":[{"id":"b"}]}`)),
			Header:     http.Header{},
		}, nil
	})

	response, err := service.FetchModelsByCredentials(context.Background(), FetchModelsByCredentialsRequest{BaseURL: "https://api.example.com", APIKey: "key"})
	if err != nil {
		t.Fatalf("FetchModelsByCredentials() error = %v", err)
	}
	if response.Success || response.Message != "模型列表响应格式无效" {
		t.Fatalf("response = %#v", response)
	}
}

func TestFetchModelsAndProviderTestRedactTransportErrors(t *testing.T) {
	repo := &fakeRepo{
		providers: map[string]StoredProvider{
			"provider-1": {LLMProvider: LLMProvider{ID: "provider-1", BaseURL: "https://api.example.com", IsActive: true}, EncryptedAPIKey: "enc:provider-secret"},
		},
	}
	service := newTestService(t, repo)
	service.httpClient = fakeHTTPDoer(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("upstream rejected Authorization: Bearer provider-secret api_key=plain url=https://api.example.com/v1/models?token=query-token")
	})

	fetchResponse, err := service.FetchModelsByCredentials(context.Background(), FetchModelsByCredentialsRequest{BaseURL: "https://api.example.com", APIKey: "plain"})
	if err != nil {
		t.Fatalf("FetchModelsByCredentials() error = %v", err)
	}
	assertNoCredentialLeak(t, fetchResponse.Message)

	testResponse, err := service.TestProvider(context.Background(), "provider-1", "deepseek-chat")
	if err != nil {
		t.Fatalf("TestProvider() error = %v", err)
	}
	if testResponse.Success {
		t.Fatalf("TestProvider() success = true, want false")
	}
	assertNoCredentialLeak(t, testResponse.Message)
}

func TestProviderBaseURLRejectsSSRFAndCredentialBoundaries(t *testing.T) {
	service := newTestService(t, &fakeRepo{})
	cases := []string{
		"http://api.example.com",
		"https://localhost:11434",
		"https://127.0.0.1:11434",
		"https://10.0.0.4/v1",
		"https://169.254.169.254/latest/meta-data",
		"https://user:pass@api.example.com",
		"https://api.example.com/v1?target=internal",
		"https://api.example.com/v1#fragment",
	}
	for _, baseURL := range cases {
		t.Run(baseURL, func(t *testing.T) {
			_, err := service.FetchModelsByCredentials(context.Background(), FetchModelsByCredentialsRequest{BaseURL: baseURL, APIKey: "key"})
			if !errors.Is(err, ErrBadRequest) {
				t.Fatalf("FetchModelsByCredentials(%q) error = %v, want ErrBadRequest", baseURL, err)
			}
		})
	}
}

func TestValidationRejectsInvalidProviderAndAgentInputs(t *testing.T) {
	service := newTestService(t, &fakeRepo{})
	_, err := service.CreateProvider(context.Background(), CreateProviderRequest{Name: "", Code: "openai", BaseURL: "bad", APIKey: ""})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("CreateProvider() error = %v, want ErrBadRequest", err)
	}

	_, err = service.UpdateAgentConfig(context.Background(), "unknown", UpdateAgentConfigRequest{ModelID: "model-1"})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("UpdateAgentConfig(unknown) error = %v, want ErrBadRequest", err)
	}
}

func assertNoCredentialLeak(t *testing.T, value string) {
	t.Helper()
	for _, leaked := range []string{"provider-secret", "api_key=plain", "token=query-token", "Bearer provider-secret"} {
		if strings.Contains(value, leaked) {
			t.Fatalf("value leaked %q in %q", leaked, value)
		}
	}
	if !strings.Contains(value, "[REDACTED]") {
		t.Fatalf("value = %q, want redaction marker", value)
	}
}

func newTestService(t *testing.T, repo *fakeRepo) *Service {
	t.Helper()
	if repo.providers == nil {
		repo.providers = map[string]StoredProvider{}
	}
	if repo.models == nil {
		repo.models = map[string]LLMModel{}
	}
	if repo.agentConfigs == nil {
		repo.agentConfigs = map[string]AgentModelConfig{}
	}
	service, err := NewService(repo, fakeCipher{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	service.now = func() time.Time { return time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC) }
	service.newID = sequentialIDs("id-1", "id-2", "id-3", "id-4")
	return service
}

func sequentialIDs(values ...string) func() (string, error) {
	index := 0
	return func() (string, error) {
		if index >= len(values) {
			return "extra-id", nil
		}
		value := values[index]
		index++
		return value, nil
	}
}

type fakeCipher struct{}

func (fakeCipher) Encrypt(value string) (string, error) { return "enc:" + value, nil }
func (fakeCipher) Decrypt(value string) (string, error) {
	return strings.TrimPrefix(value, "enc:"), nil
}

type fakeHTTPDoer func(*http.Request) (*http.Response, error)

func (f fakeHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

type fakeRepo struct {
	providers       map[string]StoredProvider
	models          map[string]LLMModel
	agentConfigs    map[string]AgentModelConfig
	createdProvider ProviderInput
}

func (r *fakeRepo) ListProviders(context.Context, bool) ([]LLMProvider, error) {
	items := make([]LLMProvider, 0, len(r.providers))
	for _, provider := range r.providers {
		items = append(items, provider.LLMProvider)
	}
	return items, nil
}

func (r *fakeRepo) GetProvider(_ context.Context, id string) (StoredProvider, bool, error) {
	provider, ok := r.providers[id]
	return provider, ok, nil
}

func (r *fakeRepo) GetProviderByCode(_ context.Context, code string) (StoredProvider, bool, error) {
	for _, provider := range r.providers {
		if provider.Code == code {
			return provider, true, nil
		}
	}
	return StoredProvider{}, false, nil
}

func (r *fakeRepo) CreateProvider(_ context.Context, input ProviderInput, now time.Time) (LLMProvider, error) {
	r.createdProvider = input
	provider := LLMProvider{ID: input.ID, Name: input.Name, Code: input.Code, BaseURL: input.BaseURL, IsActive: input.IsActive, Description: input.Description, CreatedAt: now, UpdatedAt: now}
	r.providers[input.ID] = StoredProvider{LLMProvider: provider, EncryptedAPIKey: input.EncryptedAPIKey}
	return provider, nil
}

func (r *fakeRepo) UpdateProvider(context.Context, string, ProviderUpdate, time.Time) (LLMProvider, bool, error) {
	return LLMProvider{}, false, nil
}

func (r *fakeRepo) DeleteProvider(context.Context, string) (bool, error) { return false, nil }

func (r *fakeRepo) ListModels(_ context.Context, filter ModelFilter) ([]LLMModel, error) {
	items := []LLMModel{}
	for _, model := range r.models {
		if filter.ProviderID == "" || model.ProviderID == filter.ProviderID {
			items = append(items, model)
		}
	}
	return items, nil
}

func (r *fakeRepo) GetModel(_ context.Context, id string) (LLMModel, bool, error) {
	model, ok := r.models[id]
	return model, ok, nil
}

func (r *fakeRepo) CreateModel(_ context.Context, input ModelInput, now time.Time) (LLMModel, error) {
	model := modelFromInput(input, now)
	r.models[model.ID] = model
	return model, nil
}

func (r *fakeRepo) UpdateModel(context.Context, string, ModelUpdate, time.Time) (LLMModel, bool, error) {
	return LLMModel{}, false, nil
}

func (r *fakeRepo) DeleteModel(context.Context, string) (bool, error) { return false, nil }

func (r *fakeRepo) SetDefaultModel(context.Context, string, time.Time) (bool, error) {
	return false, nil
}

func (r *fakeRepo) ReplaceProviderModels(_ context.Context, providerID string, inputs []ModelInput, now time.Time) (ModelsUpdateResult, error) {
	models := make([]LLMModel, 0, len(inputs))
	for _, input := range inputs {
		model := modelFromInput(input, now)
		r.models[model.ID] = model
		models = append(models, model)
	}
	return ModelsUpdateResult{Added: len(models), Models: models}, nil
}

func (r *fakeRepo) ListAgentConfigs(context.Context) ([]AgentModelConfig, error) {
	items := []AgentModelConfig{}
	for _, config := range r.agentConfigs {
		items = append(items, config)
	}
	return items, nil
}

func (r *fakeRepo) GetAgentConfig(_ context.Context, agentType string) (AgentModelConfig, bool, error) {
	config, ok := r.agentConfigs[agentType]
	return config, ok, nil
}

func (r *fakeRepo) UpsertAgentConfig(_ context.Context, input AgentConfigInput, now time.Time) (AgentModelConfig, error) {
	config := AgentModelConfig{
		ID:                  input.ID,
		AgentType:           input.AgentType,
		ModelID:             &input.ModelID,
		TemperatureOverride: input.TemperatureOverride,
		MaxTokensOverride:   input.MaxTokensOverride,
		TopPOverride:        input.TopPOverride,
		TimeoutOverride:     input.TimeoutOverride,
		MaxRetriesOverride:  input.MaxRetriesOverride,
		ExtraConfig:         input.ExtraConfig,
		IsActive:            input.IsActive,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	r.agentConfigs[input.AgentType] = config
	return config, nil
}

func (r *fakeRepo) DeleteAgentConfig(context.Context, string) (bool, error) { return false, nil }

func modelFromInput(input ModelInput, now time.Time) LLMModel {
	return LLMModel{
		ID:                 input.ID,
		ProviderID:         input.ProviderID,
		Name:               input.Name,
		ModelID:            input.ModelID,
		DefaultTemperature: input.DefaultTemperature,
		DefaultMaxTokens:   input.DefaultMaxTokens,
		DefaultTopP:        input.DefaultTopP,
		DefaultTimeout:     input.DefaultTimeout,
		DefaultMaxRetries:  input.DefaultMaxRetries,
		IsActive:           input.IsActive,
		IsDefault:          input.IsDefault,
		Capabilities:       input.Capabilities,
		Description:        input.Description,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}
