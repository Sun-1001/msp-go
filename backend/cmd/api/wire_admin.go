package main

import (
	"context"
	"log/slog"

	emailadapter "mathstudy/backend/internal/adapter/email"
	adminemailhttp "mathstudy/backend/internal/adapter/http/adminemail"
	admininboxhttp "mathstudy/backend/internal/adapter/http/admininbox"
	adminsettingshttp "mathstudy/backend/internal/adapter/http/adminsettings"
	adminstatshttp "mathstudy/backend/internal/adapter/http/adminstats"
	adminstoragehttp "mathstudy/backend/internal/adapter/http/adminstorage"
	adminuserhttp "mathstudy/backend/internal/adapter/http/adminuser"
	announcementhttp "mathstudy/backend/internal/adapter/http/announcement"
	securityloghttp "mathstudy/backend/internal/adapter/http/securitylog"
	adapterpostgres "mathstudy/backend/internal/adapter/postgres"
	storageadapter "mathstudy/backend/internal/adapter/storage"
	admininboxapp "mathstudy/backend/internal/application/admininbox"
	adminsettingsapp "mathstudy/backend/internal/application/adminsettings"
	adminstatsapp "mathstudy/backend/internal/application/adminstats"
	adminstorageapp "mathstudy/backend/internal/application/adminstorage"
	adminuserapp "mathstudy/backend/internal/application/adminuser"
	announcementapp "mathstudy/backend/internal/application/announcement"
	emailapp "mathstudy/backend/internal/application/email"
	securitylogapp "mathstudy/backend/internal/application/securitylog"
	"mathstudy/backend/internal/platform/config"
)

type adminModule struct {
	emailService             *emailapp.Service
	adminEmailHandler        *adminemailhttp.Handler
	adminUserService         *adminuserapp.Service
	adminUserHandler         *adminuserhttp.Handler
	adminInboxService        *admininboxapp.Service
	adminInboxHandler        *admininboxhttp.Handler
	announcementHandler      *announcementhttp.Handler
	adminStatsHandler        *adminstatshttp.Handler
	adminSettingsHandler     *adminsettingshttp.Handler
	adminStorageHandler      *adminstoragehttp.Handler
	securityLogHandler       *securityloghttp.Handler
	securityLogCleanupWorker *securitylogapp.CleanupWorker
}

func wireAdmin(ctx context.Context, cfg config.Config, logger *slog.Logger, infra *infrastructure, auth *authModule, ai *aiModule, uploadStorage *storageadapter.RuntimeManager) (*adminModule, error) {
	accounts, err := wireAdminAccounts(logger, infra, auth, ai)
	if err != nil {
		return nil, err
	}
	ops, err := wireAdminOps(ctx, cfg, logger, infra, auth, ai, uploadStorage)
	if err != nil {
		return nil, err
	}
	return &adminModule{
		emailService:             accounts.emailService,
		adminEmailHandler:        accounts.adminEmailHandler,
		adminUserService:         accounts.adminUserService,
		adminUserHandler:         accounts.adminUserHandler,
		adminInboxService:        accounts.adminInboxService,
		adminInboxHandler:        accounts.adminInboxHandler,
		announcementHandler:      accounts.announcementHandler,
		adminStatsHandler:        ops.adminStatsHandler,
		adminSettingsHandler:     ops.adminSettingsHandler,
		adminStorageHandler:      ops.adminStorageHandler,
		securityLogHandler:       ops.securityLogHandler,
		securityLogCleanupWorker: ops.securityLogCleanupWorker,
	}, nil
}

type adminAccountsModule struct {
	emailService        *emailapp.Service
	adminEmailHandler   *adminemailhttp.Handler
	adminUserService    *adminuserapp.Service
	adminUserHandler    *adminuserhttp.Handler
	adminInboxService   *admininboxapp.Service
	adminInboxHandler   *admininboxhttp.Handler
	announcementHandler *announcementhttp.Handler
}

func wireAdminAccounts(logger *slog.Logger, infra *infrastructure, auth *authModule, ai *aiModule) (*adminAccountsModule, error) {
	emailRepo, err := adapterpostgres.NewEmailRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure email repository", err)
	}
	emailService, err := emailapp.NewService(emailRepo, ai.cipher, emailadapter.NewSMTPTransport(), logger)
	if err != nil {
		return nil, stepFailed("configure email service", err)
	}
	adminEmailHandler, err := adminemailhttp.NewHandler(logger, emailService, auth.service)
	if err != nil {
		return nil, stepFailed("configure admin email handler", err)
	}
	adminUserService, err := adminuserapp.NewService(auth.userRepo, emailService)
	if err != nil {
		return nil, stepFailed("configure admin user service", err)
	}
	adminUserHandler, err := adminuserhttp.NewHandler(logger, adminUserService, auth.service)
	if err != nil {
		return nil, stepFailed("configure admin user handler", err)
	}
	adminInboxService, err := admininboxapp.NewService(auth.userRepo, auth.loginLimiter)
	if err != nil {
		return nil, stepFailed("configure admin inbox service", err)
	}
	adminInboxService.SetEventSender(emailService)
	adminInboxHandler, err := admininboxhttp.NewHandler(logger, adminInboxService, auth.service)
	if err != nil {
		return nil, stepFailed("configure admin inbox handler", err)
	}
	announcementRepo, err := adapterpostgres.NewAnnouncementRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure announcement repository", err)
	}
	announcementService, err := announcementapp.NewService(announcementRepo)
	if err != nil {
		return nil, stepFailed("configure announcement service", err)
	}
	announcementHandler, err := announcementhttp.NewHandler(logger, announcementService, auth.service)
	if err != nil {
		return nil, stepFailed("configure announcement handler", err)
	}
	return &adminAccountsModule{
		emailService:        emailService,
		adminEmailHandler:   adminEmailHandler,
		adminUserService:    adminUserService,
		adminUserHandler:    adminUserHandler,
		adminInboxService:   adminInboxService,
		adminInboxHandler:   adminInboxHandler,
		announcementHandler: announcementHandler,
	}, nil
}

