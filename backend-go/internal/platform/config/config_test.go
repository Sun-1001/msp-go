package config

import (
	"reflect"
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
	t.Setenv("EINO_ENABLED", "true")
	t.Setenv("EINO_BASE_URL", "https://api.example.com/v1")
	t.Setenv("EINO_API_KEY", "test-key")
	t.Setenv("EINO_MODEL", "deepseek-chat")
	t.Setenv("EINO_TIMEOUT_SECONDS", "12.5")
	t.Setenv("EINO_TEMPERATURE", "0.2")
	t.Setenv("EINO_MAX_TOKENS", "900")
	t.Setenv("EINO_MAX_ITERATIONS", "5")

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
	if cfg.StorageBackend != "local" {
		t.Fatalf("StorageBackend = %q", cfg.StorageBackend)
	}
	if !cfg.EinoEnabled || cfg.EinoBaseURL != "https://api.example.com/v1" || cfg.EinoAPIKey != "test-key" || cfg.EinoModel != "deepseek-chat" {
		t.Fatalf("Eino config = %#v", cfg)
	}
	if cfg.EinoTimeout != 12500*time.Millisecond || cfg.EinoTemperature != 0.2 || cfg.EinoMaxTokens != 900 || cfg.EinoMaxIterations != 5 {
		t.Fatalf("Eino tuning config = %s/%f/%d/%d", cfg.EinoTimeout, cfg.EinoTemperature, cfg.EinoMaxTokens, cfg.EinoMaxIterations)
	}
}

func TestLoadParsesListEnvironmentFormats(t *testing.T) {
	t.Setenv("CORS_ORIGINS", `["https://app.example.com", " https://admin.example.com ", ""]`)
	t.Setenv("CORS_ALLOW_METHODS", `GET, POST, 'OPTIONS'`)
	t.Setenv("MANAGEMENT_ALLOWED_CIDRS", `[]`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	assertStringSlice(t, cfg.CORSOrigins, []string{"https://app.example.com", "https://admin.example.com"})
	assertStringSlice(t, cfg.CORSAllowMethods, []string{"GET", "POST", "OPTIONS"})
	assertStringSlice(t, cfg.ManagementAllowedCIDRs, []string{"127.0.0.1/32", "::1/128", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"})
}

func TestParseEnvList(t *testing.T) {
	fallback := []string{"fallback"}
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{
			name:  "strict JSON list",
			value: `["GET", " POST ", ""]`,
			want:  []string{"GET", "POST"},
		},
		{
			name:  "comma list",
			value: `GET, POST, 'OPTIONS', [PATCH]`,
			want:  []string{"GET", "POST", "OPTIONS", "PATCH"},
		},
		{
			name:  "empty JSON list uses fallback",
			value: `["", " "]`,
			want:  fallback,
		},
		{
			name:  "malformed JSON list keeps legacy comma fallback",
			value: `["GET", "POST"`,
			want:  []string{"GET", "POST"},
		},
		{
			name:  "trailing JSON data uses fallback",
			value: `["GET"] {"extra": true}`,
			want:  fallback,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEnvList(tt.value, fallback)
			assertStringSlice(t, got, tt.want)
		})
	}
}

func TestEnvDuration(t *testing.T) {
	fallback := 42 * time.Second

	t.Setenv("TEST_DURATION_SECONDS_NUMBER", "2.5")
	if got := envSeconds("TEST_DURATION_SECONDS_NUMBER", fallback); got != 2500*time.Millisecond {
		t.Fatalf("envSeconds(number) = %s, want 2.5s", got)
	}

	t.Setenv("TEST_DURATION_SECONDS_TEXT", "1500ms")
	if got := envSeconds("TEST_DURATION_SECONDS_TEXT", fallback); got != 1500*time.Millisecond {
		t.Fatalf("envSeconds(duration) = %s, want 1.5s", got)
	}

	t.Setenv("TEST_DURATION_MILLISECONDS_NUMBER", "1500")
	if got := envMilliseconds("TEST_DURATION_MILLISECONDS_NUMBER", fallback); got != 1500*time.Millisecond {
		t.Fatalf("envMilliseconds(number) = %s, want 1.5s", got)
	}

	t.Setenv("TEST_DURATION_MILLISECONDS_TEXT", "2s")
	if got := envMilliseconds("TEST_DURATION_MILLISECONDS_TEXT", fallback); got != 2*time.Second {
		t.Fatalf("envMilliseconds(duration) = %s, want 2s", got)
	}

	t.Setenv("TEST_DURATION_BAD", "bad")
	if got := envSeconds("TEST_DURATION_BAD", fallback); got != fallback {
		t.Fatalf("envSeconds(bad) = %s, want fallback %s", got, fallback)
	}

	t.Setenv("TEST_DURATION_EMPTY", "")
	if got := envMilliseconds("TEST_DURATION_EMPTY", fallback); got != fallback {
		t.Fatalf("envMilliseconds(empty) = %s, want fallback %s", got, fallback)
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

func TestLoadRejectsProductionPlaceholderSecrets(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("JWT_SECRET_KEY", "change-me-in-each-environment")
	t.Setenv("ADMIN_PASSWORD", "Admin123!")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want production JWT secret error")
	}
}

func TestLoadRejectsProductionWeakAdminPassword(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("JWT_SECRET_KEY", strings.Repeat("s", 32))
	t.Setenv("ADMIN_PASSWORD", "weakpassword")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want production admin password error")
	}
}

func TestLoadAcceptsProductionStrongAuthConfig(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("JWT_SECRET_KEY", strings.Repeat("s", 32))
	t.Setenv("ADMIN_PASSWORD", "Admin123!")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Environment != "production" {
		t.Fatalf("Environment = %q", cfg.Environment)
	}
	if !cfg.RequiresSharedRefreshSessionStore() {
		t.Fatal("RequiresSharedRefreshSessionStore() = false, want true")
	}
}

func TestLoadRejectsProductionWildcardCORS(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("JWT_SECRET_KEY", strings.Repeat("s", 32))
	t.Setenv("ADMIN_PASSWORD", "Admin123!")
	t.Setenv("CORS_ORIGINS", "*")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want production CORS wildcard error")
	}
}

