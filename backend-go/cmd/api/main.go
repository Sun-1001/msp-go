package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	adminaiconfighthttp "mathstudy/backend-go/internal/adapter/http/adminaiconfig"
	admininboxhttp "mathstudy/backend-go/internal/adapter/http/admininbox"
	adminsettingshttp "mathstudy/backend-go/internal/adapter/http/adminsettings"
	adminstatshttp "mathstudy/backend-go/internal/adapter/http/adminstats"
	adminuserhttp "mathstudy/backend-go/internal/adapter/http/adminuser"
	authhttp "mathstudy/backend-go/internal/adapter/http/auth"
	classroomhttp "mathstudy/backend-go/internal/adapter/http/classroom"
	exercisehttp "mathstudy/backend-go/internal/adapter/http/exercise"
	knowledgehttp "mathstudy/backend-go/internal/adapter/http/knowledge"
	mistakehttp "mathstudy/backend-go/internal/adapter/http/mistake"
	portraithttp "mathstudy/backend-go/internal/adapter/http/portrait"
	progresshttp "mathstudy/backend-go/internal/adapter/http/progress"
	questionhttp "mathstudy/backend-go/internal/adapter/http/question"
	resourcehttp "mathstudy/backend-go/internal/adapter/http/resource"
	securityloghttp "mathstudy/backend-go/internal/adapter/http/securitylog"
	sessionhttp "mathstudy/backend-go/internal/adapter/http/session"
	teacherhttp "mathstudy/backend-go/internal/adapter/http/teacher"
	uploadhttp "mathstudy/backend-go/internal/adapter/http/upload"
	xidianhttp "mathstudy/backend-go/internal/adapter/http/xidian"
	einoagent "mathstudy/backend-go/internal/adapter/llm/einoagent"
	adapterpostgres "mathstudy/backend-go/internal/adapter/postgres"
	storageadapter "mathstudy/backend-go/internal/adapter/storage"
	adminaiconfigapp "mathstudy/backend-go/internal/application/adminaiconfig"
	admininboxapp "mathstudy/backend-go/internal/application/admininbox"
	adminsettingsapp "mathstudy/backend-go/internal/application/adminsettings"
	adminstatsapp "mathstudy/backend-go/internal/application/adminstats"
	adminuserapp "mathstudy/backend-go/internal/application/adminuser"
	answerocrapp "mathstudy/backend-go/internal/application/answerocr"
	authapp "mathstudy/backend-go/internal/application/auth"
	classroomapp "mathstudy/backend-go/internal/application/classroom"
	exerciseapp "mathstudy/backend-go/internal/application/exercise"
	knowledgeapp "mathstudy/backend-go/internal/application/knowledge"
	mistakeapp "mathstudy/backend-go/internal/application/mistake"
	portraitapp "mathstudy/backend-go/internal/application/portrait"
	progressapp "mathstudy/backend-go/internal/application/progress"
	questionapp "mathstudy/backend-go/internal/application/question"
	resourceapp "mathstudy/backend-go/internal/application/resource"
	securitylogapp "mathstudy/backend-go/internal/application/securitylog"
	sessionapp "mathstudy/backend-go/internal/application/session"
	teacherapp "mathstudy/backend-go/internal/application/teacher"
	uploadapp "mathstudy/backend-go/internal/application/upload"
	xidianapp "mathstudy/backend-go/internal/application/xidian"
	xidianintegration "mathstudy/backend-go/internal/integration/xidian"
	"mathstudy/backend-go/internal/platform/config"
	"mathstudy/backend-go/internal/platform/health"
	"mathstudy/backend-go/internal/platform/httpserver"
	"mathstudy/backend-go/internal/platform/metrics"
	platformpostgres "mathstudy/backend-go/internal/platform/postgres"
	platformredis "mathstudy/backend-go/internal/platform/redis"
	"mathstudy/backend-go/internal/platform/secret"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg)
	slog.SetDefault(logger)

	dbPool, err := platformpostgres.NewPool(ctx, cfg)
	if err != nil {
		logger.Error("configure postgres pool", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	redisClient := platformredis.NewClient(cfg)
	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.Warn("close redis client", "error", err)
		}
	}()
	if err := requireSharedRedis(ctx, cfg, redisClient); err != nil {
		logger.Error("redis is required for production refresh sessions", "error", err)
		os.Exit(1)
	}

	userRepo, err := adapterpostgres.NewUserRepository(dbPool)
	if err != nil {
		logger.Error("configure user repository", "error", err)
		os.Exit(1)
	}
	tokenService, err := authapp.NewTokenService(
		cfg.JWTSecretKey,
		cfg.JWTAlgorithm,
		cfg.JWTAccessTokenExpire,
		cfg.JWTRefreshTokenExpire,
	)
	if err != nil {
		logger.Error("configure token service", "error", err)
		os.Exit(1)
	}
	loginLimiter := authapp.NewLoginLimiter(redisClient, cfg.LoginMaxAttempts, cfg.LoginLockout, logger)
	captchaManager, err := authapp.NewSliderCaptchaManager(redisClient, logger, authapp.SliderCaptchaConfig{
		ChallengeTTL: cfg.LoginCaptchaTTL,
		ProofTTL:     cfg.LoginCaptchaProofTTL,
		IssueWindow:  cfg.LoginCaptchaIssueWindow,
		Tolerance:    cfg.LoginCaptchaTolerance,
		IssueLimit:   cfg.LoginCaptchaIssueLimit,
		MaxLocalSize: cfg.RedisFallbackCacheMaxSize,
		Strict:       cfg.RequiresSharedRefreshSessionStore(),
	})
	if err != nil {
		logger.Error("configure login captcha", "error", err)
		os.Exit(1)
	}
	refreshSessions := authapp.NewRefreshSessionStore(
		redisClient,
		logger,
		authapp.WithStrictRefreshSessions(cfg.RequiresSharedRefreshSessionStore()),
		authapp.WithMaxLocalRefreshSessions(cfg.RedisFallbackCacheMaxSize),
	)
	authService, err := authapp.NewService(
		userRepo,
		userRepo,
		userRepo,
		tokenService,
		loginLimiter,
		logger,
		authapp.WithRefreshSessionStore(refreshSessions),
	)
	if err != nil {
		logger.Error("configure auth service", "error", err)
		os.Exit(1)
	}
	if _, err := authService.InitAdmin(ctx, cfg.AdminUsername, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		logger.Error("initialize admin account", "error", err)
		os.Exit(1)
	}
	authHandler, err := authhttp.NewHandler(cfg, logger, authService, captchaManager)
	if err != nil {
		logger.Error("configure auth handler", "error", err)
		os.Exit(1)
	}
	progressRepo, err := adapterpostgres.NewProgressRepository(dbPool)
	if err != nil {
		logger.Error("configure progress repository", "error", err)
		os.Exit(1)
	}
	progressService, err := progressapp.NewService(progressRepo)
	if err != nil {
		logger.Error("configure progress service", "error", err)
		os.Exit(1)
	}
	progressHandler, err := progresshttp.NewHandler(logger, progressService, authService)
	if err != nil {
		logger.Error("configure progress handler", "error", err)
		os.Exit(1)
	}
	fernetKey := cfg.FernetSecretKey
	if fernetKey == "" {
		logger.Warn("FERNET_SECRET_KEY is not configured; using an ephemeral key for local secret encryption")
		fernetKey, err = secret.GenerateFernetKey()
		if err != nil {
			logger.Error("generate ephemeral fernet key", "error", err)
			os.Exit(1)
		}
	}
	appCipher, err := secret.NewFernet(fernetKey)
	if err != nil {
		logger.Error("configure fernet cipher", "error", err)
		os.Exit(1)
	}
	adminAIConfigRepo, err := adapterpostgres.NewAdminAIConfigRepository(dbPool)
	if err != nil {
		logger.Error("configure admin AI config repository", "error", err)
		os.Exit(1)
	}
	adminAIConfigService, err := adminaiconfigapp.NewService(adminAIConfigRepo, appCipher)
	if err != nil {
		logger.Error("configure admin AI config service", "error", err)
		os.Exit(1)
	}
	adminAIConfigHandler, err := adminaiconfighthttp.NewHandler(logger, adminAIConfigService, authService)
	if err != nil {
		logger.Error("configure admin AI config handler", "error", err)
		os.Exit(1)
	}
	portraitRepo, err := adapterpostgres.NewPortraitRepository(dbPool)
	if err != nil {
		logger.Error("configure portrait repository", "error", err)
		os.Exit(1)
	}
	portraitGenerator := einoagent.NewConfigurablePortraitGenerator(adminAIConfigService, einoagent.Config{
		Enabled:       cfg.EinoEnabled,
		BaseURL:       cfg.EinoBaseURL,
		APIKey:        cfg.EinoAPIKey,
		Model:         cfg.EinoModel,
		Timeout:       cfg.EinoTimeout,
		Temperature:   cfg.EinoTemperature,
		MaxTokens:     cfg.EinoMaxTokens,
		MaxIterations: cfg.EinoMaxIterations,
	})
	portraitService, err := portraitapp.NewService(portraitRepo, portraitapp.WithGenerator(portraitGenerator))
	if err != nil {
		logger.Error("configure portrait service", "error", err)
		os.Exit(1)
	}
	portraitHandler, err := portraithttp.NewHandler(logger, portraitService, authService)
	if err != nil {
		logger.Error("configure portrait handler", "error", err)
		os.Exit(1)
	}
	mistakeRepo, err := adapterpostgres.NewMistakeRepository(dbPool)
	if err != nil {
		logger.Error("configure mistake repository", "error", err)
		os.Exit(1)
	}
	mistakeService, err := mistakeapp.NewService(mistakeRepo)
	if err != nil {
		logger.Error("configure mistake service", "error", err)
		os.Exit(1)
	}
	mistakeHandler, err := mistakehttp.NewHandler(logger, mistakeService, authService)
	if err != nil {
		logger.Error("configure mistake handler", "error", err)
		os.Exit(1)
	}
	uploadStorage, err := storageadapter.NewUploadStorage(cfg, logger)
	if err != nil {
		logger.Error("configure upload storage", "error", err)
		os.Exit(1)
	}
	exerciseRepo, err := adapterpostgres.NewExerciseRepository(dbPool)
	if err != nil {
		logger.Error("configure exercise repository", "error", err)
		os.Exit(1)
	}
	mathSolver := einoagent.NewConfigurableMathSolver(adminAIConfigService, einoagent.Config{
		Enabled:       cfg.EinoEnabled,
		BaseURL:       cfg.EinoBaseURL,
		APIKey:        cfg.EinoAPIKey,
		Model:         cfg.EinoModel,
		Timeout:       cfg.EinoTimeout,
		Temperature:   cfg.EinoTemperature,
		MaxTokens:     cfg.EinoMaxTokens,
		MaxIterations: cfg.EinoMaxIterations,
	})
	answerOCRRecognizer := einoagent.NewConfigurableAnswerOCR(adminAIConfigService, einoagent.Config{
		Enabled:       cfg.EinoEnabled,
		BaseURL:       cfg.EinoBaseURL,
		APIKey:        cfg.EinoAPIKey,
		Model:         cfg.EinoModel,
		Timeout:       cfg.EinoTimeout,
		Temperature:   cfg.EinoTemperature,
		MaxTokens:     cfg.EinoMaxTokens,
		MaxIterations: cfg.EinoMaxIterations,
	})
	answerOCRService, err := answerocrapp.NewService(uploadStorage, answerOCRRecognizer)
	if err != nil {
		logger.Error("configure answer OCR service", "error", err)
		os.Exit(1)
	}
	diagnostician := einoagent.NewConfigurableDiagnostician(adminAIConfigService, einoagent.Config{
		Enabled:       cfg.EinoEnabled,
		BaseURL:       cfg.EinoBaseURL,
		APIKey:        cfg.EinoAPIKey,
		Model:         cfg.EinoModel,
		Timeout:       cfg.EinoTimeout,
		Temperature:   cfg.EinoTemperature,
		MaxTokens:     cfg.EinoMaxTokens,
		MaxIterations: cfg.EinoMaxIterations,
	})
	questionGenerator := einoagent.NewConfigurableQuestionGenerator(adminAIConfigService, einoagent.Config{
		Enabled:       cfg.EinoEnabled,
		BaseURL:       cfg.EinoBaseURL,
		APIKey:        cfg.EinoAPIKey,
		Model:         cfg.EinoModel,
		Timeout:       cfg.EinoTimeout,
		Temperature:   cfg.EinoTemperature,
		MaxTokens:     cfg.EinoMaxTokens,
		MaxIterations: cfg.EinoMaxIterations,
	})
	exerciseService, err := exerciseapp.NewService(
		exerciseRepo,
		exerciseapp.SolverAnswerChecker{Solver: mathSolver},
		exerciseapp.WithAnswerOCR(answerOCRService),
		exerciseapp.WithSolutionSolver(mathSolver),
		exerciseapp.WithDiagnostician(diagnostician),
		exerciseapp.WithQuestionGenerator(questionGenerator),
	)
	if err != nil {
		logger.Error("configure exercise service", "error", err)
		os.Exit(1)
	}
	exerciseHandler, err := exercisehttp.NewHandler(
		logger,
		exerciseService,
		authService,
		exercisehttp.WithRedisRateLimit(redisClient, cfg.RedisFallbackCacheMaxSize),
	)
	if err != nil {
		logger.Error("configure exercise handler", "error", err)
		os.Exit(1)
	}
	sessionRepo, err := adapterpostgres.NewSessionRepository(dbPool)
	if err != nil {
		logger.Error("configure session repository", "error", err)
		os.Exit(1)
	}
	tutorAgent := einoagent.NewConfigurableTutorAgent(adminAIConfigService, einoagent.Config{
		Enabled:       cfg.EinoEnabled,
		BaseURL:       cfg.EinoBaseURL,
		APIKey:        cfg.EinoAPIKey,
		Model:         cfg.EinoModel,
		Timeout:       cfg.EinoTimeout,
		Temperature:   cfg.EinoTemperature,
		MaxTokens:     cfg.EinoMaxTokens,
		MaxIterations: cfg.EinoMaxIterations,
	})
	sessionService, err := sessionapp.NewService(sessionRepo, sessionapp.WithChatAgent(tutorAgent))
	if err != nil {
		logger.Error("configure session service", "error", err)
		os.Exit(1)
	}
	sessionHandler, err := sessionhttp.NewHandler(logger, sessionService, authService)
	if err != nil {
		logger.Error("configure session handler", "error", err)
		os.Exit(1)
	}
	resourceRepo, err := adapterpostgres.NewResourceRepository(dbPool)
	if err != nil {
		logger.Error("configure resource repository", "error", err)
		os.Exit(1)
	}
	resourceService, err := resourceapp.NewService(resourceRepo)
	if err != nil {
		logger.Error("configure resource service", "error", err)
		os.Exit(1)
	}
	resourceHandler, err := resourcehttp.NewHandler(logger, resourceService, authService)
	if err != nil {
		logger.Error("configure resource handler", "error", err)
		os.Exit(1)
	}
	questionRepo, err := adapterpostgres.NewQuestionRepository(dbPool)
	if err != nil {
		logger.Error("configure question repository", "error", err)
		os.Exit(1)
	}
	questionParser := einoagent.NewConfigurableQuestionParser(adminAIConfigService, einoagent.Config{
		Enabled:       cfg.EinoEnabled,
		BaseURL:       cfg.EinoBaseURL,
		APIKey:        cfg.EinoAPIKey,
		Model:         cfg.EinoModel,
		Timeout:       cfg.EinoTimeout,
		Temperature:   cfg.EinoTemperature,
		MaxTokens:     cfg.EinoMaxTokens,
		MaxIterations: cfg.EinoMaxIterations,
	})
	questionService, err := questionapp.NewService(questionRepo, questionapp.WithParser(questionParser))
	if err != nil {
		logger.Error("configure question service", "error", err)
		os.Exit(1)
	}
	questionHandler, err := questionhttp.NewHandler(logger, questionService, authService)
	if err != nil {
		logger.Error("configure question handler", "error", err)
		os.Exit(1)
	}
	classRepo, err := adapterpostgres.NewClassRepository(dbPool)
	if err != nil {
		logger.Error("configure class repository", "error", err)
		os.Exit(1)
	}
	classService, err := classroomapp.NewService(classRepo)
	if err != nil {
		logger.Error("configure class service", "error", err)
		os.Exit(1)
	}
	classHandler, err := classroomhttp.NewHandler(logger, classService, authService)
	if err != nil {
		logger.Error("configure class handler", "error", err)
		os.Exit(1)
	}
	teacherRepo, err := adapterpostgres.NewTeacherRepository(dbPool)
	if err != nil {
		logger.Error("configure teacher repository", "error", err)
		os.Exit(1)
	}
	teacherService, err := teacherapp.NewService(teacherRepo)
	if err != nil {
		logger.Error("configure teacher service", "error", err)
		os.Exit(1)
	}
	teacherHandler, err := teacherhttp.NewHandler(logger, teacherService, authService)
	if err != nil {
		logger.Error("configure teacher handler", "error", err)
		os.Exit(1)
	}
	knowledgeRepo, err := adapterpostgres.NewKnowledgeRepository(dbPool)
	if err != nil {
		logger.Error("configure knowledge repository", "error", err)
		os.Exit(1)
	}
	knowledgeService, err := knowledgeapp.NewService(knowledgeRepo)
	if err != nil {
		logger.Error("configure knowledge service", "error", err)
		os.Exit(1)
	}
	knowledgeHandler, err := knowledgehttp.NewHandler(logger, knowledgeService, authService)
	if err != nil {
		logger.Error("configure knowledge handler", "error", err)
		os.Exit(1)
	}
	adminUserService, err := adminuserapp.NewService(userRepo)
	if err != nil {
		logger.Error("configure admin user service", "error", err)
		os.Exit(1)
	}
	adminUserHandler, err := adminuserhttp.NewHandler(logger, adminUserService, authService)
	if err != nil {
		logger.Error("configure admin user handler", "error", err)
		os.Exit(1)
	}
	adminInboxService, err := admininboxapp.NewService(userRepo, loginLimiter)
	if err != nil {
		logger.Error("configure admin inbox service", "error", err)
		os.Exit(1)
	}
	adminInboxHandler, err := admininboxhttp.NewHandler(logger, adminInboxService, authService)
	if err != nil {
		logger.Error("configure admin inbox handler", "error", err)
		os.Exit(1)
	}
	adminStatsRepo, err := adapterpostgres.NewAdminStatsRepository(dbPool)
	if err != nil {
		logger.Error("configure admin stats repository", "error", err)
		os.Exit(1)
	}
	adminStatsService, err := adminstatsapp.NewService(adminStatsRepo, adminStatusProvider(dbPool, redisClient))
	if err != nil {
		logger.Error("configure admin stats service", "error", err)
		os.Exit(1)
	}
	adminStatsHandler, err := adminstatshttp.NewHandler(logger, adminStatsService, authService)
	if err != nil {
		logger.Error("configure admin stats handler", "error", err)
		os.Exit(1)
	}
	adminSettingsRepo, err := adapterpostgres.NewAdminSettingsRepository(dbPool)
	if err != nil {
		logger.Error("configure admin settings repository", "error", err)
		os.Exit(1)
	}
	adminSettingsService, err := adminsettingsapp.NewService(adminSettingsRepo, cfg.AppName, cfg.AppVersion, poolStatsProvider(dbPool, cfg))
	if err != nil {
		logger.Error("configure admin settings service", "error", err)
		os.Exit(1)
	}
	adminSettingsHandler, err := adminsettingshttp.NewHandler(logger, adminSettingsService, authService)
	if err != nil {
		logger.Error("configure admin settings handler", "error", err)
		os.Exit(1)
	}
	securityLogRepo, err := adapterpostgres.NewSecurityLogRepository(dbPool)
	if err != nil {
		logger.Error("configure security log repository", "error", err)
		os.Exit(1)
	}
	securityLogService, err := securitylogapp.NewService(securityLogRepo, securitylogapp.CleanupConfig{
		ArchiveAfterDays: cfg.LogArchiveAfterDays,
		DeleteAfterDays:  cfg.LogDeleteAfterDays,
		BatchSize:        cfg.LogCleanupBatchSize,
		MaxLogCount:      cfg.LogMaxCount,
	})
	if err != nil {
		logger.Error("configure security log service", "error", err)
		os.Exit(1)
	}
	securityLogHandler, err := securityloghttp.NewHandler(logger, securityLogService, authService)
	if err != nil {
		logger.Error("configure security log handler", "error", err)
		os.Exit(1)
	}
	uploadService, err := uploadapp.NewService(uploadStorage)
	if err != nil {
		logger.Error("configure upload service", "error", err)
		os.Exit(1)
	}
	uploadHandler, err := uploadhttp.NewHandler(
		logger,
		uploadService,
		authService,
		uploadhttp.WithRedisRateLimit(redisClient, cfg.RedisFallbackCacheMaxSize),
	)
	if err != nil {
		logger.Error("configure upload handler", "error", err)
		os.Exit(1)
	}
	xidianRepo, err := adapterpostgres.NewXidianRepository(dbPool)
	if err != nil {
		logger.Error("configure xidian repository", "error", err)
		os.Exit(1)
	}
	xidianPortalClient, err := xidianintegration.NewClient(xidianintegration.Config{
		IDsBase:        cfg.XidianIDsBase,
		EhallBase:      cfg.XidianEhallBase,
		YjsptBase:      cfg.XidianYjsptBase,
		UserAgent:      cfg.XidianUserAgent,
		ConnectTimeout: cfg.XidianHTTPConnectTimeout,
		ReadTimeout:    cfg.XidianHTTPReadTimeout,
		RetryCount:     cfg.XidianSyncRetryCount,
		CaptchaWidth:   cfg.XidianCaptchaWidth,
	})
	if err != nil {
		logger.Error("configure xidian portal client", "error", err)
		os.Exit(1)
	}
	xidianService, err := xidianapp.NewService(xidianRepo, xidianPortalClient, appCipher, xidianapp.NewMemoryChallengeStore(cfg.RedisFallbackCacheMaxSize), xidianapp.Config{
		ChallengeTTL:            cfg.XidianChallengeTTL,
		SnapshotFallbackEnabled: cfg.XidianSnapshotFallbackEnabled,
		CaptchaWidth:            cfg.XidianCaptchaWidth,
		CaptchaHeight:           cfg.XidianCaptchaHeight,
		PieceWidth:              cfg.XidianPieceWidth,
		PieceHeight:             cfg.XidianPieceHeight,
	})
	if err != nil {
		logger.Error("configure xidian service", "error", err)
		os.Exit(1)
	}
	xidianHandler, err := xidianhttp.NewHandler(logger, xidianService, authService)
	if err != nil {
		logger.Error("configure xidian handler", "error", err)
		os.Exit(1)
	}

	store := metrics.NewStore(cfg.AppVersion, cfg.Environment)
	checker := health.NewChecker(cfg.AppVersion, dbPool, health.RedisPingerFunc(func(ctx context.Context) error {
		return redisClient.Ping(ctx).Err()
	}))

	handler, err := httpserver.NewHandler(
		cfg,
		logger,
		checker,
		store,
		httpserver.WithRoutes(func(mux *http.ServeMux) {
			authHandler.Register(mux, cfg.APIV1Prefix+"/auth")
			progressHandler.Register(mux, cfg.APIV1Prefix+"/progress")
			portraitHandler.Register(mux, cfg.APIV1Prefix+"/portrait")
			mistakeHandler.Register(mux, cfg.APIV1Prefix+"/mistakes")
			exerciseHandler.Register(mux, cfg.APIV1Prefix+"/exercise")
			sessionHandler.Register(mux, cfg.APIV1Prefix+"/session")
			resourceHandler.Register(mux, cfg.APIV1Prefix+"/resources")
			uploadHandler.Register(mux, cfg.APIV1Prefix+"/upload")
			xidianHandler.Register(mux, cfg.APIV1Prefix+"/xidian")
			questionHandler.Register(mux, cfg.APIV1Prefix+"/questions")
			classHandler.Register(mux, cfg.APIV1Prefix+"/classes")
			teacherHandler.Register(mux, cfg.APIV1Prefix+"/teacher")
			adminUserHandler.Register(mux, cfg.APIV1Prefix+"/admin/users")
			adminInboxHandler.Register(mux, cfg.APIV1Prefix+"/admin/inbox")
			adminAIConfigHandler.Register(mux, cfg.APIV1Prefix+"/admin/ai-config")
			adminStatsHandler.Register(mux, cfg.APIV1Prefix+"/admin/stats")
			adminSettingsHandler.Register(mux, cfg.APIV1Prefix+"/admin/settings")
			securityLogHandler.Register(mux, cfg.APIV1Prefix+"/admin/security-logs")
			knowledgeHandler.Register(mux, cfg.APIV1Prefix+"/admin/knowledge")
		}),
	)
	if err != nil {
		logger.Error("configure http handler", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr(),
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stopCh)

	logger.Info("Go API listening", "addr", cfg.HTTPAddr(), "environment", cfg.Environment)
	if err := serveHTTP(server, stopCh, cfg.ShutdownTimeout, logger); err != nil {
		os.Exit(1)
	}
}

