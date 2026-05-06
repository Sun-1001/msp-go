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
	t.Setenv("JWT_SECRET_KEY", "test-secret")
	t.Setenv("JWT_ALGORITHM", "hs512")
	t.Setenv("JWT_ACCESS_TOKEN_EXPIRE_MINUTES", "45")
	t.Setenv("JWT_REFRESH_TOKEN_EXPIRE_DAYS", "10")
	t.Setenv("ADMIN_USERNAME", "root")
	t.Setenv("ADMIN_EMAIL", "root@example.com")
	t.Setenv("ADMIN_PASSWORD", "Root1!")
	t.Setenv("LOGIN_MAX_ATTEMPTS", "3")
	t.Setenv("LOGIN_LOCKOUT_MINUTES", "9")
	t.Setenv("LOG_ARCHIVE_AFTER_DAYS", "14")
	t.Setenv("LOG_DELETE_AFTER_DAYS", "60")
	t.Setenv("LOG_CLEANUP_BATCH_SIZE", "250")
	t.Setenv("LOG_MAX_COUNT", "5000")

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
	if cfg.JWTAlgorithm != "HS512" || cfg.JWTAccessTokenExpire != 45*time.Minute || cfg.JWTRefreshTokenExpire != 10*24*time.Hour {
		t.Fatalf("JWT config = %s/%s/%s", cfg.JWTAlgorithm, cfg.JWTAccessTokenExpire, cfg.JWTRefreshTokenExpire)
	}
	if cfg.AdminUsername != "root" || cfg.AdminEmail != "root@example.com" || cfg.AdminPassword != "Root1!" {
		t.Fatalf("admin config = %s/%s/%s", cfg.AdminUsername, cfg.AdminEmail, cfg.AdminPassword)
	}
	if cfg.LoginMaxAttempts != 3 || cfg.LoginLockout != 9*time.Minute {
		t.Fatalf("login lockout config = %d/%s", cfg.LoginMaxAttempts, cfg.LoginLockout)
	}
	if cfg.LogArchiveAfterDays != 14 || cfg.LogDeleteAfterDays != 60 || cfg.LogCleanupBatchSize != 250 || cfg.LogMaxCount != 5000 {
		t.Fatalf("log cleanup config = %d/%d/%d/%d", cfg.LogArchiveAfterDays, cfg.LogDeleteAfterDays, cfg.LogCleanupBatchSize, cfg.LogMaxCount)
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

func TestLoadRejectsInvalidJWTAlgorithm(t *testing.T) {
	t.Setenv("JWT_ALGORITHM", "RS256")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid JWT algorithm error")
	}
}
