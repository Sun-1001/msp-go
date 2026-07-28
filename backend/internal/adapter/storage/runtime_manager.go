package storage

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"time"

	adminstorageapp "mathstudy/backend/internal/application/adminstorage"
	answerocrapp "mathstudy/backend/internal/application/answerocr"
	uploadapp "mathstudy/backend/internal/application/upload"
)

const (
	connectivityProbeKey     = "documents/.mathstudy-storage-connectivity-check.txt"
	connectivityProbeContent = "mathstudy storage connectivity check\n"
)

type runtimeState struct {
	backend UploadBackend
}

// RuntimeManager delegates each request to an immutable, atomically replaceable storage state.
type RuntimeManager struct {
	uploadDir string
	current   atomic.Pointer[runtimeState]
}

// NewRuntimeManager creates an inactive manager for administrator-managed storage.
func NewRuntimeManager(uploadDir string) *RuntimeManager {
	return &RuntimeManager{
		uploadDir: strings.TrimSpace(uploadDir),
	}
}

// LocalConfigured reports whether the deployment has a usable local root.
func (m *RuntimeManager) LocalConfigured() bool {
	return m != nil && m.uploadDir != ""
}

// Prepare builds a complete immutable state without changing active requests.
func (m *RuntimeManager) Prepare(cfg adminstorageapp.Config) (adminstorageapp.PreparedRuntime, error) {
	writer, err := m.backend(cfg.Backend, cfg)
	if err != nil {
		return nil, err
	}
	return &preparedRuntime{
		manager: m,
		state:   &runtimeState{backend: writer},
	}, nil
}

// UploadStream writes through the currently active backend.
func (m *RuntimeManager) UploadStream(ctx context.Context, reader io.Reader, key string, contentType string, size int64) (uploadapp.StoredObject, error) {
	state := m.loadState()
	if state == nil || state.backend == nil {
		return uploadapp.StoredObject{}, errors.New("upload storage is not configured")
	}
	return state.backend.UploadStream(ctx, reader, key, contentType, size)
}

// LoadImage reads only from the active administrator-selected backend.
func (m *RuntimeManager) LoadImage(ctx context.Context, reference string) (answerocrapp.Image, error) {
	state := m.loadState()
	if state == nil || state.backend == nil {
		return answerocrapp.Image{}, answerocrapp.ErrUnavailable
	}
	return state.backend.LoadImage(ctx, reference)
}

func (m *RuntimeManager) loadState() *runtimeState {
	if m == nil {
		return nil
	}
	return m.current.Load()
}

func (m *RuntimeManager) backend(backend string, cfg adminstorageapp.Config) (UploadBackend, error) {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case adminstorageapp.BackendLocal:
		if m.uploadDir == "" {
			return nil, errors.New("UPLOADS_DIR must not be empty for local storage")
		}
		return NewLocalStorage(m.uploadDir), nil
	case adminstorageapp.BackendQiniu:
		return NewQiniuStorage(QiniuConfig{
			AccessKey:     cfg.Qiniu.AccessKey,
			SecretKey:     cfg.Qiniu.SecretKey,
			BucketName:    cfg.Qiniu.BucketName,
			Domain:        cfg.Qiniu.Domain,
			PrivateBucket: cfg.Qiniu.PrivateBucket,
			URLExpire:     time.Duration(cfg.Qiniu.URLExpireSeconds) * time.Second,
			UploadURL:     cfg.Qiniu.UploadURL,
		}, nil)
	case adminstorageapp.BackendS3:
		return NewS3Storage(S3Config{
			EndpointURL:   cfg.S3.EndpointURL,
			AccessKey:     cfg.S3.AccessKey,
			SecretKey:     cfg.S3.SecretKey,
			BucketName:    cfg.S3.BucketName,
			Region:        cfg.S3.Region,
			PublicURLBase: cfg.S3.PublicURLBase,
			PrivateBucket: cfg.S3.PrivateBucket,
			URLExpire:     time.Duration(cfg.S3.URLExpireSeconds) * time.Second,
		}, nil)
	default:
		return nil, errors.New("unsupported upload storage backend")
	}
}

type preparedRuntime struct {
	manager *RuntimeManager
	state   *runtimeState
}

func (p *preparedRuntime) Test(ctx context.Context) error {
	if p == nil || p.state == nil || p.state.backend == nil {
		return errors.New("prepared storage is empty")
	}
	content := strings.NewReader(connectivityProbeContent)
	_, err := p.state.backend.UploadStream(
		ctx,
		content,
		connectivityProbeKey,
		"text/plain; charset=utf-8",
		int64(len(connectivityProbeContent)),
	)
	return err
}

func (p *preparedRuntime) Activate() {
	if p == nil || p.manager == nil || p.state == nil {
		return
	}
	p.manager.current.Store(p.state)
}
