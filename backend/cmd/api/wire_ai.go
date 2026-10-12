package main

import (
	"log/slog"
	"net/http"
	"time"

	adminaiconfighthttp "mathstudy/backend/internal/adapter/http/adminaiconfig"
	adminairiskhttp "mathstudy/backend/internal/adapter/http/adminairisk"
	openaihttp "mathstudy/backend/internal/adapter/http/openai"
	einoagent "mathstudy/backend/internal/adapter/llm/einoagent"
	moderationadapter "mathstudy/backend/internal/adapter/llm/moderation"
	openaicompatadapter "mathstudy/backend/internal/adapter/llm/openaicompat"
	adapterpostgres "mathstudy/backend/internal/adapter/postgres"
	adapterredis "mathstudy/backend/internal/adapter/redis"
	adminaiconfigapp "mathstudy/backend/internal/application/adminaiconfig"
	airiskapp "mathstudy/backend/internal/application/airisk"
	"mathstudy/backend/internal/platform/config"
	"mathstudy/backend/internal/platform/outbound"
	"mathstudy/backend/internal/platform/ratelimit"
	"mathstudy/backend/internal/platform/secret"
)

type aiModule struct {
	riskRepo            adapterpostgres.AIRiskRepository
	riskSlots           *adapterredis.AIRiskSlotStore
	cipher              *secret.Fernet
	aiConfigRepo        adapterpostgres.AdminAIConfigRepository
	aiConfigService     *adminaiconfigapp.Service
	aiConfigHandler     *adminaiconfighthttp.Handler
	responsesDispatcher *openaicompatadapter.ResponsesDispatcher
	responsesLimiter    *ratelimit.Limiter
	riskService         *airiskapp.Service
	openAIHandler       *openaihttp.Handler
	riskHandler         *adminairiskhttp.Handler
	einoConfig          einoagent.Config
}

// wireAIRiskStores builds the AI risk repository and slot store at the point
// they appear in the original startup sequence.
func wireAIRiskStores(infra *infrastructure) (adapterpostgres.AIRiskRepository, *adapterredis.AIRiskSlotStore, error) {
	aiRiskRepo, err := adapterpostgres.NewAIRiskRepository(infra.db)
	if err != nil {
		return adapterpostgres.AIRiskRepository{}, nil, stepFailed("configure AI risk repository", err)
	}
	aiRiskSlots, err := adapterredis.NewAIRiskSlotStore(infra.redis)
	if err != nil {
		return adapterpostgres.AIRiskRepository{}, nil, stepFailed("configure AI risk slot store", err)
	}
	return aiRiskRepo, aiRiskSlots, nil
}

func wireAI(cfg config.Config, logger *slog.Logger, infra *infrastructure, auth *authModule, riskRepo adapterpostgres.AIRiskRepository, riskSlots *adapterredis.AIRiskSlotStore) (*aiModule, error) {
	fernetKey := cfg.FernetSecretKey
	if fernetKey == "" {
		logger.Warn("FERNET_SECRET_KEY is not configured; using an ephemeral key for local secret encryption")
		var err error
		fernetKey, err = secret.GenerateFernetKey()
		if err != nil {
			return nil, stepFailed("generate ephemeral fernet key", err)
		}
	}
	appCipher, err := secret.NewFernet(fernetKey)
	if err != nil {
		return nil, stepFailed("configure fernet cipher", err)
	}
	adminAIConfigRepo, err := adapterpostgres.NewAdminAIConfigRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure admin AI config repository", err)
	}
	providerBaseHTTPClient := outbound.NewPublicHTTPSClient(20 * time.Second)
	if transport, ok := providerBaseHTTPClient.Transport.(*http.Transport); ok {
		transport.MaxIdleConnsPerHost = 20
	}
	providerHTTPClient := openaicompatadapter.NewProviderClient(providerBaseHTTPClient)
	adminAIConfigService, err := adminaiconfigapp.NewService(adminAIConfigRepo, appCipher, providerHTTPClient)
	if err != nil {
		return nil, stepFailed("configure admin AI config service", err)
	}
	responsesDispatcher, err := openaicompatadapter.NewResponsesDispatcher(adminAIConfigService, providerBaseHTTPClient)
	if err != nil {
		return nil, stepFailed("configure OpenAI Responses dispatcher", err)
	}
	responsesLimiter, err := ratelimit.New(
		infra.redis,
		"msp:openai_responses",
		openAIResponsesRateLimitMax,
		openAIResponsesRateLimitWindow,
		cfg.RedisFallbackCacheMaxSize,
		logger,
	)
	if err != nil {
		return nil, stepFailed("configure OpenAI Responses rate limit", err)
	}
	adminAIConfigHandler, err := adminaiconfighthttp.NewHandler(logger, adminAIConfigService, auth.service)
	if err != nil {
		return nil, stepFailed("configure admin AI config handler", err)
	}
	contentReviewer, err := moderationadapter.NewReviewer(adminAIConfigService)
	if err != nil {
		return nil, stepFailed("configure AI content reviewer", err)
	}
	aiRiskService, err := airiskapp.NewService(riskRepo, riskSlots, airiskapp.WithContentReviewer(contentReviewer))
	if err != nil {
		return nil, stepFailed("configure AI risk service", err)
	}
	openAIHandler, err := openaihttp.NewHandler(
		logger,
		responsesDispatcher,
		auth.service,
		aiRiskService,
		responsesLimiter,
		infra.metrics,
	)
	if err != nil {
		return nil, stepFailed("configure OpenAI Responses handler", err)
	}
	aiRiskHandler, err := adminairiskhttp.NewHandler(logger, aiRiskService, auth.service)
	if err != nil {
		return nil, stepFailed("configure AI risk handler", err)
	}
	return &aiModule{
		riskRepo:            riskRepo,
		riskSlots:           riskSlots,
		cipher:              appCipher,
		aiConfigRepo:        adminAIConfigRepo,
		aiConfigService:     adminAIConfigService,
		aiConfigHandler:     adminAIConfigHandler,
		responsesDispatcher: responsesDispatcher,
		responsesLimiter:    responsesLimiter,
		riskService:         aiRiskService,
		openAIHandler:       openAIHandler,
		riskHandler:         aiRiskHandler,
		einoConfig:          defaultEinoConfig(cfg),
	}, nil
}
