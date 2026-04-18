package postgres

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mathstudy/backend-go/internal/platform/config"
)

// NewPool creates the shared PostgreSQL connection pool for the Go API.
func NewPool(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	poolCfg.MaxConns = int32(cfg.DBPoolSize)
	poolCfg.MinConns = int32(cfg.DBPoolMinConns)
	poolCfg.MaxConnLifetime = cfg.DBPoolRecycle
	poolCfg.HealthCheckPeriod = cfg.DBHealthCheckPeriod
	poolCfg.ConnConfig.ConnectTimeout = cfg.DBConnectTimeout

	if poolCfg.ConnConfig.RuntimeParams == nil {
		poolCfg.ConnConfig.RuntimeParams = make(map[string]string)
	}
	poolCfg.ConnConfig.RuntimeParams["statement_timeout"] = milliseconds(cfg.DBStatementTimeout)
	poolCfg.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = milliseconds(cfg.DBIdleTxTimeout)

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	return pool, nil
}

func milliseconds(duration time.Duration) string {
	return strconv.FormatInt(duration.Milliseconds(), 10)
}
