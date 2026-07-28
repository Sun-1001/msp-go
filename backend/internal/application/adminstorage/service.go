package adminstorage

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	settingBackend = "storage_backend"

	settingQiniuAccessKey     = "storage_qiniu_access_key"
	settingQiniuSecretKey     = "storage_qiniu_secret_key"
	settingQiniuBucketName    = "storage_qiniu_bucket_name"
	settingQiniuDomain        = "storage_qiniu_domain"
	settingQiniuPrivateBucket = "storage_qiniu_private_bucket"
	settingQiniuURLExpire     = "storage_qiniu_url_expire_seconds"
	settingQiniuUploadURL     = "storage_qiniu_upload_url"

	settingS3EndpointURL   = "storage_s3_endpoint_url"
	settingS3AccessKey     = "storage_s3_access_key"
	settingS3SecretKey     = "storage_s3_secret_key"
	settingS3BucketName    = "storage_s3_bucket_name"
	settingS3Region        = "storage_s3_region"
	settingS3PublicURLBase = "storage_s3_public_url_base"
	settingS3PrivateBucket = "storage_s3_private_bucket"
	settingS3URLExpire     = "storage_s3_url_expire_seconds"

	defaultURLExpireSeconds = 3600
	defaultQiniuUploadURL   = "https://upload.qiniup.com"
	defaultS3Region         = "us-east-1"
	maxCredentialBytes      = 4096
	maxSettingBytes         = 2048
	maxURLExpireSeconds     = 7 * 24 * 60 * 60
	connectionTestTimeout   = 20 * time.Second
)

var storageSettingKeys = []string{
	settingBackend,
	settingQiniuAccessKey,
	settingQiniuSecretKey,
	settingQiniuBucketName,
	settingQiniuDomain,
	settingQiniuPrivateBucket,
	settingQiniuURLExpire,
	settingQiniuUploadURL,
	settingS3EndpointURL,
	settingS3AccessKey,
	settingS3SecretKey,
	settingS3BucketName,
	settingS3Region,
	settingS3PublicURLBase,
	settingS3PrivateBucket,
	settingS3URLExpire,
}

// Service manages encrypted storage settings and runtime activation.
type Service struct {
	repo            Repository
	cipher          Cipher
	runtime         Runtime
	localConfigured bool
	updateMu        sync.Mutex
	now             func() time.Time
}

// NewService creates an administrator storage settings service.
func NewService(repo Repository, cipher Cipher, runtime Runtime, localConfigured bool) (*Service, error) {
	if repo == nil {
		return nil, errors.New("storage settings repository is nil")
	}
	if cipher == nil {
		return nil, errors.New("storage settings cipher is nil")
	}
	if runtime == nil {
		return nil, errors.New("storage runtime is nil")
	}
	return &Service{
		repo:            repo,
		cipher:          cipher,
		runtime:         runtime,
		localConfigured: localConfigured,
		now:             func() time.Time { return time.Now().UTC() },
	}, nil
}

// Settings returns the effective credential-safe storage configuration.
func (s *Service) Settings(ctx context.Context) (SettingsResponse, error) {
	config, source, err := s.loadConfig(ctx)
	if err != nil {
		return SettingsResponse{}, err
	}
	return settingsResponse(config, source), nil
}

// UpdateSettings validates, probes, persists, and atomically activates a configuration.
func (s *Service) UpdateSettings(ctx context.Context, input UpdateInput) (SettingsResponse, error) {
	s.updateMu.Lock()
	defer s.updateMu.Unlock()

	current, _, err := s.loadConfig(ctx)
	if err != nil {
		return SettingsResponse{}, err
	}
	candidate, err := mergeInput(current, input)
	if err != nil {
		return SettingsResponse{}, err
	}
	prepared, err := s.runtime.Prepare(candidate)
	if err != nil {
		return SettingsResponse{}, invalidConfig(err)
	}
	if err := testPrepared(ctx, prepared); err != nil {
		return SettingsResponse{}, connectionFailed(err)
	}
	updates, err := s.settingUpdates(candidate)
	if err != nil {
		return SettingsResponse{}, err
	}
	if err := s.repo.SaveStorageSettings(ctx, updates); err != nil {
		return SettingsResponse{}, fmt.Errorf("save storage settings: %w", err)
	}
	prepared.Activate()
	return settingsResponse(candidate, SourceDB), nil
}

