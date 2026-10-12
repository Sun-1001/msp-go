package main

import (
	"log/slog"

	resourcehttp "mathstudy/backend/internal/adapter/http/resource"
	resourceretrievaladapter "mathstudy/backend/internal/adapter/llm/resourceretrieval"
	adapterpostgres "mathstudy/backend/internal/adapter/postgres"
	storageadapter "mathstudy/backend/internal/adapter/storage"
	resourceapp "mathstudy/backend/internal/application/resource"
	"mathstudy/backend/internal/platform/config"
)

type resourceModule struct {
	repo             adapterpostgres.ResourceRepository
	service          *resourceapp.Service
	searchService    *resourceapp.SearchService
	ingestionService *resourceapp.IngestionService
	handler          *resourcehttp.Handler
}

func wireResource(cfg config.Config, logger *slog.Logger, infra *infrastructure, auth *authModule, ai *aiModule, uploadStorage *storageadapter.RuntimeManager) (*resourceModule, error) {
	resourceRepo, err := adapterpostgres.NewResourceRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure resource repository", err)
	}
	resourceService, err := resourceapp.NewService(resourceRepo)
	if err != nil {
		return nil, stepFailed("configure resource service", err)
	}
	var resourceVectorRetriever resourceapp.SearchCandidateRetriever
	if infra.qdrant != nil {
		queryEmbedder, embedErr := resourceretrievaladapter.NewQueryEmbedder(ai.aiConfigService)
		if embedErr != nil {
			return nil, stepFailed("configure resource query embedder", embedErr)
		}
		resourceVectorRetriever, err = resourceapp.NewVectorRetriever(infra.qdrant, queryEmbedder, resourceRepo)
		if err != nil {
			return nil, stepFailed("configure resource vector retriever", err)
		}
	}
	resourceReranker, err := resourceretrievaladapter.NewReranker(ai.aiConfigService)
	if err != nil {
		return nil, stepFailed("configure resource reranker", err)
	}
	resourceSearchService, err := resourceapp.NewSearchService(resourceRepo, resourceVectorRetriever,
		resourceapp.WithSearchReranker(resourceReranker),
		resourceapp.WithSearchObserver(resourceSearchObserver(infra.metrics, logger)),
	)
	if err != nil {
		return nil, stepFailed("configure resource search service", err)
	}
	var resourceIngestionService *resourceapp.IngestionService
	if cfg.QdrantEnabled {
		documentEmbedder, embedErr := resourceretrievaladapter.NewDocumentEmbedder(ai.aiConfigService)
		if embedErr != nil {
			return nil, &startupError{step: "configure document embedder", attrs: []any{"error_code", "configuration_invalid"}}
		}
		resourceIngestionService, err = resourceapp.NewIngestionService(resourceRepo, uploadStorage, documentEmbedder)
		if err != nil {
			return nil, &startupError{step: "configure resource ingestion", attrs: []any{"error_code", "configuration_invalid"}}
		}
	}
	resourceOptions := []resourcehttp.Option{resourcehttp.WithSearchService(resourceSearchService)}
	if resourceIngestionService != nil {
		resourceOptions = append(resourceOptions, resourcehttp.WithIngestionService(resourceIngestionService))
	}
	resourceHandler, err := resourcehttp.NewHandler(
		logger, resourceService, auth.service, resourceOptions...,
	)
	if err != nil {
		return nil, stepFailed("configure resource handler", err)
	}
	return &resourceModule{
		repo:             resourceRepo,
		service:          resourceService,
		searchService:    resourceSearchService,
		ingestionService: resourceIngestionService,
		handler:          resourceHandler,
	}, nil
}
