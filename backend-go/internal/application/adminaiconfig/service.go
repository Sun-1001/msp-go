package adminaiconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/outbound"
	"mathstudy/backend-go/internal/platform/ptrutil"
	"mathstudy/backend-go/internal/platform/redact"
)

const (
	defaultTemperature = 0.7
	defaultTimeout     = 60
	defaultMaxRetries  = 2
)

var (
	ErrBadRequest = errors.New("admin ai config bad request")
	ErrNotFound   = errors.New("admin ai config not found")
	ErrConflict   = errors.New("admin ai config conflict")
)

var agentTypeNames = map[string]string{
	"math_solver":        "数学求解智能体",
	"tutor":              "导师智能体",
	"diagnostician":      "诊断智能体",
	"portrait":           "学生画像",
	"question_parser":    "题目解析智能体",
	"question_generator": "题目生成智能体",
}

var orderedAgentTypes = []string{"math_solver", "tutor", "diagnostician", "portrait", "question_parser", "question_generator"}

// Repository is the persistence surface required by admin AI configuration.
type Repository interface {
	ListProviders(context.Context, bool) ([]LLMProvider, error)
	GetProvider(context.Context, string) (StoredProvider, bool, error)
	GetProviderByCode(context.Context, string) (StoredProvider, bool, error)
	CreateProvider(context.Context, ProviderInput, time.Time) (LLMProvider, error)
	UpdateProvider(context.Context, string, ProviderUpdate, time.Time) (LLMProvider, bool, error)
	DeleteProvider(context.Context, string) (bool, error)
	ListModels(context.Context, ModelFilter) ([]LLMModel, error)
	GetModel(context.Context, string) (LLMModel, bool, error)
	CreateModel(context.Context, ModelInput, time.Time) (LLMModel, error)
	UpdateModel(context.Context, string, ModelUpdate, time.Time) (LLMModel, bool, error)
	DeleteModel(context.Context, string) (bool, error)
	SetDefaultModel(context.Context, string, time.Time) (bool, error)
	ReplaceProviderModels(context.Context, string, []ModelInput, time.Time) (ModelsUpdateResult, error)
	ListAgentConfigs(context.Context) ([]AgentModelConfig, error)
	GetAgentConfig(context.Context, string) (AgentModelConfig, bool, error)
	UpsertAgentConfig(context.Context, AgentConfigInput, time.Time) (AgentModelConfig, error)
	DeleteAgentConfig(context.Context, string) (bool, error)
}

// Cipher protects stored provider API keys.
type Cipher interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}

// HTTPDoer is implemented by http.Client.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Service implements LLM provider, model, and agent configuration use cases.
type Service struct {
	repo       Repository
	cipher     Cipher
	httpClient HTTPDoer
	now        func() time.Time
	newID      func() (string, error)
}

// NewService creates an admin AI config service.
func NewService(repo Repository, cipher Cipher, clients ...HTTPDoer) (*Service, error) {
	if repo == nil {
		return nil, errors.New("admin ai config repository is nil")
	}
	if cipher == nil {
		return nil, errors.New("admin ai config cipher is nil")
	}
	var client HTTPDoer
	if len(clients) > 0 {
		client = clients[0]
	}
	if client == nil {
		client = outbound.NewPublicHTTPSClient(20 * time.Second)
	}
	return &Service{
		repo:       repo,
		cipher:     cipher,
		httpClient: client,
		now:        func() time.Time { return time.Now().UTC() },
		newID:      newUUID,
	}, nil
}

type LLMProvider struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Code        string    `json:"code"`
	BaseURL     string    `json:"base_url"`
	IsActive    bool      `json:"is_active"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type StoredProvider struct {
	LLMProvider
	EncryptedAPIKey string
}

type LLMModel struct {
	ID                 string         `json:"id"`
	ProviderID         string         `json:"provider_id"`
	Name               string         `json:"name"`
	ModelID            string         `json:"model_id"`
	DefaultTemperature float64        `json:"default_temperature"`
	DefaultMaxTokens   *int           `json:"default_max_tokens"`
	DefaultTopP        *float64       `json:"default_top_p"`
	DefaultTimeout     int            `json:"default_timeout"`
	DefaultMaxRetries  int            `json:"default_max_retries"`
	IsActive           bool           `json:"is_active"`
	IsDefault          bool           `json:"is_default"`
	Capabilities       map[string]any `json:"capabilities"`
	Description        *string        `json:"description"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	ProviderName       *string        `json:"provider_name"`
	ProviderCode       *string        `json:"provider_code"`
}