// TestConnection probes a draft configuration without saving or activating it.
func (s *Service) TestConnection(ctx context.Context, input UpdateInput) (TestResponse, error) {
	current, _, err := s.loadConfig(ctx)
	if err != nil {
		return TestResponse{}, err
	}
	candidate, err := mergeInput(current, input)
	if err != nil {
		return TestResponse{}, err
	}
	prepared, err := s.runtime.Prepare(candidate)
	if err != nil {
		return TestResponse{}, invalidConfig(err)
	}
	startedAt := s.now()
	if err := testPrepared(ctx, prepared); err != nil {
		return TestResponse{}, connectionFailed(err)
	}
	return TestResponse{
		Success:   true,
		Message:   backendLabel(candidate.Backend) + "存储连接成功",
		Backend:   candidate.Backend,
		LatencyMS: max(0, s.now().Sub(startedAt).Milliseconds()),
	}, nil
}

// ActivateStored activates administrator-managed database settings when present.
func (s *Service) ActivateStored(ctx context.Context) error {
	s.updateMu.Lock()
	defer s.updateMu.Unlock()

	config, source, err := s.loadConfig(ctx)
	if err != nil {
		return err
	}
	if source == SourceUnconfigured {
		return nil
	}
	prepared, err := s.runtime.Prepare(config)
	if err != nil {
		return fmt.Errorf("prepare effective storage settings: %w", err)
	}
	prepared.Activate()
	return nil
}

func (s *Service) loadConfig(ctx context.Context) (Config, string, error) {
	values, err := s.repo.GetSettings(ctx, storageSettingKeys)
	if err != nil {
		return Config{}, "", fmt.Errorf("get storage settings: %w", err)
	}
	if strings.TrimSpace(values[settingBackend]) == "" {
		return normalizeDefaults(Config{
			Backend:         BackendLocal,
			LocalConfigured: s.localConfigured,
		}), SourceUnconfigured, nil
	}

	qiniuAccessKey, err := s.decryptSetting(values, settingQiniuAccessKey)
	if err != nil {
		return Config{}, "", err
	}
	qiniuSecretKey, err := s.decryptSetting(values, settingQiniuSecretKey)
	if err != nil {
		return Config{}, "", err
	}
	s3AccessKey, err := s.decryptSetting(values, settingS3AccessKey)
	if err != nil {
		return Config{}, "", err
	}
	s3SecretKey, err := s.decryptSetting(values, settingS3SecretKey)
	if err != nil {
		return Config{}, "", err
	}

	qiniuPrivate, err := parseStoredBool(values[settingQiniuPrivateBucket], false, settingQiniuPrivateBucket)
	if err != nil {
		return Config{}, "", err
	}
	qiniuExpire, err := parseStoredInt(values[settingQiniuURLExpire], defaultURLExpireSeconds, settingQiniuURLExpire)
	if err != nil {
		return Config{}, "", err
	}
	s3Private, err := parseStoredBool(values[settingS3PrivateBucket], false, settingS3PrivateBucket)
	if err != nil {
		return Config{}, "", err
	}
	s3Expire, err := parseStoredInt(values[settingS3URLExpire], defaultURLExpireSeconds, settingS3URLExpire)
	if err != nil {
		return Config{}, "", err
	}

	config := Config{
		Backend:         strings.ToLower(strings.TrimSpace(values[settingBackend])),
		LocalConfigured: s.localConfigured,
		Qiniu: QiniuConfig{
			AccessKey:        qiniuAccessKey,
			SecretKey:        qiniuSecretKey,
			BucketName:       strings.TrimSpace(values[settingQiniuBucketName]),
			Domain:           strings.TrimSpace(values[settingQiniuDomain]),
			PrivateBucket:    qiniuPrivate,
			URLExpireSeconds: qiniuExpire,
			UploadURL:        strings.TrimSpace(values[settingQiniuUploadURL]),
		},
		S3: S3Config{
			EndpointURL:      strings.TrimSpace(values[settingS3EndpointURL]),
			AccessKey:        s3AccessKey,
			SecretKey:        s3SecretKey,
			BucketName:       strings.TrimSpace(values[settingS3BucketName]),
			Region:           strings.TrimSpace(values[settingS3Region]),
			PublicURLBase:    strings.TrimSpace(values[settingS3PublicURLBase]),
			PrivateBucket:    s3Private,
			URLExpireSeconds: s3Expire,
		},
	}
	return normalizeDefaults(config), SourceDB, nil
}

