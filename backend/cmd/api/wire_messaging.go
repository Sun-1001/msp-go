package main

import (
	"log/slog"

	conversationhttp "mathstudy/backend/internal/adapter/http/conversation"
	forumhttp "mathstudy/backend/internal/adapter/http/forum"
	messagecenterhttp "mathstudy/backend/internal/adapter/http/messagecenter"
	noticehttp "mathstudy/backend/internal/adapter/http/notice"
	qathreadhttp "mathstudy/backend/internal/adapter/http/qathread"
	adapterpostgres "mathstudy/backend/internal/adapter/postgres"
	conversationapp "mathstudy/backend/internal/application/conversation"
	forumapp "mathstudy/backend/internal/application/forum"
	messagecenterapp "mathstudy/backend/internal/application/messagecenter"
	noticeapp "mathstudy/backend/internal/application/notice"
	qathreadapp "mathstudy/backend/internal/application/qathread"
	"mathstudy/backend/internal/platform/config"
	"mathstudy/backend/internal/platform/ratelimit"
)

type messagingModule struct {
	writeLimiter         *ratelimit.Limiter
	searchLimiter        *ratelimit.Limiter
	conversationHandler  *conversationhttp.Handler
	noticeHandler        *noticehttp.Handler
	qaThreadHandler      *qathreadhttp.Handler
	forumHandler         *forumhttp.Handler
	messageCenterHandler *messagecenterhttp.Handler
}

func wireMessaging(cfg config.Config, logger *slog.Logger, infra *infrastructure, auth *authModule, enqueuer adapterpostgres.WechatReminderEnqueuer) (*messagingModule, error) {
	messageCenterWriteLimiter, err := ratelimit.New(
		infra.redis,
		"msp:message_center:write",
		messageCenterWriteRateLimitMax,
		messageCenterRateLimitWindow,
		cfg.RedisFallbackCacheMaxSize,
		logger,
	)
	if err != nil {
		return nil, stepFailed("configure message center write rate limit", err)
	}
	messageCenterSearchLimiter, err := ratelimit.New(
		infra.redis,
		"msp:message_center:search",
		messageCenterSearchRateLimitMax,
		messageCenterRateLimitWindow,
		cfg.RedisFallbackCacheMaxSize,
		logger,
	)
	if err != nil {
		return nil, stepFailed("configure message center search rate limit", err)
	}
	// Message center: conversations
	conversationRepo, err := adapterpostgres.NewConversationRepository(infra.db, enqueuer)
	if err != nil {
		return nil, stepFailed("configure conversation repository", err)
	}
	conversationService, err := conversationapp.NewService(conversationRepo)
	if err != nil {
		return nil, stepFailed("configure conversation service", err)
	}
	conversationHandler, err := conversationhttp.NewHandler(
		logger,
		conversationService,
		auth.service,
		conversationhttp.WithRateLimits(messageCenterWriteLimiter, messageCenterSearchLimiter),
	)
	if err != nil {
		return nil, stepFailed("configure conversation handler", err)
	}

	// Message center: notices
	noticeRepo, err := adapterpostgres.NewNoticeRepository(infra.db, enqueuer)
	if err != nil {
		return nil, stepFailed("configure notice repository", err)
	}
	noticeService, err := noticeapp.NewService(noticeRepo)
	if err != nil {
		return nil, stepFailed("configure notice service", err)
	}
	noticeHandler, err := noticehttp.NewHandler(
		logger,
		noticeService,
		auth.service,
		noticehttp.WithWriteRateLimit(messageCenterWriteLimiter),
		noticehttp.WithSearchRateLimit(messageCenterSearchLimiter),
	)
	if err != nil {
		return nil, stepFailed("configure notice handler", err)
	}

	// Message center: Q&A threads
	qaThreadRepo, err := adapterpostgres.NewQAThreadRepository(infra.db, enqueuer)
	if err != nil {
		return nil, stepFailed("configure qathread repository", err)
	}
	qaThreadService, err := qathreadapp.NewService(qaThreadRepo)
	if err != nil {
		return nil, stepFailed("configure qathread service", err)
	}
	qaThreadHandler, err := qathreadhttp.NewHandler(
		logger,
		qaThreadService,
		auth.service,
		qathreadhttp.WithWriteRateLimit(messageCenterWriteLimiter),
		qathreadhttp.WithSearchRateLimit(messageCenterSearchLimiter),
	)
	if err != nil {
		return nil, stepFailed("configure qathread handler", err)
	}

	// Message center: global forum
	forumRepo, err := adapterpostgres.NewForumRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure forum repository", err)
	}
	forumService, err := forumapp.NewService(forumRepo)
	if err != nil {
		return nil, stepFailed("configure forum service", err)
	}
	forumHandler, err := forumhttp.NewHandler(
		logger,
		forumService,
		auth.service,
		forumhttp.WithRateLimits(messageCenterWriteLimiter, messageCenterSearchLimiter),
	)
	if err != nil {
		return nil, stepFailed("configure forum handler", err)
	}

	messageCenterRepo, err := adapterpostgres.NewMessageCenterRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure message center repository", err)
	}
	messageCenterService, err := messagecenterapp.NewService(messageCenterRepo)
	if err != nil {
		return nil, stepFailed("configure message center service", err)
	}
	messageCenterHandler, err := messagecenterhttp.NewHandler(logger, messageCenterService, auth.service)
	if err != nil {
		return nil, stepFailed("configure message center handler", err)
	}
	return &messagingModule{
		writeLimiter:         messageCenterWriteLimiter,
		searchLimiter:        messageCenterSearchLimiter,
		conversationHandler:  conversationHandler,
		noticeHandler:        noticeHandler,
		qaThreadHandler:      qaThreadHandler,
		forumHandler:         forumHandler,
		messageCenterHandler: messageCenterHandler,
	}, nil
}
