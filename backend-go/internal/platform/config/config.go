package config

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAPIPrefix = "/api/v1"
)

// Config contains process-level settings loaded from environment variables.
type Config struct {
	AppName     string
	AppVersion  string
	Debug       bool
	Environment string

	Host string
	Port int

	APIV1Prefix       string
	CORSOrigins       []string
	CORSAllowMethods  []string
	CORSAllowHeaders  []string
	RequestTimeout    time.Duration
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MetricsEnabled    bool

	UploadsDir string

	PostgresHost        string
	PostgresPort        int
	PostgresUser        string
	PostgresPassword    string
	PostgresDB          string
	DBPoolSize          int
	DBPoolMinConns      int
	DBPoolRecycle       time.Duration
	DBConnectTimeout    time.Duration
	DBStatementTimeout  time.Duration
	DBIdleTxTimeout     time.Duration
	DBHealthCheckPeriod time.Duration

	RedisHost                 string
	RedisPort                 int
	RedisPassword             string
	RedisDB                   int
	RedisMaxConnections       int
	RedisSocketTimeout        time.Duration
	RedisConnectTimeout       time.Duration
	RedisFallbackCacheMaxSize int

	JWTSecretKey          string
	JWTAlgorithm          string
	JWTAccessTokenExpire  time.Duration
	JWTRefreshTokenExpire time.Duration

	AdminUsername string
	AdminEmail    string
	AdminPassword string

	LoginMaxAttempts int
	LoginLockout     time.Duration
}

// Load reads the single repository .env file without overwriting process env.
func Load() (Config, error) {
	loadEnvFiles([]string{
		".env",
		filepath.Join("..", ".env"),
	})

	cfg := Config{
		AppName:                   envString("APP_NAME", "高等数学智能学习平台"),
		AppVersion:                envString("APP_VERSION", "0.1.0"),
		Debug:                     envBool("DEBUG", false),
		Environment:               envString("ENVIRONMENT", "development"),
		Host:                      envString("GO_API_HOST", envString("HOST", "0.0.0.0")),
		Port:                      envInt("GO_API_PORT", envInt("PORT", 8000)),
		APIV1Prefix:               cleanPrefix(envString("API_V1_PREFIX", defaultAPIPrefix)),
		CORSOrigins:               envList("CORS_ORIGINS", []string{"http://localhost:3000", "http://localhost:5173"}),
		CORSAllowMethods:          envList("CORS_ALLOW_METHODS", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}),
		CORSAllowHeaders:          envList("CORS_ALLOW_HEADERS", []string{"Authorization", "Content-Type", "Accept", "Origin", "X-Requested-With", "X-CSRF-Token"}),
		RequestTimeout:            envSeconds("REQUEST_TIMEOUT_DEFAULT", 30*time.Second),
		ReadHeaderTimeout:         envSeconds("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
		ReadTimeout:               envSeconds("HTTP_READ_TIMEOUT", 35*time.Second),
		WriteTimeout:              envSeconds("HTTP_WRITE_TIMEOUT", 310*time.Second),
		IdleTimeout:               envSeconds("HTTP_IDLE_TIMEOUT", 60*time.Second),
		ShutdownTimeout:           envSeconds("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
		MetricsEnabled:            envBool("METRICS_ENABLED", true),
		UploadsDir:                envString("UPLOADS_DIR", filepath.Join("..", "backend", "uploads")),
		PostgresHost:              envString("POSTGRES_HOST", "localhost"),
		PostgresPort:              envInt("POSTGRES_PORT", 5432),
		PostgresUser:              envString("POSTGRES_USER", "postgres"),
		PostgresPassword:          envString("POSTGRES_PASSWORD", "postgres"),
		PostgresDB:                envString("POSTGRES_DB", "math_platform"),
		DBPoolSize:                envInt("DB_POOL_SIZE", 12),
		DBPoolMinConns:            envInt("DB_POOL_MIN_CONNS", 0),
		DBPoolRecycle:             envSeconds("DB_POOL_RECYCLE_SECONDS", 1800*time.Second),
		DBConnectTimeout:          envSeconds("DB_CONNECT_TIMEOUT_SECONDS", 5*time.Second),
		DBStatementTimeout:        envMilliseconds("DB_STATEMENT_TIMEOUT_MS", 30*time.Second),
		DBIdleTxTimeout:           envMilliseconds("DB_IDLE_TX_TIMEOUT_MS", 60*time.Second),
		DBHealthCheckPeriod:       envSeconds("DB_HEALTH_CHECK_PERIOD_SECONDS", 30*time.Second),
		RedisHost:                 envString("REDIS_HOST", "localhost"),
		RedisPort:                 envInt("REDIS_PORT", 6379),
		RedisPassword:             envString("REDIS_PASSWORD", ""),
		RedisDB:                   envInt("REDIS_DB", 0),
		RedisMaxConnections:       envInt("REDIS_MAX_CONNECTIONS", 20),
		RedisSocketTimeout:        envSeconds("REDIS_SOCKET_TIMEOUT_SECONDS", 3*time.Second),
		RedisConnectTimeout:       envSeconds("REDIS_SOCKET_CONNECT_TIMEOUT_SECONDS", 3*time.Second),
		RedisFallbackCacheMaxSize: envInt("REDIS_FALLBACK_CACHE_MAX_SIZE", 500),
		JWTSecretKey:              envString("JWT_SECRET_KEY", "your-secret-key-change-in-production"),
		JWTAlgorithm:              strings.ToUpper(envString("JWT_ALGORITHM", "HS256")),
		JWTAccessTokenExpire:      time.Duration(envInt("JWT_ACCESS_TOKEN_EXPIRE_MINUTES", 30)) * time.Minute,
		JWTRefreshTokenExpire:     time.Duration(envInt("JWT_REFRESH_TOKEN_EXPIRE_DAYS", 7)) * 24 * time.Hour,
		AdminUsername:             envString("ADMIN_USERNAME", "admin"),
		AdminEmail:                envString("ADMIN_EMAIL", "admin@example.com"),
		AdminPassword:             envString("ADMIN_PASSWORD", "admin123"),
		LoginMaxAttempts:          envInt("LOGIN_MAX_ATTEMPTS", 5),
		LoginLockout:              time.Duration(envInt("LOGIN_LOCKOUT_MINUTES", 15)) * time.Minute,
	}

	if cfg.Port <= 0 || cfg.Port > 65535 {
		return Config{}, fmt.Errorf("GO_API_PORT must be between 1 and 65535, got %d", cfg.Port)
	}
	if cfg.DBPoolSize <= 0 {
		return Config{}, errors.New("DB_POOL_SIZE must be greater than 0")
	}
	if cfg.DBPoolMinConns < 0 {
		return Config{}, errors.New("DB_POOL_MIN_CONNS must be zero or greater")
	}
	if cfg.DBPoolMinConns > cfg.DBPoolSize {
		return Config{}, errors.New("DB_POOL_MIN_CONNS must not exceed DB_POOL_SIZE")
	}
	if cfg.RedisMaxConnections <= 0 {
		return Config{}, errors.New("REDIS_MAX_CONNECTIONS must be greater than 0")
	}
	if cfg.RedisFallbackCacheMaxSize <= 0 {
		return Config{}, errors.New("REDIS_FALLBACK_CACHE_MAX_SIZE must be greater than 0")
	}
	if !allowedJWTAlgorithms()[cfg.JWTAlgorithm] {
		return Config{}, fmt.Errorf("JWT_ALGORITHM must be one of HS256, HS384, HS512, got %s", cfg.JWTAlgorithm)
	}
	if strings.TrimSpace(cfg.JWTSecretKey) == "" {
		return Config{}, errors.New("JWT_SECRET_KEY must not be empty")
	}
	if cfg.JWTAccessTokenExpire <= 0 {
		return Config{}, errors.New("JWT_ACCESS_TOKEN_EXPIRE_MINUTES must be greater than 0")
	}
	if cfg.JWTRefreshTokenExpire <= 0 {
		return Config{}, errors.New("JWT_REFRESH_TOKEN_EXPIRE_DAYS must be greater than 0")
	}
	if strings.TrimSpace(cfg.AdminUsername) == "" {
		return Config{}, errors.New("ADMIN_USERNAME must not be empty")
	}
	if strings.TrimSpace(cfg.AdminEmail) == "" {
		return Config{}, errors.New("ADMIN_EMAIL must not be empty")
	}
	if strings.TrimSpace(cfg.AdminPassword) == "" {
		return Config{}, errors.New("ADMIN_PASSWORD must not be empty")
	}
	if cfg.LoginMaxAttempts <= 0 {
		return Config{}, errors.New("LOGIN_MAX_ATTEMPTS must be greater than 0")
	}
	if cfg.LoginLockout <= 0 {
		return Config{}, errors.New("LOGIN_LOCKOUT_MINUTES must be greater than 0")
	}
	return cfg, nil
}

// HTTPAddr returns the TCP address used by the HTTP server.
func (c Config) HTTPAddr() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

// DatabaseURL returns the PostgreSQL DSN for pgx.
func (c Config) DatabaseURL() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.PostgresUser, c.PostgresPassword),
		Host:   net.JoinHostPort(c.PostgresHost, strconv.Itoa(c.PostgresPort)),
		Path:   c.PostgresDB,
	}
	return u.String()
}