func (s *Service) decryptSetting(values map[string]string, key string) (string, error) {
	encrypted := strings.TrimSpace(values[key])
	if encrypted == "" {
		return "", nil
	}
	decrypted, err := s.cipher.Decrypt(encrypted)
	if err != nil {
		return "", fmt.Errorf("decrypt %s: %w", key, err)
	}
	return decrypted, nil
}

func (s *Service) settingUpdates(config Config) ([]SettingUpdate, error) {
	qiniuAccessKey, err := encryptCredential(s.cipher, config.Qiniu.AccessKey, settingQiniuAccessKey)
	if err != nil {
		return nil, err
	}
	qiniuSecretKey, err := encryptCredential(s.cipher, config.Qiniu.SecretKey, settingQiniuSecretKey)
	if err != nil {
		return nil, err
	}
	s3AccessKey, err := encryptCredential(s.cipher, config.S3.AccessKey, settingS3AccessKey)
	if err != nil {
		return nil, err
	}
	s3SecretKey, err := encryptCredential(s.cipher, config.S3.SecretKey, settingS3SecretKey)
	if err != nil {
		return nil, err
	}

	now := s.now()
	return []SettingUpdate{
		{Key: settingBackend, Value: config.Backend, Description: "当前对象存储后端", UpdatedAt: now},
		{Key: settingQiniuAccessKey, Value: qiniuAccessKey, Description: "七牛 Access Key（Fernet 加密）", UpdatedAt: now},
		{Key: settingQiniuSecretKey, Value: qiniuSecretKey, Description: "七牛 Secret Key（Fernet 加密）", UpdatedAt: now},
		{Key: settingQiniuBucketName, Value: config.Qiniu.BucketName, Description: "七牛存储空间名称", UpdatedAt: now},
		{Key: settingQiniuDomain, Value: config.Qiniu.Domain, Description: "七牛访问域名", UpdatedAt: now},
		{Key: settingQiniuPrivateBucket, Value: strconv.FormatBool(config.Qiniu.PrivateBucket), Description: "七牛是否为私有空间", UpdatedAt: now},
		{Key: settingQiniuURLExpire, Value: strconv.Itoa(config.Qiniu.URLExpireSeconds), Description: "七牛私有 URL 有效期（秒）", UpdatedAt: now},
		{Key: settingQiniuUploadURL, Value: config.Qiniu.UploadURL, Description: "七牛上传地址", UpdatedAt: now},
		{Key: settingS3EndpointURL, Value: config.S3.EndpointURL, Description: "S3 兼容服务地址", UpdatedAt: now},
		{Key: settingS3AccessKey, Value: s3AccessKey, Description: "S3 Access Key（Fernet 加密）", UpdatedAt: now},
		{Key: settingS3SecretKey, Value: s3SecretKey, Description: "S3 Secret Key（Fernet 加密）", UpdatedAt: now},
		{Key: settingS3BucketName, Value: config.S3.BucketName, Description: "S3 存储桶名称", UpdatedAt: now},
		{Key: settingS3Region, Value: config.S3.Region, Description: "S3 区域", UpdatedAt: now},
		{Key: settingS3PublicURLBase, Value: config.S3.PublicURLBase, Description: "S3 公共访问地址", UpdatedAt: now},
		{Key: settingS3PrivateBucket, Value: strconv.FormatBool(config.S3.PrivateBucket), Description: "S3 是否为私有存储桶", UpdatedAt: now},
		{Key: settingS3URLExpire, Value: strconv.Itoa(config.S3.URLExpireSeconds), Description: "S3 私有 URL 有效期（秒）", UpdatedAt: now},
	}, nil
}