type AgentModelConfig struct {
	ID                  string         `json:"id"`
	AgentType           string         `json:"agent_type"`
	ModelID             *string        `json:"model_id"`
	TemperatureOverride *float64       `json:"temperature_override"`
	MaxTokensOverride   *int           `json:"max_tokens_override"`
	TopPOverride        *float64       `json:"top_p_override"`
	TimeoutOverride     *int           `json:"timeout_override"`
	MaxRetriesOverride  *int           `json:"max_retries_override"`
	ExtraConfig         map[string]any `json:"extra_config"`
	IsActive            bool           `json:"is_active"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	ModelName           *string        `json:"model_name"`
	ModelModelID        *string        `json:"model_model_id"`
	ProviderName        *string        `json:"provider_name"`
}

type AgentTypeInfo struct {
	Type       string `json:"type"`
	Name       string `json:"name"`
	Configured bool   `json:"configured"`
}

type AgentTypesResponse struct {
	Items []AgentTypeInfo `json:"items"`
}

type ListResponse[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type ProviderTestResult struct {
	Success   bool    `json:"success"`
	Message   string  `json:"message"`
	LatencyMS float64 `json:"latency_ms"`
	ModelID   *string `json:"model_id,omitempty"`
}

type FetchModelsResponse struct {
	Success bool     `json:"success"`
	Models  []string `json:"models"`
	Message string   `json:"message"`
}

type ProviderWithModelsResponse struct {
	Provider    LLMProvider `json:"provider"`
	Models      []LLMModel  `json:"models"`
	ModelsCount int         `json:"models_count"`
}

type ModelCreateSimple struct {
	ModelID string `json:"model_id"`
	Name    string `json:"name,omitempty"`
}

type ModelsUpdateResult struct {
	Added     int        `json:"added"`
	Removed   int        `json:"removed"`
	Unchanged int        `json:"unchanged"`
	Models    []LLMModel `json:"models"`
}

type ProviderInput struct {
	ID              string
	Name            string
	Code            string
	BaseURL         string
	EncryptedAPIKey string
	Description     *string
	IsActive        bool
}

type ProviderUpdate struct {
	Name            *string
	BaseURL         *string
	EncryptedAPIKey *string
	IsActive        *bool
	Description     *string
	DescriptionSet  bool
}

type ModelInput struct {
	ID                 string
	ProviderID         string
	Name               string
	ModelID            string
	DefaultTemperature float64
	DefaultMaxTokens   *int
	DefaultTopP        *float64
	DefaultTimeout     int
	DefaultMaxRetries  int
	Capabilities       map[string]any
	Description        *string
	IsActive           bool
	IsDefault          bool
}

type ModelUpdate struct {
	Name                *string
	ModelID             *string
	DefaultTemperature  *float64
	DefaultMaxTokens    *int
	DefaultMaxTokensSet bool
	DefaultTopP         *float64
	DefaultTopPSet      bool
	DefaultTimeout      *int
	DefaultMaxRetries   *int
	IsActive            *bool
	Capabilities        map[string]any
	CapabilitiesSet     bool
	Description         *string
	DescriptionSet      bool
}

type ModelFilter struct {
	ProviderID      string
	IncludeInactive bool
}

type AgentConfigInput struct {
	ID                  string
	AgentType           string
	ModelID             string
	TemperatureOverride *float64
	MaxTokensOverride   *int
	TopPOverride        *float64
	TimeoutOverride     *int
	MaxRetriesOverride  *int
	ExtraConfig         map[string]any
	IsActive            bool
}

type CreateProviderRequest struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	BaseURL     string `json:"base_url"`
	APIKey      string `json:"api_key"`
	Description string `json:"description"`
}

type CreateProviderWithModelsRequest struct {
	Name        string              `json:"name"`
	Code        string              `json:"code"`
	BaseURL     string              `json:"base_url"`
	APIKey      string              `json:"api_key"`
	Description string              `json:"description"`
	Models      []ModelCreateSimple `json:"models"`
}

type UpdateProviderRequest struct {
	Name        *string `json:"name"`
	BaseURL     *string `json:"base_url"`
	APIKey      *string `json:"api_key"`
	IsActive    *bool   `json:"is_active"`
	Description *string `json:"description"`
}

type CreateModelRequest struct {
	ProviderID         string         `json:"provider_id"`
	Name               string         `json:"name"`
	ModelID            string         `json:"model_id"`
	DefaultTemperature *float64       `json:"default_temperature"`
	DefaultMaxTokens   *int           `json:"default_max_tokens"`
	DefaultTopP        *float64       `json:"default_top_p"`
	DefaultTimeout     *int           `json:"default_timeout"`
	DefaultMaxRetries  *int           `json:"default_max_retries"`
	Capabilities       map[string]any `json:"capabilities"`
	Description        string         `json:"description"`
}

type UpdateModelRequest struct {
	Name               *string         `json:"name"`
	ModelID            *string         `json:"model_id"`
	DefaultTemperature *float64        `json:"default_temperature"`
	DefaultMaxTokens   *int            `json:"default_max_tokens"`
	DefaultTopP        *float64        `json:"default_top_p"`
	DefaultTimeout     *int            `json:"default_timeout"`
	DefaultMaxRetries  *int            `json:"default_max_retries"`
	IsActive           *bool           `json:"is_active"`
	Capabilities       *map[string]any `json:"capabilities"`
	Description        *string         `json:"description"`
}

type ModelsUpdateRequest struct {
	Models []ModelCreateSimple `json:"models"`
}

type UpdateAgentConfigRequest struct {
	ModelID             string         `json:"model_id"`
	TemperatureOverride *float64       `json:"temperature_override"`
	MaxTokensOverride   *int           `json:"max_tokens_override"`
	TopPOverride        *float64       `json:"top_p_override"`
	TimeoutOverride     *int           `json:"timeout_override"`
	MaxRetriesOverride  *int           `json:"max_retries_override"`
	ExtraConfig         map[string]any `json:"extra_config"`
}

type FetchModelsByCredentialsRequest struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

// RuntimeConfig stores the provider/model settings used by an agent runtime.
type RuntimeConfig struct {
	ProviderCode  string
	ProviderName  string
	BaseURL       string
	APIKey        string
	Model         string
	Temperature   float64
	MaxTokens     int
	TopP          *float64
	Timeout       time.Duration
	MaxRetries    int
	MaxIterations int
}

func (s *Service) ListProviders(ctx context.Context, includeInactive bool) (ListResponse[LLMProvider], error) {
	items, err := s.repo.ListProviders(ctx, includeInactive)
	if err != nil {
		return ListResponse[LLMProvider]{}, err
	}
	return ListResponse[LLMProvider]{Items: items, Total: len(items)}, nil
}

func (s *Service) GetProvider(ctx context.Context, providerID string) (LLMProvider, error) {
	provider, ok, err := s.repo.GetProvider(ctx, strings.TrimSpace(providerID))
	if err != nil {
		return LLMProvider{}, err
	}
	if !ok {
		return LLMProvider{}, ErrNotFound
	}
	return provider.LLMProvider, nil
}

func (s *Service) CreateProvider(ctx context.Context, request CreateProviderRequest) (LLMProvider, error) {
	input, err := s.providerInput(request)
	if err != nil {
		return LLMProvider{}, err
	}
	return s.repo.CreateProvider(ctx, input, s.now())
}

func (s *Service) CreateProviderWithModels(ctx context.Context, request CreateProviderWithModelsRequest) (ProviderWithModelsResponse, error) {
	if len(request.Models) == 0 || len(request.Models) > 100 {
		return ProviderWithModelsResponse{}, badRequest("models 长度必须在 1 到 100 之间")
	}
	input, err := s.providerInput(CreateProviderRequest{
		Name:        request.Name,
		Code:        request.Code,
		BaseURL:     request.BaseURL,
		APIKey:      request.APIKey,
		Description: request.Description,
	})
	if err != nil {
		return ProviderWithModelsResponse{}, err
	}
	inputs, err := s.modelInputs(input.ID, request.Models)
	if err != nil {
		return ProviderWithModelsResponse{}, err
	}
	provider, err := s.repo.CreateProvider(ctx, input, s.now())
	if err != nil {
		return ProviderWithModelsResponse{}, err
	}
	result, err := s.repo.ReplaceProviderModels(ctx, provider.ID, inputs, s.now())
	if err != nil {
		return ProviderWithModelsResponse{}, normalizeRepositoryError(err)
	}
	return ProviderWithModelsResponse{
		Provider:    provider,
		Models:      result.Models,
		ModelsCount: len(result.Models),
	}, nil
}

func (s *Service) UpdateProvider(ctx context.Context, providerID string, request UpdateProviderRequest) (LLMProvider, error) {
	update := ProviderUpdate{}
	if request.Name != nil {
		value := strings.TrimSpace(*request.Name)
		if value == "" || len([]rune(value)) > 100 {
			return LLMProvider{}, badRequest("name 长度必须在 1 到 100 之间")
		}
		update.Name = &value
	}
	if request.BaseURL != nil {
		value, err := normalizeBaseURL(*request.BaseURL)
		if err != nil {
			return LLMProvider{}, err
		}
		update.BaseURL = &value
	}
	if request.APIKey != nil && strings.TrimSpace(*request.APIKey) != "" {
		encrypted, err := s.cipher.Encrypt(strings.TrimSpace(*request.APIKey))
		if err != nil {
			return LLMProvider{}, fmt.Errorf("encrypt provider api key: %w", err)
		}
		update.EncryptedAPIKey = &encrypted
	}
	if request.IsActive != nil {
		update.IsActive = request.IsActive
	}
	if request.Description != nil {
		update.DescriptionSet = true
		update.Description = optionalTrimmedString(*request.Description, 500)
	}
	provider, ok, err := s.repo.UpdateProvider(ctx, strings.TrimSpace(providerID), update, s.now())
	if err != nil {
		return LLMProvider{}, normalizeRepositoryError(err)
	}
	if !ok {
		return LLMProvider{}, ErrNotFound
	}
	return provider, nil
}

func (s *Service) DeleteProvider(ctx context.Context, providerID string) (SuccessResponse, error) {
	ok, err := s.repo.DeleteProvider(ctx, strings.TrimSpace(providerID))
	if err != nil {
		return SuccessResponse{}, err
	}
	if !ok {
		return SuccessResponse{}, ErrNotFound
	}
	return SuccessResponse{Success: true, Message: "渠道已删除"}, nil
}

func (s *Service) ListModels(ctx context.Context, providerID string, includeInactive bool) (ListResponse[LLMModel], error) {
	items, err := s.repo.ListModels(ctx, ModelFilter{
		ProviderID:      strings.TrimSpace(providerID),
		IncludeInactive: includeInactive,
	})
	if err != nil {
		return ListResponse[LLMModel]{}, err
	}
	return ListResponse[LLMModel]{Items: items, Total: len(items)}, nil
}

func (s *Service) GetModel(ctx context.Context, modelID string) (LLMModel, error) {
	model, ok, err := s.repo.GetModel(ctx, strings.TrimSpace(modelID))
	if err != nil {
		return LLMModel{}, err
	}
	if !ok {
		return LLMModel{}, ErrNotFound
	}
	return model, nil
}

func (s *Service) CreateModel(ctx context.Context, request CreateModelRequest) (LLMModel, error) {
	input, err := s.modelInputFromRequest(request)
	if err != nil {
		return LLMModel{}, err
	}
	model, err := s.repo.CreateModel(ctx, input, s.now())
	if err != nil {
		return LLMModel{}, normalizeRepositoryError(err)
	}
	return model, nil
}

func (s *Service) UpdateModel(ctx context.Context, modelID string, request UpdateModelRequest) (LLMModel, error) {
	update, err := modelUpdateFromRequest(request)
	if err != nil {
		return LLMModel{}, err
	}
	model, ok, err := s.repo.UpdateModel(ctx, strings.TrimSpace(modelID), update, s.now())
	if err != nil {
		return LLMModel{}, normalizeRepositoryError(err)
	}
	if !ok {
		return LLMModel{}, ErrNotFound
	}
	return model, nil
}

func (s *Service) DeleteModel(ctx context.Context, modelID string) (SuccessResponse, error) {
	ok, err := s.repo.DeleteModel(ctx, strings.TrimSpace(modelID))
	if err != nil {
		return SuccessResponse{}, err
	}
	if !ok {
		return SuccessResponse{}, ErrNotFound
	}
	return SuccessResponse{Success: true, Message: "模型已删除"}, nil
}

func (s *Service) SetDefaultModel(ctx context.Context, modelID string) (SuccessResponse, error) {
	ok, err := s.repo.SetDefaultModel(ctx, strings.TrimSpace(modelID), s.now())
	if err != nil {
		return SuccessResponse{}, err
	}
	if !ok {
		return SuccessResponse{}, ErrNotFound
	}
	return SuccessResponse{Success: true, Message: "默认模型已更新"}, nil
}

func (s *Service) UpdateProviderModels(ctx context.Context, providerID string, request ModelsUpdateRequest) (ModelsUpdateResult, error) {
	if len(request.Models) > 100 {
		return ModelsUpdateResult{}, badRequest("models 长度不能超过 100")
	}
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return ModelsUpdateResult{}, ErrNotFound
	}
	if _, ok, err := s.repo.GetProvider(ctx, providerID); err != nil {
		return ModelsUpdateResult{}, err
	} else if !ok {
		return ModelsUpdateResult{}, ErrNotFound
	}
	inputs, err := s.modelInputs(providerID, request.Models)
	if err != nil {
		return ModelsUpdateResult{}, err
	}
	result, err := s.repo.ReplaceProviderModels(ctx, providerID, inputs, s.now())
	if err != nil {
		return ModelsUpdateResult{}, normalizeRepositoryError(err)
	}
	return result, nil
}

func (s *Service) ListAgentConfigs(ctx context.Context) (ListResponse[AgentModelConfig], error) {
	items, err := s.repo.ListAgentConfigs(ctx)
	if err != nil {
		return ListResponse[AgentModelConfig]{}, err
	}
	return ListResponse[AgentModelConfig]{Items: items, Total: len(items)}, nil
}

func (s *Service) ListAgentTypes(ctx context.Context) (AgentTypesResponse, error) {
	configs, err := s.repo.ListAgentConfigs(ctx)
	if err != nil {
		return AgentTypesResponse{}, err
	}
	configured := map[string]bool{}
	for _, config := range configs {
		configured[config.AgentType] = config.ModelID != nil && config.IsActive
	}
	items := make([]AgentTypeInfo, 0, len(orderedAgentTypes))
	for _, agentType := range orderedAgentTypes {
		items = append(items, AgentTypeInfo{
			Type:       agentType,
			Name:       agentTypeNames[agentType],
			Configured: configured[agentType],
		})
	}
	return AgentTypesResponse{Items: items}, nil
}

func (s *Service) GetAgentConfig(ctx context.Context, agentType string) (AgentModelConfig, error) {
	agentType, err := normalizeAgentType(agentType)
	if err != nil {
		return AgentModelConfig{}, err
	}
	config, ok, err := s.repo.GetAgentConfig(ctx, agentType)
	if err != nil {
		return AgentModelConfig{}, err
	}
	if !ok {
		return AgentModelConfig{}, ErrNotFound
	}
	return config, nil
}

func (s *Service) UpdateAgentConfig(ctx context.Context, agentType string, request UpdateAgentConfigRequest) (AgentModelConfig, error) {
	agentType, err := normalizeAgentType(agentType)
	if err != nil {
		return AgentModelConfig{}, err
	}
	modelID := strings.TrimSpace(request.ModelID)
	if modelID == "" {
		return AgentModelConfig{}, badRequest("model_id 不能为空")
	}
	model, ok, err := s.repo.GetModel(ctx, modelID)
	if err != nil {
		return AgentModelConfig{}, err
	}
	if !ok {
		return AgentModelConfig{}, badRequest("model_id 不存在")
	}
	if !model.IsActive {
		return AgentModelConfig{}, badRequest("不能选择已禁用模型")
	}
	if err := validateOptionalGenerationOverrides(request.TemperatureOverride, request.MaxTokensOverride, request.TopPOverride, request.TimeoutOverride, request.MaxRetriesOverride); err != nil {
		return AgentModelConfig{}, err
	}
	id, err := s.newID()
	if err != nil {
		return AgentModelConfig{}, err
	}
	input := AgentConfigInput{
		ID:                  id,
		AgentType:           agentType,
		ModelID:             modelID,
		TemperatureOverride: request.TemperatureOverride,
		MaxTokensOverride:   request.MaxTokensOverride,
		TopPOverride:        request.TopPOverride,
		TimeoutOverride:     request.TimeoutOverride,
		MaxRetriesOverride:  request.MaxRetriesOverride,
		ExtraConfig:         normalizeObjectMap(request.ExtraConfig),
		IsActive:            true,
	}
	return s.repo.UpsertAgentConfig(ctx, input, s.now())
}

func (s *Service) DeleteAgentConfig(ctx context.Context, agentType string) (SuccessResponse, error) {
	agentType, err := normalizeAgentType(agentType)
	if err != nil {
		return SuccessResponse{}, err
	}
	ok, err := s.repo.DeleteAgentConfig(ctx, agentType)
	if err != nil {
		return SuccessResponse{}, err
	}
	if !ok {
		return SuccessResponse{}, ErrNotFound
	}
	return SuccessResponse{Success: true, Message: "智能体配置已重置"}, nil
}

func (s *Service) TestProvider(ctx context.Context, providerID string, requestedModelID string) (ProviderTestResult, error) {
	provider, ok, err := s.repo.GetProvider(ctx, strings.TrimSpace(providerID))
	if err != nil {
		return ProviderTestResult{}, err
	}
	if !ok {
		return ProviderTestResult{}, ErrNotFound
	}
	apiKey, err := s.cipher.Decrypt(provider.EncryptedAPIKey)
	if err != nil || strings.TrimSpace(apiKey) == "" {
		return ProviderTestResult{Success: false, Message: "API 密钥不可用", LatencyMS: 0}, nil
	}
	modelID := strings.TrimSpace(requestedModelID)
	if modelID == "" {
		models, err := s.repo.ListModels(ctx, ModelFilter{ProviderID: provider.ID})
		if err != nil {
			return ProviderTestResult{}, err
		}
		for _, model := range models {
			if model.IsDefault && model.IsActive {
				modelID = model.ModelID
				break
			}
		}
		if modelID == "" && len(models) > 0 {
			modelID = models[0].ModelID
		}
	}
	if modelID == "" {
		return ProviderTestResult{Success: false, Message: "请先配置至少一个模型", LatencyMS: 0}, nil
	}
	baseURL, err := normalizeBaseURL(provider.BaseURL)
	if err != nil {
		return ProviderTestResult{}, err
	}
	start := time.Now()
	err = s.chatCompletionProbe(ctx, baseURL, apiKey, modelID)
	latency := float64(time.Since(start).Microseconds()) / 1000
	if err != nil {
		return ProviderTestResult{Success: false, Message: "连接失败: " + redact.String(err.Error()), LatencyMS: latency, ModelID: &modelID}, nil
	}
	return ProviderTestResult{Success: true, Message: "连接成功", LatencyMS: latency, ModelID: &modelID}, nil
}

func (s *Service) FetchAvailableModels(ctx context.Context, providerID string) (FetchModelsResponse, error) {
	provider, ok, err := s.repo.GetProvider(ctx, strings.TrimSpace(providerID))
	if err != nil {
		return FetchModelsResponse{}, err
	}
	if !ok {
		return FetchModelsResponse{}, ErrNotFound
	}
	apiKey, err := s.cipher.Decrypt(provider.EncryptedAPIKey)
	if err != nil {
		return FetchModelsResponse{Success: false, Models: []string{}, Message: "API 密钥不可用"}, nil
	}
	baseURL, err := normalizeBaseURL(provider.BaseURL)
	if err != nil {
		return FetchModelsResponse{}, err
	}
	return s.fetchModels(ctx, baseURL, apiKey)
}

func (s *Service) FetchModelsByCredentials(ctx context.Context, request FetchModelsByCredentialsRequest) (FetchModelsResponse, error) {
	baseURL, err := normalizeBaseURL(request.BaseURL)
	if err != nil {
		return FetchModelsResponse{}, err
	}
	apiKey := strings.TrimSpace(request.APIKey)
	if apiKey == "" {
		return FetchModelsResponse{}, badRequest("api_key 不能为空")
	}
	return s.fetchModels(ctx, baseURL, apiKey)
}

func (s *Service) RuntimeConfig(ctx context.Context, agentType string) (RuntimeConfig, bool, error) {
	agentType, err := normalizeAgentType(agentType)
	if err != nil {
		return RuntimeConfig{}, false, err
	}
	config, ok, err := s.repo.GetAgentConfig(ctx, agentType)
	if err != nil {
		return RuntimeConfig{}, false, err
	}
	if !ok || !config.IsActive || config.ModelID == nil {
		return RuntimeConfig{}, false, nil
	}
	model, ok, err := s.repo.GetModel(ctx, *config.ModelID)
	if err != nil {
		return RuntimeConfig{}, false, err
	}
	if !ok || !model.IsActive {
		return RuntimeConfig{}, false, nil
	}
	provider, ok, err := s.repo.GetProvider(ctx, model.ProviderID)
	if err != nil {
		return RuntimeConfig{}, false, err
	}
	if !ok || !provider.IsActive {
		return RuntimeConfig{}, false, nil
	}
	baseURL, err := normalizeBaseURL(provider.BaseURL)
	if err != nil {
		return RuntimeConfig{}, false, err
	}
	apiKey, err := s.cipher.Decrypt(provider.EncryptedAPIKey)
	if err != nil || strings.TrimSpace(apiKey) == "" {
		return RuntimeConfig{}, false, nil
	}
	temperature := model.DefaultTemperature
	if config.TemperatureOverride != nil {
		temperature = *config.TemperatureOverride
	}
	maxTokens := 0
	if model.DefaultMaxTokens != nil {
		maxTokens = *model.DefaultMaxTokens
	}
	if config.MaxTokensOverride != nil {
		maxTokens = *config.MaxTokensOverride
	}
	topP := model.DefaultTopP
	if config.TopPOverride != nil {
		topP = config.TopPOverride
	}
	timeoutSeconds := model.DefaultTimeout
	if config.TimeoutOverride != nil {
		timeoutSeconds = *config.TimeoutOverride
	}
	retries := model.DefaultMaxRetries
	if config.MaxRetriesOverride != nil {
		retries = *config.MaxRetriesOverride
	}
	return RuntimeConfig{
		ProviderCode:  provider.Code,
		ProviderName:  provider.Name,
		BaseURL:       baseURL,
		APIKey:        apiKey,
		Model:         model.ModelID,
		Temperature:   temperature,
		MaxTokens:     maxTokens,
		TopP:          topP,
		Timeout:       time.Duration(timeoutSeconds) * time.Second,
		MaxRetries:    retries,
		MaxIterations: max(1, retries+1),
	}, true, nil
}

func (s *Service) providerInput(request CreateProviderRequest) (ProviderInput, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" || len([]rune(name)) > 100 {
		return ProviderInput{}, badRequest("name 长度必须在 1 到 100 之间")
	}
	code := normalizeProviderCode(request.Code)
	if code == "" || len(code) > 50 {
		return ProviderInput{}, badRequest("code 长度必须在 1 到 50 之间")
	}
	baseURL, err := normalizeBaseURL(request.BaseURL)
	if err != nil {
		return ProviderInput{}, err
	}
	apiKey := strings.TrimSpace(request.APIKey)
	if apiKey == "" {
		return ProviderInput{}, badRequest("api_key 不能为空")
	}
	encrypted, err := s.cipher.Encrypt(apiKey)
	if err != nil {
		return ProviderInput{}, fmt.Errorf("encrypt provider api key: %w", err)
	}
	id, err := s.newID()
	if err != nil {
		return ProviderInput{}, err
	}
	return ProviderInput{
		ID:              id,
		Name:            name,
		Code:            code,
		BaseURL:         baseURL,
		EncryptedAPIKey: encrypted,
		Description:     optionalTrimmedString(request.Description, 500),
		IsActive:        true,
	}, nil
}

func (s *Service) modelInputFromRequest(request CreateModelRequest) (ModelInput, error) {
	id, err := s.newID()
	if err != nil {
		return ModelInput{}, err
	}
	name := strings.TrimSpace(request.Name)
	modelID := strings.TrimSpace(request.ModelID)
	if name == "" {
		name = modelID
	}
	if err := validateModelNameAndID(name, modelID); err != nil {
		return ModelInput{}, err
	}
	if err := validateGenerationDefaults(request.DefaultTemperature, request.DefaultMaxTokens, request.DefaultTopP, request.DefaultTimeout, request.DefaultMaxRetries); err != nil {
		return ModelInput{}, err
	}
	return ModelInput{
		ID:                 id,
		ProviderID:         strings.TrimSpace(request.ProviderID),
		Name:               name,
		ModelID:            modelID,
		DefaultTemperature: ptrutil.ValueOrDefault(request.DefaultTemperature, defaultTemperature),
		DefaultMaxTokens:   request.DefaultMaxTokens,
		DefaultTopP:        request.DefaultTopP,
		DefaultTimeout:     ptrutil.ValueOrDefault(request.DefaultTimeout, defaultTimeout),
		DefaultMaxRetries:  ptrutil.ValueOrDefault(request.DefaultMaxRetries, defaultMaxRetries),
		Capabilities:       normalizeObjectMap(request.Capabilities),
		Description:        optionalTrimmedString(request.Description, 500),
		IsActive:           true,
		IsDefault:          false,
	}, nil
}

func (s *Service) modelInputs(providerID string, models []ModelCreateSimple) ([]ModelInput, error) {
	seen := map[string]bool{}
	inputs := make([]ModelInput, 0, len(models))
	for index, model := range models {
		modelID := strings.TrimSpace(model.ModelID)
		name := strings.TrimSpace(model.Name)
		if name == "" {
			name = modelID
		}
		if seen[modelID] {
			continue
		}
		if err := validateModelNameAndID(name, modelID); err != nil {
			return nil, err
		}
		id, err := s.newID()
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, ModelInput{
			ID:                 id,
			ProviderID:         providerID,
			Name:               name,
			ModelID:            modelID,
			DefaultTemperature: defaultTemperature,
			DefaultTimeout:     defaultTimeout,
			DefaultMaxRetries:  defaultMaxRetries,
			Capabilities:       map[string]any{},
			IsActive:           true,
			IsDefault:          index == 0,
		})
		seen[modelID] = true
	}
	return inputs, nil
}

func modelUpdateFromRequest(request UpdateModelRequest) (ModelUpdate, error) {
	update := ModelUpdate{}
	if request.Name != nil {
		value := strings.TrimSpace(*request.Name)
		if value == "" || len([]rune(value)) > 100 {
			return ModelUpdate{}, badRequest("name 长度必须在 1 到 100 之间")
		}
		update.Name = &value
	}
	if request.ModelID != nil {
		value := strings.TrimSpace(*request.ModelID)
		if value == "" || len([]rune(value)) > 100 {
			return ModelUpdate{}, badRequest("model_id 长度必须在 1 到 100 之间")
		}
		update.ModelID = &value
	}
	if err := validateGenerationDefaults(request.DefaultTemperature, request.DefaultMaxTokens, request.DefaultTopP, request.DefaultTimeout, request.DefaultMaxRetries); err != nil {
		return ModelUpdate{}, err
	}
	update.DefaultTemperature = request.DefaultTemperature
	update.DefaultMaxTokens = request.DefaultMaxTokens
	update.DefaultMaxTokensSet = request.DefaultMaxTokens != nil
	update.DefaultTopP = request.DefaultTopP
	update.DefaultTopPSet = request.DefaultTopP != nil
	update.DefaultTimeout = request.DefaultTimeout
	update.DefaultMaxRetries = request.DefaultMaxRetries
	update.IsActive = request.IsActive
	if request.Capabilities != nil {
		update.Capabilities = normalizeObjectMap(*request.Capabilities)
		update.CapabilitiesSet = true
	}
	if request.Description != nil {
		update.Description = optionalTrimmedString(*request.Description, 500)
		update.DescriptionSet = true
	}
	return update, nil
}

func (s *Service) fetchModels(ctx context.Context, baseURL string, apiKey string) (FetchModelsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, joinProviderURL(baseURL, "/v1/models"), nil)
	if err != nil {
		return FetchModelsResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return FetchModelsResponse{Success: false, Models: []string{}, Message: "获取模型列表失败: " + redact.String(err.Error())}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return FetchModelsResponse{Success: false, Models: []string{}, Message: fmt.Sprintf("获取模型列表失败: HTTP %d", resp.StatusCode)}, nil
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := httpjson.DecodeLimited(resp.Body, 4<<20, &payload); err != nil {
		return FetchModelsResponse{Success: false, Models: []string{}, Message: "模型列表响应格式无效"}, nil
	}
	models := make([]string, 0, len(payload.Data))
	seen := map[string]bool{}
	for _, item := range payload.Data {
		id := strings.TrimSpace(item.ID)
		if id != "" && !seen[id] {
			models = append(models, id)
			seen[id] = true
		}
	}
	sort.Strings(models)
	return FetchModelsResponse{Success: true, Models: models, Message: "获取模型列表成功"}, nil
}

func (s *Service) chatCompletionProbe(ctx context.Context, baseURL string, apiKey string, modelID string) error {
	body, _ := json.Marshal(map[string]any{
		"model":       modelID,
		"messages":    []map[string]string{{"role": "user", "content": "ping"}},
		"max_tokens":  1,
		"temperature": 0,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, joinProviderURL(baseURL, "/v1/chat/completions"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func normalizeBaseURL(value string) (string, error) {
	baseURL, err := outbound.NormalizePublicHTTPSBaseURL(value)
	if err != nil {
		return "", badRequest("base_url " + err.Error())
	}
	return baseURL, nil
}

func normalizeProviderCode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	return value
}

func normalizeAgentType(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if _, ok := agentTypeNames[value]; !ok {
		return "", badRequest("不支持的智能体类型: " + value)
	}
	return value, nil
}

func validateModelNameAndID(name string, modelID string) error {
	if name == "" || len([]rune(name)) > 100 {
		return badRequest("name 长度必须在 1 到 100 之间")
	}
	if modelID == "" || len([]rune(modelID)) > 100 {
		return badRequest("model_id 长度必须在 1 到 100 之间")
	}
	return nil
}

func validateGenerationDefaults(temperature *float64, maxTokens *int, topP *float64, timeout *int, retries *int) error {
	return validateOptionalGenerationOverrides(temperature, maxTokens, topP, timeout, retries)
}

func validateOptionalGenerationOverrides(temperature *float64, maxTokens *int, topP *float64, timeout *int, retries *int) error {
	if temperature != nil && (*temperature < 0 || *temperature > 2) {
		return badRequest("temperature 必须在 0 到 2 之间")
	}
	if maxTokens != nil && *maxTokens < 0 {
		return badRequest("max_tokens 必须大于等于 0")
	}
	if topP != nil && (*topP < 0 || *topP > 1) {
		return badRequest("top_p 必须在 0 到 1 之间")
	}
	if timeout != nil && (*timeout <= 0 || *timeout > 600) {
		return badRequest("timeout 必须在 1 到 600 秒之间")
	}
	if retries != nil && (*retries < 0 || *retries > 10) {
		return badRequest("max_retries 必须在 0 到 10 之间")
	}
	return nil
}

func optionalTrimmedString(value string, maxRunes int) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	runes := []rune(trimmed)
	if maxRunes > 0 && len(runes) > maxRunes {
		trimmed = string(runes[:maxRunes])
	}
	return &trimmed
}

func normalizeObjectMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func joinProviderURL(baseURL string, apiPath string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(base, "/v1") && strings.HasPrefix(apiPath, "/v1/") {
		return base + strings.TrimPrefix(apiPath, "/v1")
	}
	return base + apiPath
}

func badRequest(message string) error {
	return Error{Kind: ErrBadRequest, Message: message}
}

func conflict(message string) error {
	return Error{Kind: ErrConflict, Message: message}
}

func normalizeRepositoryError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrConflict) || errors.Is(err, ErrBadRequest) || errors.Is(err, ErrNotFound) {
		return err
	}
	return err
}

type Error struct {
	Kind    error
	Message string
}

func (e Error) Error() string {
	return e.Message
}

func (e Error) Unwrap() error {
	return e.Kind
}
