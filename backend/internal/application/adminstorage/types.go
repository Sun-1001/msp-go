package adminstorage

import (
	"context"
	"errors"
	"time"
)

const (
	BackendLocal       = "local"
	BackendQiniu       = "qiniu"
	BackendS3          = "s3"
	SourceUnconfigured = "unconfigured"
	SourceDB           = "database"
)

var (
	// ErrBadRequest is returned when storage settings are invalid.
	ErrBadRequest = errors.New("bad storage settings request")
	// ErrConnection is returned when a storage backend cannot complete a write probe.
	ErrConnection = errors.New("storage connection failed")
)

// Error keeps a safe public message while retaining the internal cause for logs.
type Error struct {
	Kind    error
	Message string
	Cause   error
}

func (e Error) Error() string {
	return e.Message
}

func (e Error) Unwrap() error {
	if e.Cause != nil {
		return errors.Join(e.Kind, e.Cause)
	}
	return e.Kind
}

// Cipher protects object-storage credentials at rest.
type Cipher interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}

// Repository persists administrator-managed storage settings.
type Repository interface {
	GetSettings(context.Context, []string) (map[string]string, error)
	SaveStorageSettings(context.Context, []SettingUpdate) error
}

// Runtime prepares immutable storage adapters before they become active.
type Runtime interface {
	Prepare(Config) (PreparedRuntime, error)
}

// PreparedRuntime is safe to probe and can be atomically activated.
type PreparedRuntime interface {
	Test(context.Context) error
	Activate()
}

// SettingUpdate stores one system setting mutation.
type SettingUpdate struct {
	Key         string
	Value       string
	Description string
	UpdatedAt   time.Time
}

// Config is the complete secret-bearing runtime configuration.
type Config struct {
	Backend         string
	LocalConfigured bool
	Qiniu           QiniuConfig
	S3              S3Config
}

// QiniuConfig contains the complete Qiniu Kodo runtime settings.
type QiniuConfig struct {
	AccessKey        string
	SecretKey        string
	BucketName       string
	Domain           string
	PrivateBucket    bool
	URLExpireSeconds int
	UploadURL        string
}

// S3Config contains the complete S3-compatible runtime settings.
type S3Config struct {
	EndpointURL      string
	AccessKey        string
	SecretKey        string
	BucketName       string
	Region           string
	PublicURLBase    string
	PrivateBucket    bool
	URLExpireSeconds int
}

// UpdateInput replaces non-secret settings and optionally rotates credentials.
type UpdateInput struct {
	Backend string             `json:"backend"`
	Qiniu   QiniuSettingsInput `json:"qiniu"`
	S3      S3SettingsInput    `json:"s3"`
}

// QiniuSettingsInput is the administrator-editable Qiniu configuration.
type QiniuSettingsInput struct {
	AccessKey        *string `json:"access_key"`
	SecretKey        *string `json:"secret_key"`
	BucketName       string  `json:"bucket_name"`
	Domain           string  `json:"domain"`
	PrivateBucket    bool    `json:"private_bucket"`
	URLExpireSeconds int     `json:"url_expire_seconds"`
	UploadURL        string  `json:"upload_url"`
}

// S3SettingsInput is the administrator-editable S3-compatible configuration.
type S3SettingsInput struct {
	EndpointURL      string  `json:"endpoint_url"`
	AccessKey        *string `json:"access_key"`
	SecretKey        *string `json:"secret_key"`
	BucketName       string  `json:"bucket_name"`
	Region           string  `json:"region"`
	PublicURLBase    string  `json:"public_url_base"`
	PrivateBucket    bool    `json:"private_bucket"`
	URLExpireSeconds int     `json:"url_expire_seconds"`
}

// SettingsResponse is safe for the administrator UI and never contains credentials.
type SettingsResponse struct {
	Backend string                `json:"backend"`
	Source  string                `json:"source"`
	Local   LocalSettingsResponse `json:"local"`
	Qiniu   QiniuSettingsResponse `json:"qiniu"`
	S3      S3SettingsResponse    `json:"s3"`
}

// LocalSettingsResponse reports whether the deployment has a usable local directory.
type LocalSettingsResponse struct {
	Configured bool `json:"configured"`
}

// QiniuSettingsResponse contains only non-secret Qiniu settings and credential state.
type QiniuSettingsResponse struct {
	BucketName          string `json:"bucket_name"`
	Domain              string `json:"domain"`
	PrivateBucket       bool   `json:"private_bucket"`
	URLExpireSeconds    int    `json:"url_expire_seconds"`
	UploadURL           string `json:"upload_url"`
	AccessKeyConfigured bool   `json:"access_key_configured"`
	SecretKeyConfigured bool   `json:"secret_key_configured"`
	Configured          bool   `json:"configured"`
}

// S3SettingsResponse contains only non-secret S3 settings and credential state.
type S3SettingsResponse struct {
	EndpointURL         string `json:"endpoint_url"`
	BucketName          string `json:"bucket_name"`
	Region              string `json:"region"`
	PublicURLBase       string `json:"public_url_base"`
	PrivateBucket       bool   `json:"private_bucket"`
	URLExpireSeconds    int    `json:"url_expire_seconds"`
	AccessKeyConfigured bool   `json:"access_key_configured"`
	SecretKeyConfigured bool   `json:"secret_key_configured"`
	Configured          bool   `json:"configured"`
}

// TestResponse describes a successful write probe.
type TestResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Backend   string `json:"backend"`
	LatencyMS int64  `json:"latency_ms"`
}