func TestLoadDevelopmentAllowsLocalRefreshSessionFallback(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.RequiresSharedRefreshSessionStore() {
		t.Fatal("RequiresSharedRefreshSessionStore() = true, want false")
	}
}

func TestLoadReadsS3StorageConfig(t *testing.T) {
	t.Setenv("STORAGE_BACKEND", "s3")
	t.Setenv("S3_ENDPOINT_URL", "https://s3.example.com")
	t.Setenv("S3_ACCESS_KEY", "access")
	t.Setenv("S3_SECRET_KEY", "secret")
	t.Setenv("S3_BUCKET_NAME", "bucket")
	t.Setenv("S3_REGION", "")
	t.Setenv("S3_PUBLIC_URL_BASE", "https://cdn.example.com")
	t.Setenv("S3_PRIVATE_BUCKET", "true")
	t.Setenv("S3_URL_EXPIRE_SECONDS", "900")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.StorageBackend != "s3" || cfg.S3EndpointURL != "https://s3.example.com" || cfg.S3Region != "us-east-1" {
		t.Fatalf("S3 config = %#v", cfg)
	}
	if !cfg.S3PrivateBucket || cfg.S3URLExpire != 15*time.Minute {
		t.Fatalf("S3 private config = %t/%s", cfg.S3PrivateBucket, cfg.S3URLExpire)
	}
}

func TestLoadRejectsInvalidStorageBackend(t *testing.T) {
	t.Setenv("STORAGE_BACKEND", "ftp")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid storage backend error")
	}
}

func TestLoadRejectsEnabledEinoWithoutRequiredModelConfig(t *testing.T) {
	t.Setenv("EINO_ENABLED", "true")
	t.Setenv("EINO_API_KEY", "test-key")
	t.Setenv("EINO_MODEL", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing Eino model error")
	}
}

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("slice = %#v, want %#v", got, want)
	}
}
