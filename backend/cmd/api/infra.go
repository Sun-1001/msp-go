package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	qdrantadapter "mathstudy/backend/internal/adapter/qdrant"
	"mathstudy/backend/internal/platform/config"
	"mathstudy/backend/internal/platform/health"
	"mathstudy/backend/internal/platform/metrics"
	platformpostgres "mathstudy/backend/internal/platform/postgres"
	platformredis "mathstudy/backend/internal/platform/redis"
)

type infrastructure struct {
	db           *pgxpool.Pool
	redis        *goredis.Client
	qdrant       *qdrantadapter.Client
	qdrantPinger health.Pinger
	metrics      *metrics.Store
}

func newInfrastructure(ctx context.Context, cfg config.Config, logger *slog.Logger) (*infrastructure, func(), error) {
	dbPool, err := platformpostgres.NewPool(ctx, cfg)
	if err != nil {
		return nil, nil, stepFailed("configure postgres pool", err)
	}
	redisClient := platformredis.NewClient(cfg)
	cleanup := func() {
		if err := redisClient.Close(); err != nil {
			logger.Warn("close redis client", "error", err)
		}
		dbPool.Close()
	}
	fail := func(err error) (*infrastructure, func(), error) {
		cleanup()
		return nil, nil, err
	}
	if err := requireSharedRedis(ctx, cfg, redisClient); err != nil {
		return fail(stepFailed("redis is required for configured shared state", err))
	}
	var qdrantClient *qdrantadapter.Client
	var qdrantPinger health.Pinger
	if cfg.QdrantEnabled {
		qdrantClient, err = qdrantadapter.New(qdrantadapter.Config{
			BaseURL:        cfg.QdrantURL,
			APIKey:         cfg.QdrantAPIKey,
			Collection:     cfg.QdrantCollection,
			Timeout:        cfg.QdrantTimeout,
			HealthTimeout:  cfg.QdrantHealthTimeout,
			MaxBatchSize:   cfg.QdrantMaxBatchSize,
			SearchHNSWEF:   cfg.QdrantSearchHNSWEF,
			WaitForChanges: cfg.QdrantWaitForChanges,
			PayloadIndexes: cfg.QdrantPayloadIndexFields,
			CAFile:         cfg.QdrantCAFile, ShardNumber: cfg.QdrantShardNumber,
			ReplicationFactor: cfg.QdrantReplicationFactor, WriteConsistencyFactor: cfg.QdrantWriteConsistency,
		}, qdrantadapter.WithResourceCollections())
		if err != nil {
			return fail(stepFailed("configure qdrant vector index", err))
		}
		qdrantPinger = qdrantClient
		logger.Info("qdrant vector capability enabled")
	}
	store := metrics.NewStore(cfg.AppVersion, cfg.Environment)
	store.SetRuntimeStatsProvider(func() metrics.RuntimeStats {
		postgresStats := dbPool.Stat()
		redisStats := redisClient.PoolStats()
		return metrics.RuntimeStats{
			Postgres: metrics.PostgresPoolStats{
				MaxConnections:          int64(postgresStats.MaxConns()),
				TotalConnections:        int64(postgresStats.TotalConns()),
				AcquiredConnections:     int64(postgresStats.AcquiredConns()),
				IdleConnections:         int64(postgresStats.IdleConns()),
				ConstructingConnections: int64(postgresStats.ConstructingConns()),
				AcquireCount:            postgresStats.AcquireCount(),
				EmptyAcquireCount:       postgresStats.EmptyAcquireCount(),
				CanceledAcquireCount:    postgresStats.CanceledAcquireCount(),
				AcquireDuration:         postgresStats.AcquireDuration(),
				EmptyAcquireWaitTime:    postgresStats.EmptyAcquireWaitTime(),
			},
			Redis: metrics.RedisPoolStats{
				MaxConnections:   int64(cfg.RedisMaxConnections),
				TotalConnections: int64(redisStats.TotalConns),
				IdleConnections:  int64(redisStats.IdleConns),
				StaleConnections: int64(redisStats.StaleConns),
				Hits:             uint64(redisStats.Hits),
				Misses:           uint64(redisStats.Misses),
				Timeouts:         uint64(redisStats.Timeouts),
				WaitCount:        uint64(redisStats.WaitCount),
				Unusable:         uint64(redisStats.Unusable),
				WaitDuration:     time.Duration(redisStats.WaitDurationNs),
			},
		}
	})
	return &infrastructure{
		db:           dbPool,
		redis:        redisClient,
		qdrant:       qdrantClient,
		qdrantPinger: qdrantPinger,
		metrics:      store,
	}, cleanup, nil
}
