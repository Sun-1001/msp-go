package storage

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	answerocrapp "mathstudy/backend-go/internal/application/answerocr"
	uploadapp "mathstudy/backend-go/internal/application/upload"
	"mathstudy/backend-go/internal/platform/config"
)

// UploadBackend supports both upload writes and trusted answer-image reads.
type UploadBackend interface {
	uploadapp.Storage
	answerocrapp.ImageLoader
}

// NewUploadStorage creates the configured upload storage adapter.
func NewUploadStorage(cfg config.Config, logger *slog.Logger) (UploadBackend, error) {
	if logger == nil {
		logger = slog.Default()
	}
	switch strings.ToLower(strings.TrimSpace(cfg.StorageBackend)) {
	case "local":
		return NewLocalStorage(cfg.UploadsDir), nil
	case "s3":
		return NewS3Storage(S3Config{
			EndpointURL:   cfg.S3EndpointURL,
			AccessKey:     cfg.S3AccessKey,
			SecretKey:     cfg.S3SecretKey,
			BucketName:    cfg.S3BucketName,
			Region:        cfg.S3Region,
			PublicURLBase: cfg.S3PublicURLBase,
			PrivateBucket: cfg.S3PrivateBucket,
			URLExpire:     cfg.S3URLExpire,
		}, nil)
	case "qiniu":
		return NewQiniuStorage(QiniuConfig{
			AccessKey:     cfg.QiniuAccessKey,
			SecretKey:     cfg.QiniuSecretKey,
			BucketName:    cfg.QiniuBucketName,
			Domain:        cfg.QiniuDomain,
			PrivateBucket: cfg.QiniuPrivateBucket,
			URLExpire:     cfg.QiniuURLExpire,
			UploadURL:     cfg.QiniuUploadURL,
		}, nil)
	default:
		logger.Error("unsupported upload storage backend", "backend", cfg.StorageBackend)
		return nil, errors.New("unsupported upload storage backend")
	}
}

func defaultTimeout(client *http.Client) *http.Client {
	if client == nil {
		return &http.Client{Timeout: 5 * time.Minute}
	}
	if client.Timeout == 0 {
		copy := *client
		copy.Timeout = 5 * time.Minute
		return &copy
	}
	return client
}
