package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	einoagent "mathstudy/backend/internal/adapter/llm/einoagent"
	adapterpostgres "mathstudy/backend/internal/adapter/postgres"
	storageadapter "mathstudy/backend/internal/adapter/storage"
	adminsettingsapp "mathstudy/backend/internal/application/adminsettings"
	adminstatsapp "mathstudy/backend/internal/application/adminstats"
	"mathstudy/backend/internal/platform/config"
	"mathstudy/backend/internal/platform/health"
	"mathstudy/backend/internal/platform/httpserver"
	"mathstudy/backend/internal/platform/metrics"
)

const (
	messageCenterWriteRateLimitMax  = 60
	messageCenterSearchRateLimitMax = 30
	messageCenterRateLimitWindow    = time.Minute
	dailyQuestionGenerationLimitMax = 10
	dailyQuestionGenerationWindow   = time.Minute
	openAIResponsesRateLimitMax     = 30
	openAIResponsesRateLimitWindow  = time.Minute
)

var errServeFailed = errors.New("serve http")

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg)
	slog.SetDefault(logger)

	if err := run(ctx, cfg, logger); err != nil {
		var se *startupError
		if errors.As(err, &se) {
			logger.Error(se.step, se.attrs...)
		}
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	infra, cleanup, err := newInfrastructure(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer cleanup()

	auth, err := wireAuth(ctx, cfg, logger, infra)
	if err != nil {
		return err
	}
	riskRepo, riskSlots, err := wireAIRiskStores(infra)
	if err != nil {
		return err
	}
	progress, err := wireProgress(logger, infra, auth)
	if err != nil {
		return err
	}
	ai, err := wireAI(cfg, logger, infra, auth, riskRepo, riskSlots)
	if err != nil {
		return err
	}
	portrait, err := wirePortrait(logger, infra, auth, ai)
	if err != nil {
		return err
	}
	uploadStorage := storageadapter.NewRuntimeManager(cfg.UploadsDir)
	exercise, err := wireExercise(cfg, logger, infra, auth, ai, uploadStorage)
	if err != nil {
		return err
	}
	wechatReminderEnqueuer, err := adapterpostgres.NewWechatReminderEnqueuer(
		cfg.WechatMessageRemindersEnabled,
		cfg.WechatOfficialAccountAppID,
	)
	if err != nil {
		return stepFailed("configure wechat reminder enqueuer", err)
	}
	dailyQuestion, err := wireDailyQuestion(cfg, logger, infra, auth, ai, exercise, wechatReminderEnqueuer)
	if err != nil {
		return err
	}
	resource, err := wireResource(cfg, logger, infra, auth, ai, uploadStorage)
	if err != nil {
		return err
	}
	session, err := wireSession(logger, infra, auth, ai, exercise, resource)
	if err != nil {
		return err
	}
	question, err := wireQuestion(logger, infra, auth, ai)
	if err != nil {
		return err
	}
	classroom, err := wireClassroom(logger, infra, auth)
	if err != nil {
		return err
	}
	messaging, err := wireMessaging(cfg, logger, infra, auth, wechatReminderEnqueuer)
	if err != nil {
		return err
	}
	admin, err := wireAdmin(ctx, cfg, logger, infra, auth, ai, uploadStorage)
	if err != nil {
		return err
	}
	integrations, err := wireIntegrations(cfg, logger, infra, auth, uploadStorage)
	if err != nil {
		return err
	}

	checker := health.NewChecker(cfg.AppVersion, infra.db, health.RedisPingerFunc(func(ctx context.Context) error {
		return infra.redis.Ping(ctx).Err()
	}), infra.qdrantPinger)

	handler, err := httpserver.NewHandler(
		cfg,
		logger,
		checker,
		infra.metrics,
		httpserver.WithRoutes(func(mux *http.ServeMux) {
			registerRoutes(mux, cfg.APIV1Prefix, ai, auth, progress, portrait, exercise, dailyQuestion, resource, session, question, classroom, messaging, admin, integrations)
		}),
	)
	if err != nil {
		return stepFailed("configure http handler", err)
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

	stopSecurityLogCleanupWorker := startSecurityLogCleanupWorker(
		admin.securityLogCleanupWorker,
		cfg.LogCleanupEnabled,
		cfg.ShutdownTimeout,
		logger,
	)
	stopWechatReminderWorker := startWechatReminderWorker(
		integrations.wechatReminderWorker,
		cfg.WechatMessageRemindersEnabled,
		cfg.ShutdownTimeout,
		logger,
	)
	stopDailyQuestionWorker := startDailyQuestionWorker(
		dailyQuestion.worker,
		cfg.DailyQuestionPreGenerationEnabled,
		cfg.ShutdownTimeout,
		logger,
	)
	stopDailyQuestionReminderWorker := startDailyQuestionReminderWorker(
		dailyQuestion.reminderWorker,
		cfg.WechatMessageRemindersEnabled,
		cfg.ShutdownTimeout,
		logger,
	)
	logger.Info("Go API listening", "addr", cfg.HTTPAddr(), "environment", cfg.Environment)
	serveErr := serveHTTP(server, stopCh, cfg.ShutdownTimeout, logger)
	stopSecurityLogCleanupWorker()
	stopDailyQuestionReminderWorker()
	stopDailyQuestionWorker()
	stopWechatReminderWorker()
	if serveErr != nil {
		return errServeFailed
	}
	return nil
}

func newLogger(cfg config.Config) *slog.Logger {
	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func defaultEinoConfig(cfg config.Config) einoagent.Config {
	return einoagent.Config{
		Enabled:       cfg.EinoEnabled,
		BaseURL:       cfg.EinoBaseURL,
		APIKey:        cfg.EinoAPIKey,
		Model:         cfg.EinoModel,
		Timeout:       cfg.EinoTimeout,
		Temperature:   cfg.EinoTemperature,
		MaxTokens:     cfg.EinoMaxTokens,
		MaxIterations: cfg.EinoMaxIterations,
	}
}

func requireSharedRedis(ctx context.Context, cfg config.Config, redisClient *goredis.Client) error {
	if !cfg.RequiresSharedRefreshSessionStore() && !cfg.WechatOfficialAccountEnabled {
		return nil
	}
	checkCtx, cancel := context.WithTimeout(ctx, cfg.RedisConnectTimeout+cfg.RedisSocketTimeout)
	defer cancel()
	return redisClient.Ping(checkCtx).Err()
}

func adminStatusProvider(dbPool *pgxpool.Pool, redisClient *goredis.Client, optional ...health.Pinger) adminstatsapp.StatusProviderFunc {
	return func(ctx context.Context) ([]adminstatsapp.ServiceStatus, error) {
		statuses := []adminstatsapp.ServiceStatus{
			pingStatus(ctx, "PostgreSQL", func(ctx context.Context) error { return dbPool.Ping(ctx) }),
			pingStatus(ctx, "Redis", func(ctx context.Context) error { return redisClient.Ping(ctx).Err() }),
		}
		if len(optional) > 0 && optional[0] != nil {
			statuses = append(statuses, pingStatus(ctx, "Qdrant", optional[0].Ping))
		}
		return statuses, nil
	}
}

func adminOperationsProvider(store *metrics.Store) adminstatsapp.OperationsProviderFunc {
	return func() adminstatsapp.OperationsSnapshot {
		snapshot := store.OperationalSnapshot()
		return adminstatsapp.OperationsSnapshot{
			Version:           snapshot.Version,
			Environment:       snapshot.Environment,
			StartedAt:         snapshot.StartedAt,
			Uptime:            snapshot.Uptime,
			CPUUsagePercent:   snapshot.Process.CPUUsagePercent,
			HeapUsedBytes:     snapshot.Process.HeapUsedBytes,
			HeapReservedBytes: snapshot.Process.HeapReservedBytes,
			Goroutines:        snapshot.Process.Goroutines,
			LogicalCPUs:       snapshot.Process.LogicalCPUs,
			GOMAXPROCS:        snapshot.Process.GOMAXPROCS,
			GoVersion:         snapshot.Process.GoVersion,
			OS:                snapshot.Process.OS,
			Arch:              snapshot.Process.Arch,
			RequestsTotal:     snapshot.HTTP.RequestsTotal,
			ClientErrorsTotal: snapshot.HTTP.ClientErrorsTotal,
			ServerErrorsTotal: snapshot.HTTP.ServerErrorsTotal,
			AverageLatency:    snapshot.HTTP.AverageDuration,
			P95Latency:        snapshot.HTTP.P95Duration,
			P95Clamped:        snapshot.HTTP.P95Clamped,
			TrafficStartedAt:  snapshot.TrafficStartedAt,
			TrafficWindow:     snapshot.TrafficWindow,
			PostgreSQL: adminstatsapp.DatabasePoolSnapshot{
				MaxConnections:       snapshot.Dependencies.Postgres.MaxConnections,
				TotalConnections:     snapshot.Dependencies.Postgres.TotalConnections,
				AcquiredConnections:  snapshot.Dependencies.Postgres.AcquiredConnections,
				IdleConnections:      snapshot.Dependencies.Postgres.IdleConnections,
				EmptyAcquireCount:    snapshot.Dependencies.Postgres.EmptyAcquireCount,
				CanceledAcquireCount: snapshot.Dependencies.Postgres.CanceledAcquireCount,
			},
			Redis: adminstatsapp.RedisPoolSnapshot{
				MaxConnections:   snapshot.Dependencies.Redis.MaxConnections,
				TotalConnections: snapshot.Dependencies.Redis.TotalConnections,
				IdleConnections:  snapshot.Dependencies.Redis.IdleConnections,
				StaleConnections: snapshot.Dependencies.Redis.StaleConnections,
				Hits:             snapshot.Dependencies.Redis.Hits,
				Misses:           snapshot.Dependencies.Redis.Misses,
				Timeouts:         snapshot.Dependencies.Redis.Timeouts,
				WaitCount:        snapshot.Dependencies.Redis.WaitCount,
				Unusable:         snapshot.Dependencies.Redis.Unusable,
			},
		}
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
