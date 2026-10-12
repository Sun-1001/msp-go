package main

import (
	"log/slog"

	dailyquestionhttp "mathstudy/backend/internal/adapter/http/dailyquestion"
	exercisehttp "mathstudy/backend/internal/adapter/http/exercise"
	mistakehttp "mathstudy/backend/internal/adapter/http/mistake"
	portraithttp "mathstudy/backend/internal/adapter/http/portrait"
	progresshttp "mathstudy/backend/internal/adapter/http/progress"
	questionhttp "mathstudy/backend/internal/adapter/http/question"
	sessionhttp "mathstudy/backend/internal/adapter/http/session"
	einoagent "mathstudy/backend/internal/adapter/llm/einoagent"
	adapterpostgres "mathstudy/backend/internal/adapter/postgres"
	storageadapter "mathstudy/backend/internal/adapter/storage"
	answerocrapp "mathstudy/backend/internal/application/answerocr"
	dailyquestionapp "mathstudy/backend/internal/application/dailyquestion"
	exerciseapp "mathstudy/backend/internal/application/exercise"
	mistakeapp "mathstudy/backend/internal/application/mistake"
	portraitapp "mathstudy/backend/internal/application/portrait"
	progressapp "mathstudy/backend/internal/application/progress"
	questionapp "mathstudy/backend/internal/application/question"
	sessionapp "mathstudy/backend/internal/application/session"
	"mathstudy/backend/internal/platform/config"
	"mathstudy/backend/internal/platform/ratelimit"
)

type progressModule struct {
	repo    adapterpostgres.ProgressRepository
	service *progressapp.Service
	handler *progresshttp.Handler
}

func wireProgress(logger *slog.Logger, infra *infrastructure, auth *authModule) (*progressModule, error) {
	progressRepo, err := adapterpostgres.NewProgressRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure progress repository", err)
	}
	progressService, err := progressapp.NewService(progressRepo)
	if err != nil {
		return nil, stepFailed("configure progress service", err)
	}
	progressHandler, err := progresshttp.NewHandler(logger, progressService, auth.service)
	if err != nil {
		return nil, stepFailed("configure progress handler", err)
	}
	return &progressModule{repo: progressRepo, service: progressService, handler: progressHandler}, nil
}

type portraitModule struct {
	repo    adapterpostgres.PortraitRepository
	service *portraitapp.Service
	handler *portraithttp.Handler
}

func wirePortrait(logger *slog.Logger, infra *infrastructure, auth *authModule, ai *aiModule) (*portraitModule, error) {
	portraitRepo, err := adapterpostgres.NewPortraitRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure portrait repository", err)
	}
	portraitGenerator := einoagent.NewConfigurablePortraitGenerator(ai.aiConfigService, ai.einoConfig)
	portraitService, err := portraitapp.NewService(portraitRepo, portraitapp.WithGenerator(portraitGenerator))
	if err != nil {
		return nil, stepFailed("configure portrait service", err)
	}
	portraitHandler, err := portraithttp.NewHandler(logger, portraitService, auth.service, portraithttp.WithAIRequestGuard(ai.riskService))
	if err != nil {
		return nil, stepFailed("configure portrait handler", err)
	}
	return &portraitModule{repo: portraitRepo, service: portraitService, handler: portraitHandler}, nil
}

type exerciseModule struct {
	repo           adapterpostgres.ExerciseRepository
	service        *exerciseapp.Service
	handler        *exercisehttp.Handler
	mistakeService *mistakeapp.Service
	mistakeHandler *mistakehttp.Handler
}

func wireExercise(cfg config.Config, logger *slog.Logger, infra *infrastructure, auth *authModule, ai *aiModule, uploadStorage *storageadapter.RuntimeManager) (*exerciseModule, error) {
	exerciseRepo, err := adapterpostgres.NewExerciseRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure exercise repository", err)
	}
	mathSolver := einoagent.NewConfigurableMathSolver(ai.aiConfigService, ai.einoConfig)
	answerOCRRecognizer := einoagent.NewConfigurableAnswerOCR(ai.aiConfigService, ai.einoConfig)
	answerOCRService, err := answerocrapp.NewService(uploadStorage, answerOCRRecognizer)
	if err != nil {
		return nil, stepFailed("configure answer OCR service", err)
	}
	diagnostician := einoagent.NewConfigurableDiagnostician(ai.aiConfigService, ai.einoConfig)
	questionGenerator := einoagent.NewConfigurableQuestionGenerator(ai.aiConfigService, ai.einoConfig)
	exerciseService, err := exerciseapp.NewService(
		exerciseRepo,
		exerciseapp.SolverAnswerChecker{Solver: mathSolver},
		exerciseapp.WithAnswerOCR(answerOCRService),
		exerciseapp.WithSolutionSolver(mathSolver),
		exerciseapp.WithDiagnostician(diagnostician),
		exerciseapp.WithQuestionGenerator(questionGenerator),
	)
	if err != nil {
		return nil, stepFailed("configure exercise service", err)
	}
	mistakeRepo, err := adapterpostgres.NewMistakeRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure mistake repository", err)
	}
	mistakeService, err := mistakeapp.NewService(mistakeRepo, exerciseService)
	if err != nil {
		return nil, stepFailed("configure mistake service", err)
	}
	mistakeHandler, err := mistakehttp.NewHandler(logger, mistakeService, auth.service)
	if err != nil {
		return nil, stepFailed("configure mistake handler", err)
	}
	exerciseHandler, err := exercisehttp.NewHandler(
		logger,
		exerciseService,
		auth.service,
		exercisehttp.WithRedisRateLimit(infra.redis, cfg.RedisFallbackCacheMaxSize),
		exercisehttp.WithAIRequestGuard(ai.riskService),
	)
	if err != nil {
		return nil, stepFailed("configure exercise handler", err)
	}
	return &exerciseModule{
		repo:           exerciseRepo,
		service:        exerciseService,
		handler:        exerciseHandler,
		mistakeService: mistakeService,
		mistakeHandler: mistakeHandler,
	}, nil
}

