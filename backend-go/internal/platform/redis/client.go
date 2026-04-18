package redis

import (
	goredis "github.com/redis/go-redis/v9"

	"mathstudy/backend-go/internal/platform/config"
)

// NewClient creates the shared Redis client for cache, rate limit, and short-lived state.
func NewClient(cfg config.Config) *goredis.Client {
	return goredis.NewClient(&goredis.Options{
		Addr:         cfg.RedisAddr(),
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		PoolSize:     cfg.RedisMaxConnections,
		DialTimeout:  cfg.RedisConnectTimeout,
		ReadTimeout:  cfg.RedisSocketTimeout,
		WriteTimeout: cfg.RedisSocketTimeout,
	})
}
