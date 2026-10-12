package main

import (
	"context"
	"log/slog"

	authhttp "mathstudy/backend/internal/adapter/http/auth"
	adapterpostgres "mathstudy/backend/internal/adapter/postgres"
	authapp "mathstudy/backend/internal/application/auth"
	"mathstudy/backend/internal/platform/config"
)

type authModule struct {
	userRepo        adapterpostgres.UserRepository
	tokenService    authapp.TokenService
	loginLimiter    *authapp.LoginLimiter
	captcha         *authapp.SliderCaptchaManager
	refreshSessions *authapp.RefreshSessionStore
	service         *authapp.Service
	handler         *authhttp.Handler
}

func wireAuth(ctx context.Context, cfg config.Config, logger *slog.Logger, infra *infrastructure) (*authModule, error) {
	userRepo, err := adapterpostgres.NewUserRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure user repository", err)
	}
	tokenService, err := authapp.NewTokenService(
		cfg.JWTSecretKey,
		cfg.JWTAlgorithm,
		cfg.JWTAccessTokenExpire,
		cfg.JWTRefreshTokenExpire,
	)
	if err != nil {
		return nil, stepFailed("configure token service", err)
	}
	loginLimiter := authapp.NewLoginLimiter(infra.redis, cfg.LoginMaxAttempts, cfg.LoginLockout, logger)
	captchaManager, err := authapp.NewSliderCaptchaManager(infra.redis, logger, authapp.SliderCaptchaConfig{
		ChallengeTTL: cfg.LoginCaptchaTTL,
		ProofTTL:     cfg.LoginCaptchaProofTTL,
		IssueWindow:  cfg.LoginCaptchaIssueWindow,
		Tolerance:    cfg.LoginCaptchaTolerance,
		IssueLimit:   cfg.LoginCaptchaIssueLimit,
		MaxLocalSize: cfg.RedisFallbackCacheMaxSize,
		Strict:       cfg.RequiresSharedRefreshSessionStore(),
	})
	if err != nil {
		return nil, stepFailed("configure login captcha", err)
	}
	refreshSessions := authapp.NewRefreshSessionStore(
		infra.redis,
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
		return nil, stepFailed("configure auth service", err)
	}
	if _, err := authService.InitAdmin(ctx, cfg.AdminUsername, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		return nil, stepFailed("initialize admin account", err)
	}
	authHandler, err := authhttp.NewHandler(cfg, logger, authService, captchaManager)
	if err != nil {
		return nil, stepFailed("configure auth handler", err)
	}
	return &authModule{
		userRepo:        userRepo,
		tokenService:    tokenService,
		loginLimiter:    loginLimiter,
		captcha:         captchaManager,
		refreshSessions: refreshSessions,
		service:         authService,
		handler:         authHandler,
	}, nil
}