// RedisAddr returns the host:port address for Redis.
func (c Config) RedisAddr() string {
	return net.JoinHostPort(c.RedisHost, strconv.Itoa(c.RedisPort))
}

func loadEnvFiles(paths []string) {
	for _, path := range paths {
		loadEnvFile(path)
	}
}

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(strings.TrimPrefix(key, "export "))
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		os.Setenv(key, trimEnvValue(value))
	}
}

func trimEnvValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return value
}

func envString(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envSeconds(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	if strings.ContainsAny(value, "hms") {
		parsed, err := time.ParseDuration(value)
		if err == nil {
			return parsed
		}
	}
	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return time.Duration(seconds * float64(time.Second))
}

func envMilliseconds(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	if strings.ContainsAny(value, "hms") {
		parsed, err := time.ParseDuration(value)
		if err == nil {
			return parsed
		}
	}
	milliseconds, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return time.Duration(milliseconds * float64(time.Millisecond))
}

func envList(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	var jsonValues []string
	if strings.HasPrefix(value, "[") && json.Unmarshal([]byte(value), &jsonValues) == nil {
		if len(jsonValues) > 0 {
			return jsonValues
		}
		return fallback
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(strings.Trim(part, `"'[]`))
		if item != "" {
			result = append(result, item)
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}

func cleanPrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return defaultAPIPrefix
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return strings.TrimRight(prefix, "/")
}

func allowedJWTAlgorithms() map[string]bool {
	return map[string]bool{
		"HS256": true,
		"HS384": true,
		"HS512": true,
	}
}
