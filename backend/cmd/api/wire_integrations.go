package main

import (
	"log/slog"
	"time"

	uploadhttp "mathstudy/backend/internal/adapter/http/upload"
	wechathttp "mathstudy/backend/internal/adapter/http/wechat"
	xidianhttp "mathstudy/backend/internal/adapter/http/xidian"
	adapterpostgres "mathstudy/backend/internal/adapter/postgres"
	adapterredis "mathstudy/backend/internal/adapter/redis"
	storageadapter "mathstudy/backend/internal/adapter/storage"
	uploadapp "mathstudy/backend/internal/application/upload"
	wechatapp "mathstudy/backend/internal/application/wechat"
	wechatreminder "mathstudy/backend/internal/application/wechatreminder"
	xidianapp "mathstudy/backend/internal/application/xidian"
	wechatintegration "mathstudy/backend/internal/integration/wechat"
	xidianintegration "mathstudy/backend/internal/integration/xidian"
	"mathstudy/backend/internal/platform/config"
	"mathstudy/backend/internal/platform/outbound"
)

type integrationsModule struct {
	uploadHandler        *uploadhttp.Handler
	xidianHandler        *xidianhttp.Handler
	wechatHandler        *wechathttp.Handler
	wechatReminderWorker *wechatreminder.Worker
}

func wireIntegrations(cfg config.Config, logger *slog.Logger, infra *infrastructure, auth *authModule, uploadStorage *storageadapter.RuntimeManager) (*integrationsModule, error) {
	uploadXidian, err := wireUploadXidian(cfg, logger, infra, auth, uploadStorage)
	if err != nil {
		return nil, err
	}
	wechat, err := wireWechat(cfg, logger, infra, auth)
	if err != nil {
		return nil, err
	}
	return &integrationsModule{
		uploadHandler:        uploadXidian.uploadHandler,
		xidianHandler:        uploadXidian.xidianHandler,
		wechatHandler:        wechat.wechatHandler,
		wechatReminderWorker: wechat.wechatReminderWorker,
	}, nil
}

type uploadXidianModule struct {
	uploadHandler *uploadhttp.Handler
	xidianHandler *xidianhttp.Handler
}

func wireUploadXidian(cfg config.Config, logger *slog.Logger, infra *infrastructure, auth *authModule, uploadStorage *storageadapter.RuntimeManager) (*uploadXidianModule, error) {
	uploadService, err := uploadapp.NewService(uploadStorage)
	if err != nil {
		return nil, stepFailed("configure upload service", err)
	}
	uploadAccessRepo, err := adapterpostgres.NewUploadAccessRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure upload access repository", err)
	}
	uploadHandler, err := uploadhttp.NewHandler(
		logger,
		uploadService,
		auth.service,
		uploadhttp.WithRedisRateLimit(infra.redis, cfg.RedisFallbackCacheMaxSize),
		uploadhttp.WithProtectedLocalDownloads(cfg.UploadsDir, uploadAccessRepo),
	)
	if err != nil {
		return nil, stepFailed("configure upload handler", err)
	}
	xidianRepo, err := adapterpostgres.NewXidianRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure xidian repository", err)
	}
	xidianPortalClient, err := xidianintegration.NewClient(xidianintegration.Config{
		IDsBase:        cfg.XidianIDsBase,
		EhallBase:      cfg.XidianEhallBase,
		UserAgent:      cfg.XidianUserAgent,
		ConnectTimeout: cfg.XidianHTTPConnectTimeout,
		ReadTimeout:    cfg.XidianHTTPReadTimeout,
		RetryCount:     cfg.XidianHTTPRetryCount,
		CaptchaWidth:   cfg.XidianCaptchaWidth,
	})
	if err != nil {
		return nil, stepFailed("configure xidian portal client", err)
	}
	xidianService, err := xidianapp.NewService(xidianRepo, xidianPortalClient, xidianapp.NewMemoryChallengeStore(cfg.RedisFallbackCacheMaxSize), xidianapp.Config{
		ChallengeTTL:  cfg.XidianChallengeTTL,
		CaptchaWidth:  cfg.XidianCaptchaWidth,
		CaptchaHeight: cfg.XidianCaptchaHeight,
		PieceWidth:    cfg.XidianPieceWidth,
		PieceHeight:   cfg.XidianPieceHeight,
	})
	if err != nil {
		return nil, stepFailed("configure xidian service", err)
	}
	xidianHandler, err := xidianhttp.NewHandler(logger, xidianService, auth.service)
	if err != nil {
		return nil, stepFailed("configure xidian handler", err)
	}
	return &uploadXidianModule{uploadHandler: uploadHandler, xidianHandler: xidianHandler}, nil
}

