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
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAPIPrefix         = "/api/v1"
	defaultJWTSecretKey      = "your-secret-key-change-in-production"
	defaultAdminPassword     = "admin123"
	minProductionSecretBytes = 32
)

// Config contains process-level settings loaded from environment variables.
type Config struct {
	AppName     string
	AppVersion  string
	Debug       bool
	Environment string

	Host string
	Port int

	APIV1Prefix            string
	CORSOrigins            []string
	CORSAllowMethods       []string
	CORSAllowHeaders       []string
	RequestTimeout         time.Duration
	ReadHeaderTimeout      time.Duration
	ReadTimeout            time.Duration
	WriteTimeout           time.Duration
	IdleTimeout            time.Duration
	ShutdownTimeout        time.Duration
	MetricsEnabled         bool
	ManagementAllowedCIDRs []string

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

	LogArchiveAfterDays int
	LogDeleteAfterDays  int
	LogCleanupBatchSize int
	LogMaxCount         int

	StorageBackend string

	QiniuAccessKey     string
	QiniuSecretKey     string
	QiniuBucketName    string
	QiniuDomain        string
	QiniuPrivateBucket bool
	QiniuURLExpire     time.Duration
	QiniuUploadURL     string
	S3EndpointURL      string
	S3AccessKey        string
	S3SecretKey        string
	S3BucketName       string
	S3Region           string
	S3PublicURLBase    string
	S3PrivateBucket    bool
	S3URLExpire        time.Duration

	FernetSecretKey string

	XidianIDsBase                 string
	XidianEhallBase               string
	XidianYjsptBase               string
	XidianUserAgent               string
	XidianHTTPConnectTimeout      time.Duration
	XidianHTTPReadTimeout         time.Duration
	XidianChallengeTTL            time.Duration
	XidianSessionTTL              time.Duration
	XidianSyncRetryCount          int
	XidianSnapshotFallbackEnabled bool
	XidianCaptchaWidth            int
	XidianCaptchaHeight           int
	XidianPieceWidth              int
	XidianPieceHeight             int

	EinoEnabled       bool
	EinoBaseURL       string
	EinoAPIKey        string
	EinoModel         string
	EinoTimeout       time.Duration
	EinoTemperature   float64
	EinoMaxTokens     int
	EinoMaxIterations int
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
		ManagementAllowedCIDRs:    envList("MANAGEMENT_ALLOWED_CIDRS", []string{"127.0.0.1/32", "::1/128", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}),
		UploadsDir:                envString("UPLOADS_DIR", filepath.Join("..", "uploads")),
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
		JWTSecretKey:              envString("JWT_SECRET_KEY", defaultJWTSecretKey),
		JWTAlgorithm:              strings.ToUpper(envString("JWT_ALGORITHM", "HS256")),
		JWTAccessTokenExpire:      time.Duration(envInt("JWT_ACCESS_TOKEN_EXPIRE_MINUTES", 30)) * time.Minute,
		JWTRefreshTokenExpire:     time.Duration(envInt("JWT_REFRESH_TOKEN_EXPIRE_DAYS", 7)) * 24 * time.Hour,
		AdminUsername:             envString("ADMIN_USERNAME", "admin"),
		AdminEmail:                envString("ADMIN_EMAIL", "admin@example.com"),
		AdminPassword:             envString("ADMIN_PASSWORD", defaultAdminPassword),
		LoginMaxAttempts:          envInt("LOGIN_MAX_ATTEMPTS", 5),
		LoginLockout:              time.Duration(envInt("LOGIN_LOCKOUT_MINUTES", 15)) * time.Minute,
		LogArchiveAfterDays:       envInt("LOG_ARCHIVE_AFTER_DAYS", 30),
		LogDeleteAfterDays:        envInt("LOG_DELETE_AFTER_DAYS", 90),
		LogCleanupBatchSize:       envInt("LOG_CLEANUP_BATCH_SIZE", 500),
		LogMaxCount:               envInt("LOG_MAX_COUNT", 100000),
		StorageBackend:            strings.ToLower(envString("STORAGE_BACKEND", "local")),
		QiniuAccessKey:            envString("QINIU_ACCESS_KEY", ""),
		QiniuSecretKey:            envString("QINIU_SECRET_KEY", ""),
		QiniuBucketName:           envString("QINIU_BUCKET_NAME", ""),
		QiniuDomain:               envString("QINIU_DOMAIN", ""),
		QiniuPrivateBucket:        envBool("QINIU_PRIVATE_BUCKET", false),
		QiniuURLExpire:            time.Duration(envInt("QINIU_URL_EXPIRE_SECONDS", 3600)) * time.Second,
		QiniuUploadURL:            envString("QINIU_UPLOAD_URL", "https://upload.qiniup.com"),
		S3EndpointURL:             envString("S3_ENDPOINT_URL", ""),
		S3AccessKey:               envString("S3_ACCESS_KEY", ""),
		S3SecretKey:               envString("S3_SECRET_KEY", ""),
		S3BucketName:              envString("S3_BUCKET_NAME", ""),
		S3Region:                  envString("S3_REGION", "us-east-1"),
		S3PublicURLBase:           envString("S3_PUBLIC_URL_BASE", ""),
		S3PrivateBucket:           envBool("S3_PRIVATE_BUCKET", false),
		S3URLExpire:               time.Duration(envInt("S3_URL_EXPIRE_SECONDS", 3600)) * time.Second,
		FernetSecretKey:           envString("FERNET_SECRET_KEY", ""),
		XidianIDsBase:             envString("XIDIAN_IDS_BASE", "https://ids.xidian.edu.cn"),
		XidianEhallBase:           envString("XIDIAN_EHALL_BASE", "https://ehall.xidian.edu.cn"),
		XidianYjsptBase:           envString("XIDIAN_YJSPT_BASE", "https://yjspt.xidian.edu.cn"),
		XidianUserAgent: envString("XIDIAN_USER_AGENT",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"),
		XidianHTTPConnectTimeout:      envSeconds("XIDIAN_HTTP_CONNECT_TIMEOUT", 10*time.Second),
		XidianHTTPReadTimeout:         envSeconds("XIDIAN_HTTP_READ_TIMEOUT", 30*time.Second),
		XidianChallengeTTL:            time.Duration(envInt("XIDIAN_CHALLENGE_TTL", 600)) * time.Second,
		XidianSessionTTL:              time.Duration(envInt("XIDIAN_SESSION_TTL", 86400)) * time.Second,
		XidianSyncRetryCount:          envInt("XIDIAN_SYNC_RETRY_COUNT", 2),
		XidianSnapshotFallbackEnabled: envBool("XIDIAN_SNAPSHOT_FALLBACK_ENABLED", true),
		XidianCaptchaWidth:            envInt("XIDIAN_CAPTCHA_WIDTH", 280),
		XidianCaptchaHeight:           envInt("XIDIAN_CAPTCHA_HEIGHT", 155),
		XidianPieceWidth:              envInt("XIDIAN_PIECE_WIDTH", 44),
		XidianPieceHeight:             envInt("XIDIAN_PIECE_HEIGHT", 155),
		EinoEnabled:                   envBool("EINO_ENABLED", false),
		EinoBaseURL:                   envString("EINO_BASE_URL", ""),
		EinoAPIKey:                    envString("EINO_API_KEY", ""),
		EinoModel:                     envString("EINO_MODEL", ""),
		EinoTimeout:                   envSeconds("EINO_TIMEOUT_SECONDS", 45*time.Second),
		EinoTemperature:               envFloat("EINO_TEMPERATURE", 0.3),
		EinoMaxTokens:                 envInt("EINO_MAX_TOKENS", 1200),
		EinoMaxIterations:             envInt("EINO_MAX_ITERATIONS", 8),
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
	if err := validateProductionAuthConfig(cfg); err != nil {
		return Config{}, err
	}
	if err := validateCORSConfig(cfg); err != nil {
		return Config{}, err
	}
	if cfg.LoginMaxAttempts <= 0 {
		return Config{}, errors.New("LOGIN_MAX_ATTEMPTS must be greater than 0")
	}
	if cfg.LoginLockout <= 0 {
		return Config{}, errors.New("LOGIN_LOCKOUT_MINUTES must be greater than 0")
	}
	if cfg.LogArchiveAfterDays <= 0 {
		return Config{}, errors.New("LOG_ARCHIVE_AFTER_DAYS must be greater than 0")
	}
	if cfg.LogDeleteAfterDays <= 0 {
		return Config{}, errors.New("LOG_DELETE_AFTER_DAYS must be greater than 0")
	}
	if cfg.LogCleanupBatchSize <= 0 {
		return Config{}, errors.New("LOG_CLEANUP_BATCH_SIZE must be greater than 0")
	}
	if cfg.LogMaxCount <= 0 {
		return Config{}, errors.New("LOG_MAX_COUNT must be greater than 0")
	}
	if err := validateStorageConfig(cfg); err != nil {
		return Config{}, err
	}
	if err := validateXidianConfig(cfg); err != nil {
		return Config{}, err
	}
	if err := validateEinoConfig(cfg); err != nil {
		return Config{}, err
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

// RequiresSharedRefreshSessionStore reports whether refresh sessions must use
// shared Redis state instead of local fallback.
func (c Config) RequiresSharedRefreshSessionStore() bool {
	return isStrictEnvironment(c.Environment)
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

func envFloat(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
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

func validateProductionAuthConfig(cfg Config) error {
	if !isStrictEnvironment(cfg.Environment) {
		return nil
	}
	secret := strings.TrimSpace(cfg.JWTSecretKey)
	if len([]byte(secret)) < minProductionSecretBytes || placeholderSecret(secret) {
		return errors.New("JWT_SECRET_KEY must be a non-placeholder secret with at least 32 bytes outside development")
	}
	adminPassword := strings.TrimSpace(cfg.AdminPassword)
	if adminPassword == defaultAdminPassword || placeholderSecret(adminPassword) {
		return errors.New("ADMIN_PASSWORD must not use the default or placeholder value outside development")
	}
	if !strongConfigPassword(adminPassword) {
		return errors.New("ADMIN_PASSWORD must be 8-72 bytes and include uppercase, lowercase, digit, and special characters outside development")
	}
	return nil
}

func validateCORSConfig(cfg Config) error {
	if !isStrictEnvironment(cfg.Environment) {
		return nil
	}
	for _, origin := range cfg.CORSOrigins {
		if strings.TrimSpace(origin) == "*" {
			return errors.New("CORS_ORIGINS must not contain * outside development")
		}
	}
	return nil
}

func isStrictEnvironment(environment string) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "", "development", "dev", "local", "test":
		return false
	default:
		return true
	}
}

func placeholderSecret(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "change-me", "change-me-in-each-environment", defaultJWTSecretKey, "secret", "test-secret":
		return true
	default:
		return false
	}
}

func strongConfigPassword(password string) bool {
	if len(password) < 8 || len([]byte(password)) > 72 {
		return false
	}
	hasUpper, hasLower, hasDigit, hasSpecial := false, false, false, false
	for _, r := range password {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}
	return hasUpper && hasLower && hasDigit && hasSpecial
}

func validateStorageConfig(cfg Config) error {
	switch cfg.StorageBackend {
	case "local":
		if strings.TrimSpace(cfg.UploadsDir) == "" {
			return errors.New("UPLOADS_DIR must not be empty when STORAGE_BACKEND=local")
		}
	case "qiniu":
		if err := requireConfigValues("Qiniu", map[string]string{
			"QINIU_ACCESS_KEY":  cfg.QiniuAccessKey,
			"QINIU_SECRET_KEY":  cfg.QiniuSecretKey,
			"QINIU_BUCKET_NAME": cfg.QiniuBucketName,
			"QINIU_DOMAIN":      cfg.QiniuDomain,
			"QINIU_UPLOAD_URL":  cfg.QiniuUploadURL,
		}); err != nil {
			return err
		}
		if cfg.QiniuURLExpire <= 0 {
			return errors.New("QINIU_URL_EXPIRE_SECONDS must be greater than 0")
		}
	case "s3":
		if err := requireConfigValues("S3", map[string]string{
			"S3_ENDPOINT_URL": cfg.S3EndpointURL,
			"S3_ACCESS_KEY":   cfg.S3AccessKey,
			"S3_SECRET_KEY":   cfg.S3SecretKey,
			"S3_BUCKET_NAME":  cfg.S3BucketName,
			"S3_REGION":       cfg.S3Region,
		}); err != nil {
			return err
		}
		if cfg.S3URLExpire <= 0 {
			return errors.New("S3_URL_EXPIRE_SECONDS must be greater than 0")
		}
	default:
		return fmt.Errorf("STORAGE_BACKEND must be one of local, qiniu, s3, got %s", cfg.StorageBackend)
	}
	return nil
}

func requireConfigValues(name string, values map[string]string) error {
	missing := make([]string, 0)
	for key, value := range values {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("%s storage config missing: %s", name, strings.Join(missing, ", "))
	}
	return nil
}

func validateXidianConfig(cfg Config) error {
	if cfg.XidianHTTPConnectTimeout <= 0 {
		return errors.New("XIDIAN_HTTP_CONNECT_TIMEOUT must be greater than 0")
	}
	if cfg.XidianHTTPReadTimeout <= 0 {
		return errors.New("XIDIAN_HTTP_READ_TIMEOUT must be greater than 0")
	}
	if cfg.XidianChallengeTTL <= 0 {
		return errors.New("XIDIAN_CHALLENGE_TTL must be greater than 0")
	}
	if cfg.XidianSessionTTL <= 0 {
		return errors.New("XIDIAN_SESSION_TTL must be greater than 0")
	}
	if cfg.XidianSyncRetryCount < 0 {
		return errors.New("XIDIAN_SYNC_RETRY_COUNT must be zero or greater")
	}
	if cfg.XidianCaptchaWidth <= 0 || cfg.XidianCaptchaHeight <= 0 || cfg.XidianPieceWidth <= 0 || cfg.XidianPieceHeight <= 0 {
		return errors.New("Xidian captcha dimensions must be greater than 0")
	}
	return requireConfigValues("Xidian", map[string]string{
		"XIDIAN_IDS_BASE":   cfg.XidianIDsBase,
		"XIDIAN_EHALL_BASE": cfg.XidianEhallBase,
		"XIDIAN_YJSPT_BASE": cfg.XidianYjsptBase,
		"XIDIAN_USER_AGENT": cfg.XidianUserAgent,
	})
}

func validateEinoConfig(cfg Config) error {
	if !cfg.EinoEnabled {
		return nil
	}
	if strings.TrimSpace(cfg.EinoAPIKey) == "" {
		return errors.New("EINO_API_KEY must not be empty when EINO_ENABLED=true")
	}
	if strings.TrimSpace(cfg.EinoModel) == "" {
		return errors.New("EINO_MODEL must not be empty when EINO_ENABLED=true")
	}
	if cfg.EinoTimeout <= 0 {
		return errors.New("EINO_TIMEOUT_SECONDS must be greater than 0 when EINO_ENABLED=true")
	}
	if cfg.EinoTemperature < 0 || cfg.EinoTemperature > 2 {
		return errors.New("EINO_TEMPERATURE must be between 0 and 2 when EINO_ENABLED=true")
	}
	if cfg.EinoMaxTokens < 0 {
		return errors.New("EINO_MAX_TOKENS must be zero or greater when EINO_ENABLED=true")
	}
	if cfg.EinoMaxIterations <= 0 {
		return errors.New("EINO_MAX_ITERATIONS must be greater than 0 when EINO_ENABLED=true")
	}
	return nil
}