type adminOpsModule struct {
	adminStatsHandler        *adminstatshttp.Handler
	adminSettingsHandler     *adminsettingshttp.Handler
	adminStorageHandler      *adminstoragehttp.Handler
	securityLogHandler       *securityloghttp.Handler
	securityLogCleanupWorker *securitylogapp.CleanupWorker
}

func wireAdminOps(ctx context.Context, cfg config.Config, logger *slog.Logger, infra *infrastructure, auth *authModule, ai *aiModule, uploadStorage *storageadapter.RuntimeManager) (*adminOpsModule, error) {
	adminStatsRepo, err := adapterpostgres.NewAdminStatsRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure admin stats repository", err)
	}
	adminStatsService, err := adminstatsapp.NewService(adminStatsRepo, adminStatusProvider(infra.db, infra.redis, infra.qdrantPinger))
	if err != nil {
		return nil, stepFailed("configure admin stats service", err)
	}
	adminStatsService.SetOperationsProvider(adminOperationsProvider(infra.metrics))
	adminStatsService.SetOperationsResetter(adminstatsapp.OperationsResetterFunc(infra.metrics.ResetOperationalHTTP))
	adminStatsHandler, err := adminstatshttp.NewHandler(logger, adminStatsService, auth.service)
	if err != nil {
		return nil, stepFailed("configure admin stats handler", err)
	}
	adminSettingsRepo, err := adapterpostgres.NewAdminSettingsRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure admin settings repository", err)
	}
	adminSettingsService, err := adminsettingsapp.NewService(adminSettingsRepo, cfg.AppName, cfg.AppVersion, poolStatsProvider(infra.db, cfg))
	if err != nil {
		return nil, stepFailed("configure admin settings service", err)
	}
	adminSettingsHandler, err := adminsettingshttp.NewHandler(logger, adminSettingsService, auth.service)
	if err != nil {
		return nil, stepFailed("configure admin settings handler", err)
	}
	adminStorageService, err := adminstorageapp.NewService(
		adminSettingsRepo,
		ai.cipher,
		uploadStorage,
		uploadStorage.LocalConfigured(),
	)
	if err != nil {
		return nil, stepFailed("configure admin storage service", err)
	}
	if err := adminStorageService.ActivateStored(ctx); err != nil {
		return nil, stepFailed("activate storage settings", err)
	}
	adminStorageHandler, err := adminstoragehttp.NewHandler(logger, adminStorageService, auth.service)
	if err != nil {
		return nil, stepFailed("configure admin storage handler", err)
	}
	securityLogRepo, err := adapterpostgres.NewSecurityLogRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure security log repository", err)
	}
	securityLogService, err := securitylogapp.NewService(securityLogRepo, securitylogapp.CleanupConfig{
		ArchiveAfterDays: cfg.LogArchiveAfterDays,
		DeleteAfterDays:  cfg.LogDeleteAfterDays,
		BatchSize:        cfg.LogCleanupBatchSize,
		MaxBatches:       cfg.LogCleanupMaxBatches,
		MaxLogCount:      cfg.LogMaxCount,
	})
	if err != nil {
		return nil, stepFailed("configure security log service", err)
	}
	securityLogHandler, err := securityloghttp.NewHandler(logger, securityLogService, auth.service)
	if err != nil {
		return nil, stepFailed("configure security log handler", err)
	}
	securityLogCleanupWorker, err := securitylogapp.NewCleanupWorker(
		securityLogService,
		logger,
		securitylogapp.CleanupWorkerConfig{
			Interval: cfg.LogCleanupInterval,
			Timeout:  cfg.LogCleanupTimeout,
		},
	)
	if err != nil {
		return nil, stepFailed("configure security log cleanup worker", err)
	}
	return &adminOpsModule{
		adminStatsHandler:        adminStatsHandler,
		adminSettingsHandler:     adminSettingsHandler,
		adminStorageHandler:      adminStorageHandler,
		securityLogHandler:       securityLogHandler,
		securityLogCleanupWorker: securityLogCleanupWorker,
	}, nil
}