type wechatModule struct {
	wechatHandler        *wechathttp.Handler
	wechatReminderWorker *wechatreminder.Worker
}

func wireWechat(cfg config.Config, logger *slog.Logger, infra *infrastructure, auth *authModule) (*wechatModule, error) {
	wechatRepo, err := adapterpostgres.NewWechatRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure wechat repository", err)
	}
	wechatState, err := adapterredis.NewWechatStateStore(infra.redis)
	if err != nil {
		return nil, stepFailed("configure wechat state store", err)
	}
	var wechatProtocol *wechatintegration.Protocol
	var wechatTextSender wechatapp.Sender
	var wechatTemplateSender wechatreminder.Sender
	if cfg.WechatOfficialAccountEnabled {
		wechatProtocol, err = wechatintegration.NewProtocol(
			cfg.WechatOfficialAccountAppID,
			cfg.WechatOfficialAccountToken,
			cfg.WechatOfficialAccountAESKey,
		)
		if err != nil {
			return nil, stepFailed("configure wechat callback protocol", err)
		}
		wechatAPIClient, clientErr := wechatintegration.NewAPIClient(
			cfg.WechatOfficialAccountAppID,
			cfg.WechatOfficialAccountAppSecret,
			outbound.NewPublicHTTPSClient(cfg.WechatOfficialAccountHTTPTimeout),
			wechatState,
		)
		if clientErr != nil {
			return nil, stepFailed("configure wechat API client", clientErr)
		}
		wechatTextSender = wechatAPIClient
		wechatTemplateSender = wechatAPIClient
	}
	wechatService, err := wechatapp.NewService(wechatRepo, wechatState, wechatTextSender, wechatapp.Config{
		Enabled:            cfg.WechatOfficialAccountEnabled,
		AppID:              cfg.WechatOfficialAccountAppID,
		AccountName:        cfg.WechatOfficialAccountName,
		BindingTicketTTL:   10 * time.Minute,
		EventDedupeTTL:     24 * time.Hour,
		EventProcessingTTL: 6 * time.Second,
	})
	if err != nil {
		return nil, stepFailed("configure wechat service", err)
	}
	wechatHandler, err := wechathttp.NewHandler(logger, wechatService, auth.service, wechatProtocol, wechathttp.Config{
		Enabled:     cfg.WechatOfficialAccountEnabled,
		MessageMode: cfg.WechatOfficialAccountMessageMode,
	}, wechathttp.WithRedisRateLimit(infra.redis, cfg.RedisFallbackCacheMaxSize))
	if err != nil {
		return nil, stepFailed("configure wechat handler", err)
	}
	wechatReminderRepo, err := adapterpostgres.NewWechatReminderRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure wechat reminder repository", err)
	}
	wechatReminderWorker, err := wechatreminder.NewWorker(
		wechatReminderRepo,
		wechatTemplateSender,
		wechatreminder.ClassifySendError,
		logger,
		wechatreminder.Config{
			Enabled:                  cfg.WechatMessageRemindersEnabled,
			AppID:                    cfg.WechatOfficialAccountAppID,
			PrivateMessageTemplateID: cfg.WechatPrivateMessageTemplateID,
			NoticeTemplateID:         cfg.WechatNoticeTemplateID,
			QAMessageTemplateID:      cfg.WechatQAMessageTemplateID,
			SendTimeout:              cfg.WechatOfficialAccountHTTPTimeout,
		},
	)
	if err != nil {
		return nil, stepFailed("configure wechat reminder worker", err)
	}
	return &wechatModule{wechatHandler: wechatHandler, wechatReminderWorker: wechatReminderWorker}, nil
}
