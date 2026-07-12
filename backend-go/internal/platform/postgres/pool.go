package postgres

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mathstudy/backend-go/internal/platform/config"
)

// NewPool creates the shared PostgreSQL connection pool for the Go API.
func NewPool(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	if err := validatePoolLimits(cfg.DBPoolSize, cfg.DBPoolMinConns); err != nil {
		return nil, err
	}
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	// #nosec G115 -- validatePoolLimits proves both values fit in int32.
	poolCfg.MaxConns = int32(cfg.DBPoolSize)
	// #nosec G115 -- validatePoolLimits proves both values fit in int32.
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

func validatePoolLimits(maxConns, minConns int) error {
	if maxConns <= 0 || maxConns > math.MaxInt32 {
		return fmt.Errorf("DB_POOL_SIZE must be between 1 and %d", math.MaxInt32)
	}
	if minConns < 0 || minConns > math.MaxInt32 {
		return fmt.Errorf("DB_POOL_MIN_CONNS must be between 0 and %d", math.MaxInt32)
	}
	if minConns > maxConns {
		return fmt.Errorf("DB_POOL_MIN_CONNS must not exceed DB_POOL_SIZE")
	}
	return nil
}

func milliseconds(duration time.Duration) string {
	return strconv.FormatInt(duration.Milliseconds(), 10)
}