func mergeInput(current Config, input UpdateInput) (Config, error) {
	candidate := Config{
		Backend:         strings.ToLower(strings.TrimSpace(input.Backend)),
		LocalConfigured: current.LocalConfigured,
		Qiniu: QiniuConfig{
			AccessKey:        current.Qiniu.AccessKey,
			SecretKey:        current.Qiniu.SecretKey,
			BucketName:       strings.TrimSpace(input.Qiniu.BucketName),
			Domain:           strings.TrimSpace(input.Qiniu.Domain),
			PrivateBucket:    input.Qiniu.PrivateBucket,
			URLExpireSeconds: input.Qiniu.URLExpireSeconds,
			UploadURL:        strings.TrimSpace(input.Qiniu.UploadURL),
		},
		S3: S3Config{
			EndpointURL:      strings.TrimSpace(input.S3.EndpointURL),
			AccessKey:        current.S3.AccessKey,
			SecretKey:        current.S3.SecretKey,
			BucketName:       strings.TrimSpace(input.S3.BucketName),
			Region:           strings.TrimSpace(input.S3.Region),
			PublicURLBase:    strings.TrimSpace(input.S3.PublicURLBase),
			PrivateBucket:    input.S3.PrivateBucket,
			URLExpireSeconds: input.S3.URLExpireSeconds,
		},
	}
	if value := nonEmptyCredential(input.Qiniu.AccessKey); value != "" {
		candidate.Qiniu.AccessKey = value
	}
	if value := nonEmptyCredential(input.Qiniu.SecretKey); value != "" {
		candidate.Qiniu.SecretKey = value
	}
	if value := nonEmptyCredential(input.S3.AccessKey); value != "" {
		candidate.S3.AccessKey = value
	}
	if value := nonEmptyCredential(input.S3.SecretKey); value != "" {
		candidate.S3.SecretKey = value
	}
	candidate = normalizeDefaults(candidate)
	if err := validateConfig(candidate); err != nil {
		return Config{}, err
	}
	return candidate, nil
}

func normalizeDefaults(config Config) Config {
	config.Backend = strings.ToLower(strings.TrimSpace(config.Backend))
	if strings.TrimSpace(config.Qiniu.UploadURL) == "" {
		config.Qiniu.UploadURL = defaultQiniuUploadURL
	}
	if config.Qiniu.URLExpireSeconds <= 0 {
		config.Qiniu.URLExpireSeconds = defaultURLExpireSeconds
	}
	if strings.TrimSpace(config.S3.Region) == "" {
		config.S3.Region = defaultS3Region
	}
	if config.S3.URLExpireSeconds <= 0 {
		config.S3.URLExpireSeconds = defaultURLExpireSeconds
	}
	return config
}

func validateConfig(config Config) error {
	if config.Backend != BackendLocal && config.Backend != BackendQiniu && config.Backend != BackendS3 {
		return badRequest("backend 必须是 local、qiniu 或 s3")
	}
	for name, value := range map[string]string{
		"qiniu.bucket_name":  config.Qiniu.BucketName,
		"qiniu.domain":       config.Qiniu.Domain,
		"qiniu.upload_url":   config.Qiniu.UploadURL,
		"s3.endpoint_url":    config.S3.EndpointURL,
		"s3.bucket_name":     config.S3.BucketName,
		"s3.region":          config.S3.Region,
		"s3.public_url_base": config.S3.PublicURLBase,
	} {
		if len(value) > maxSettingBytes || strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return badRequest(name + " 格式无效")
		}
	}
	for name, value := range map[string]string{
		"qiniu.access_key": config.Qiniu.AccessKey,
		"qiniu.secret_key": config.Qiniu.SecretKey,
		"s3.access_key":    config.S3.AccessKey,
		"s3.secret_key":    config.S3.SecretKey,
	} {
		if len(value) > maxCredentialBytes || strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return badRequest(name + " 格式无效")
		}
	}
	if err := validateExpire("qiniu.url_expire_seconds", config.Qiniu.URLExpireSeconds); err != nil {
		return err
	}
	if err := validateExpire("s3.url_expire_seconds", config.S3.URLExpireSeconds); err != nil {
		return err
	}
	if config.Backend == BackendQiniu {
		if missing := missingValues(map[string]string{
			"qiniu.access_key":  config.Qiniu.AccessKey,
			"qiniu.secret_key":  config.Qiniu.SecretKey,
			"qiniu.bucket_name": config.Qiniu.BucketName,
			"qiniu.domain":      config.Qiniu.Domain,
			"qiniu.upload_url":  config.Qiniu.UploadURL,
		}); len(missing) > 0 {
			return badRequest(strings.Join(missing, "、") + " 不能为空")
		}
	}
	if config.Backend == BackendS3 {
		if missing := missingValues(map[string]string{
			"s3.endpoint_url": config.S3.EndpointURL,
			"s3.access_key":   config.S3.AccessKey,
			"s3.secret_key":   config.S3.SecretKey,
			"s3.bucket_name":  config.S3.BucketName,
			"s3.region":       config.S3.Region,
		}); len(missing) > 0 {
			return badRequest(strings.Join(missing, "、") + " 不能为空")
		}
	}
	return nil
}

