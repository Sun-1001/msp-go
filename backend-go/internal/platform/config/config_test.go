package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadUsesEnvironmentAndBuildsAddresses(t *testing.T) {
	t.Setenv("GO_API_HOST", "127.0.0.1")
	t.Setenv("GO_API_PORT", "18080")
	t.Setenv("API_V1_PREFIX", "api/v1")
	t.Setenv("POSTGRES_HOST", "db")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_USER", "user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_DB", "msp")
	t.Setenv("DB_POOL_MIN_CONNS", "2")
	t.Setenv("DB_STATEMENT_TIMEOUT_MS", "1500")
	t.Setenv("DB_IDLE_TX_TIMEOUT_MS", "45000")
	t.Setenv("REDIS_HOST", "cache")
	t.Setenv("REDIS_PORT", "6380")
	t.Setenv("REDIS_FALLBACK_CACHE_MAX_SIZE", "20")
	t.Setenv("REQUEST_TIMEOUT_DEFAULT", "2.5")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.HTTPAddr() != "127.0.0.1:18080" {
		t.Fatalf("HTTPAddr() = %q", cfg.HTTPAddr())
	}
	if cfg.APIV1Prefix != "/api/v1" {
		t.Fatalf("APIV1Prefix = %q", cfg.APIV1Prefix)
	}
	if cfg.RedisAddr() != "cache:6380" {
		t.Fatalf("RedisAddr() = %q", cfg.RedisAddr())
	}
	if cfg.RequestTimeout != 2500*time.Millisecond {
		t.Fatalf("RequestTimeout = %s", cfg.RequestTimeout)
	}
	if cfg.DBPoolMinConns != 2 {
		t.Fatalf("DBPoolMinConns = %d", cfg.DBPoolMinConns)
	}
	if cfg.DBStatementTimeout != 1500*time.Millisecond {
		t.Fatalf("DBStatementTimeout = %s", cfg.DBStatementTimeout)
	}
	if cfg.DBIdleTxTimeout != 45*time.Second {
		t.Fatalf("DBIdleTxTimeout = %s", cfg.DBIdleTxTimeout)
	}
	if cfg.RedisFallbackCacheMaxSize != 20 {
		t.Fatalf("RedisFallbackCacheMaxSize = %d", cfg.RedisFallbackCacheMaxSize)
	}
	if !strings.Contains(cfg.DatabaseURL(), "postgres://user:secret@db:5433/msp") {
		t.Fatalf("DatabaseURL() = %q", cfg.DatabaseURL())
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("GO_API_PORT", "70000")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid port error")
	}
}

func TestLoadRejectsInvalidPoolMinConns(t *testing.T) {
	t.Setenv("DB_POOL_SIZE", "2")
	t.Setenv("DB_POOL_MIN_CONNS", "3")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid pool min conns error")
	}
}
