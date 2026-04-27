package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	authhttp "mathstudy/backend-go/internal/adapter/http/auth"
	classroomhttp "mathstudy/backend-go/internal/adapter/http/classroom"
	exercisehttp "mathstudy/backend-go/internal/adapter/http/exercise"
	mistakehttp "mathstudy/backend-go/internal/adapter/http/mistake"
	portraithttp "mathstudy/backend-go/internal/adapter/http/portrait"
	progresshttp "mathstudy/backend-go/internal/adapter/http/progress"
	resourcehttp "mathstudy/backend-go/internal/adapter/http/resource"
	sessionhttp "mathstudy/backend-go/internal/adapter/http/session"
	adapterpostgres "mathstudy/backend-go/internal/adapter/postgres"
	authapp "mathstudy/backend-go/internal/application/auth"
	classroomapp "mathstudy/backend-go/internal/application/classroom"
	exerciseapp "mathstudy/backend-go/internal/application/exercise"
	mistakeapp "mathstudy/backend-go/internal/application/mistake"
	portraitapp "mathstudy/backend-go/internal/application/portrait"
	progressapp "mathstudy/backend-go/internal/application/progress"
	resourceapp "mathstudy/backend-go/internal/application/resource"
	sessionapp "mathstudy/backend-go/internal/application/session"
	"mathstudy/backend-go/internal/platform/config"
	"mathstudy/backend-go/internal/platform/health"
	"mathstudy/backend-go/internal/platform/httpserver"
	"mathstudy/backend-go/internal/platform/metrics"
	platformpostgres "mathstudy/backend-go/internal/platform/postgres"
	platformredis "mathstudy/backend-go/internal/platform/redis"
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
	authService, err := authapp.NewService(userRepo, userRepo, userRepo, tokenService, loginLimiter, logger)
	if err != nil {
		logger.Error("configure auth service", "error", err)
		os.Exit(1)
	}
	if _, err := authService.InitAdmin(ctx, cfg.AdminUsername, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		logger.Error("initialize admin account", "error", err)
		os.Exit(1)
	}
	authHandler, err := authhttp.NewHandler(cfg, logger, authService)
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
	portraitRepo, err := adapterpostgres.NewPortraitRepository(dbPool)
	if err != nil {
		logger.Error("configure portrait repository", "error", err)
		os.Exit(1)
	}
	portraitService, err := portraitapp.NewService(portraitRepo)
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
	exerciseRepo, err := adapterpostgres.NewExerciseRepository(dbPool)
	if err != nil {
		logger.Error("configure exercise repository", "error", err)
		os.Exit(1)
	}
	exerciseService, err := exerciseapp.NewService(exerciseRepo, nil)
	if err != nil {
		logger.Error("configure exercise service", "error", err)
		os.Exit(1)
	}
	exerciseHandler, err := exercisehttp.NewHandler(logger, exerciseService, authService)
	if err != nil {
		logger.Error("configure exercise handler", "error", err)
		os.Exit(1)
	}
	sessionRepo, err := adapterpostgres.NewSessionRepository(dbPool)
	if err != nil {
		logger.Error("configure session repository", "error", err)
		os.Exit(1)
	}
	sessionService, err := sessionapp.NewService(sessionRepo)
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
			classHandler.Register(mux, cfg.APIV1Prefix+"/classes")
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

	errCh := make(chan error, 1)
	go func() {
		logger.Info("Go API listening", "addr", cfg.HTTPAddr(), "environment", cfg.Environment)
		errCh <- server.ListenAndServe()
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-stopCh:
		logger.Info("shutdown requested", "signal", sig.String())
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("server shutdown complete")
}

func newLogger(cfg config.Config) *slog.Logger {
	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