func validateExpire(name string, value int) error {
	if value < 1 || value > maxURLExpireSeconds {
		return badRequest(name + " 必须在 1 到 604800 秒之间")
	}
	return nil
}

func missingValues(values map[string]string) []string {
	missing := make([]string, 0)
	for key, value := range values {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}
	// Keep the public validation message deterministic.
	sort.Strings(missing)
	return missing
}

func parseStoredBool(value string, fallback bool, key string) (bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}

func parseStoredInt(value string, fallback int, key string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}

func encryptCredential(cipher Cipher, value string, key string) (string, error) {
	if value == "" {
		return "", nil
	}
	encrypted, err := cipher.Encrypt(value)
	if err != nil {
		return "", fmt.Errorf("encrypt %s: %w", key, err)
	}
	return encrypted, nil
}

func nonEmptyCredential(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func testPrepared(ctx context.Context, prepared PreparedRuntime) error {
	testCtx, cancel := context.WithTimeout(ctx, connectionTestTimeout)
	defer cancel()
	return prepared.Test(testCtx)
}

func settingsResponse(config Config, source string) SettingsResponse {
	qiniuAccessConfigured := strings.TrimSpace(config.Qiniu.AccessKey) != ""
	qiniuSecretConfigured := strings.TrimSpace(config.Qiniu.SecretKey) != ""
	s3AccessConfigured := strings.TrimSpace(config.S3.AccessKey) != ""
	s3SecretConfigured := strings.TrimSpace(config.S3.SecretKey) != ""
	return SettingsResponse{
		Backend: config.Backend,
		Source:  source,
		Local:   LocalSettingsResponse{Configured: config.LocalConfigured},
		Qiniu: QiniuSettingsResponse{
			BucketName:          config.Qiniu.BucketName,
			Domain:              config.Qiniu.Domain,
			PrivateBucket:       config.Qiniu.PrivateBucket,
			URLExpireSeconds:    config.Qiniu.URLExpireSeconds,
			UploadURL:           config.Qiniu.UploadURL,
			AccessKeyConfigured: qiniuAccessConfigured,
			SecretKeyConfigured: qiniuSecretConfigured,
			Configured: qiniuAccessConfigured && qiniuSecretConfigured &&
				config.Qiniu.BucketName != "" && config.Qiniu.Domain != "" && config.Qiniu.UploadURL != "",
		},
		S3: S3SettingsResponse{
			EndpointURL:         config.S3.EndpointURL,
			BucketName:          config.S3.BucketName,
			Region:              config.S3.Region,
			PublicURLBase:       config.S3.PublicURLBase,
			PrivateBucket:       config.S3.PrivateBucket,
			URLExpireSeconds:    config.S3.URLExpireSeconds,
			AccessKeyConfigured: s3AccessConfigured,
			SecretKeyConfigured: s3SecretConfigured,
			Configured: s3AccessConfigured && s3SecretConfigured &&
				config.S3.EndpointURL != "" && config.S3.BucketName != "" && config.S3.Region != "",
		},
	}
}

func backendLabel(backend string) string {
	switch backend {
	case BackendQiniu:
		return "七牛云"
	case BackendS3:
		return "S3"
	default:
		return "本地"
	}
}

func badRequest(message string) error {
	return Error{Kind: ErrBadRequest, Message: message}
}

func invalidConfig(cause error) error {
	return Error{Kind: ErrBadRequest, Message: "存储配置无效，请检查地址和必填项", Cause: cause}
}

func connectionFailed(cause error) error {
	return Error{Kind: ErrConnection, Message: "存储连接失败，请检查网络、凭据和存储桶权限", Cause: cause}
}