func newLogger(cfg config.Config) *slog.Logger {
	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func requireSharedRedis(ctx context.Context, cfg config.Config, redisClient *goredis.Client) error {
	if !cfg.RequiresSharedRefreshSessionStore() {
		return nil
	}
	checkCtx, cancel := context.WithTimeout(ctx, cfg.RedisConnectTimeout+cfg.RedisSocketTimeout)
	defer cancel()
	return redisClient.Ping(checkCtx).Err()
}

func adminStatusProvider(dbPool *pgxpool.Pool, redisClient *goredis.Client) adminstatsapp.StatusProviderFunc {
	return func(ctx context.Context) ([]adminstatsapp.ServiceStatus, error) {
		return []adminstatsapp.ServiceStatus{
			pingStatus(ctx, "PostgreSQL", func(ctx context.Context) error { return dbPool.Ping(ctx) }),
			pingStatus(ctx, "Redis", func(ctx context.Context) error { return redisClient.Ping(ctx).Err() }),
		}, nil
	}
}

func pingStatus(ctx context.Context, name string, ping func(context.Context) error) adminstatsapp.ServiceStatus {
	start := time.Now()
	status := "running"
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := ping(checkCtx); err != nil {
		status = "stopped"
	}
	latency := float64(time.Since(start).Microseconds()) / 1000
	return adminstatsapp.ServiceStatus{Name: name, Status: status, LatencyMS: &latency}
}

func poolStatsProvider(dbPool *pgxpool.Pool, cfg config.Config) adminsettingsapp.PoolStatsProviderFunc {
	return func() adminsettingsapp.ConnectionPoolStatus {
		stats := dbPool.Stat()
		maxConns := int(stats.MaxConns())
		acquired := int(stats.AcquiredConns())
		idle := int(stats.IdleConns())
		usage := 0.0
		if maxConns > 0 {
			usage = float64(acquired) / float64(maxConns) * 100
		}
		return adminsettingsapp.ConnectionPoolStatus{
			PoolSize:     maxConns,
			MaxOverflow:  0,
			CheckedOut:   acquired,
			CheckedIn:    idle,
			Overflow:     0,
			PoolTimeout:  int(cfg.DBConnectTimeout.Seconds()),
			PoolRecycle:  int(cfg.DBPoolRecycle.Seconds()),
			UsagePercent: usage,
		}
	}
}