type dailyQuestionModule struct {
	repo           adapterpostgres.DailyQuestionRepository
	service        *dailyquestionapp.Service
	worker         *dailyquestionapp.Worker
	reminderWorker *dailyquestionapp.ReminderWorker
	handler        *dailyquestionhttp.Handler
}

func wireDailyQuestion(cfg config.Config, logger *slog.Logger, infra *infrastructure, auth *authModule, ai *aiModule, exercise *exerciseModule, enqueuer adapterpostgres.WechatReminderEnqueuer) (*dailyQuestionModule, error) {
	dailyQuestionRepo, err := adapterpostgres.NewDailyQuestionRepository(infra.db, enqueuer)
	if err != nil {
		return nil, stepFailed("configure daily question repository", err)
	}
	dailyQuestionGenerationLimiter, err := ratelimit.New(
		infra.redis,
		"msp:exercise_generation",
		dailyQuestionGenerationLimitMax,
		dailyQuestionGenerationWindow,
		cfg.RedisFallbackCacheMaxSize,
		logger,
	)
	if err != nil {
		return nil, stepFailed("configure daily question generation rate limit", err)
	}
	dailyQuestionService, err := dailyquestionapp.NewService(
		dailyQuestionRepo,
		exercise.service,
		dailyquestionapp.WithAIRequestGuard(ai.riskService),
		dailyquestionapp.WithGenerationLimiter(dailyQuestionGenerationLimiter),
		dailyquestionapp.WithWechatRemindersEnabled(cfg.WechatMessageRemindersEnabled),
	)
	if err != nil {
		return nil, stepFailed("configure daily question service", err)
	}
	dailyQuestionWorker, err := dailyquestionapp.NewWorker(
		dailyQuestionRepo,
		dailyQuestionService,
		logger,
		dailyquestionapp.WorkerConfig{
			BatchSize:      cfg.DailyQuestionPreGenerationBatchSize,
			Concurrency:    cfg.DailyQuestionPreGenerationConcurrency,
			BatchInterval:  cfg.DailyQuestionPreGenerationBatchInterval,
			StudentTimeout: cfg.DailyQuestionPreGenerationStudentTimeout,
		},
	)
	if err != nil {
		return nil, stepFailed("configure daily question worker", err)
	}
	dailyQuestionReminderWorker, err := dailyquestionapp.NewReminderWorker(dailyQuestionService, logger)
	if err != nil {
		return nil, stepFailed("configure daily question reminder worker", err)
	}
	dailyQuestionHandler, err := dailyquestionhttp.NewHandler(
		logger,
		dailyQuestionService,
		auth.service,
	)
	if err != nil {
		return nil, stepFailed("configure daily question handler", err)
	}
	return &dailyQuestionModule{
		repo:           dailyQuestionRepo,
		service:        dailyQuestionService,
		worker:         dailyQuestionWorker,
		reminderWorker: dailyQuestionReminderWorker,
		handler:        dailyQuestionHandler,
	}, nil
}

type sessionModule struct {
	repo    adapterpostgres.SessionRepository
	service *sessionapp.Service
	handler *sessionhttp.Handler
}

func wireSession(logger *slog.Logger, infra *infrastructure, auth *authModule, ai *aiModule, exercise *exerciseModule, resource *resourceModule) (*sessionModule, error) {
	sessionRepo, err := adapterpostgres.NewSessionRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure session repository", err)
	}
	tutorAgent := einoagent.NewConfigurableTutorAgent(ai.aiConfigService, ai.einoConfig)
	sessionService, err := sessionapp.NewService(
		sessionRepo,
		sessionapp.WithChatAgent(tutorAgent),
		sessionapp.WithAIRequestGuard(ai.riskService),
		sessionapp.WithLogger(logger),
		sessionapp.WithKnowledgeRetriever(resource.searchService),
		sessionapp.WithStudentContextReader(sessionRepo),
		sessionapp.WithExerciseTutorReader(exercise.service),
	)
	if err != nil {
		return nil, stepFailed("configure session service", err)
	}
	sessionHandler, err := sessionhttp.NewHandler(logger, sessionService, auth.service)
	if err != nil {
		return nil, stepFailed("configure session handler", err)
	}
	return &sessionModule{repo: sessionRepo, service: sessionService, handler: sessionHandler}, nil
}

type questionModule struct {
	repo    adapterpostgres.QuestionRepository
	service *questionapp.Service
	handler *questionhttp.Handler
}

func wireQuestion(logger *slog.Logger, infra *infrastructure, auth *authModule, ai *aiModule) (*questionModule, error) {
	questionRepo, err := adapterpostgres.NewQuestionRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure question repository", err)
	}
	questionParser := einoagent.NewConfigurableQuestionParser(ai.aiConfigService, ai.einoConfig)
	questionService, err := questionapp.NewService(questionRepo, questionapp.WithParser(questionParser))
	if err != nil {
		return nil, stepFailed("configure question service", err)
	}
	questionHandler, err := questionhttp.NewHandler(logger, questionService, auth.service)
	if err != nil {
		return nil, stepFailed("configure question handler", err)
	}
	return &questionModule{repo: questionRepo, service: questionService, handler: questionHandler}, nil
}
