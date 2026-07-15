package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	answerocrapp "mathstudy/backend-go/internal/application/answerocr"
	uploadapp "mathstudy/backend-go/internal/application/upload"
	"mathstudy/backend-go/internal/platform/uploadpath"
)

// LocalStorage persists uploads under the configured uploads directory.
type LocalStorage struct {
	uploadDir string
}

// NewLocalStorage creates a local filesystem upload adapter.
func NewLocalStorage(uploadDir string) *LocalStorage {
	return &LocalStorage{uploadDir: uploadDir}
}

// UploadStream writes one object and returns its Python-compatible /uploads URL.
func (s *LocalStorage) UploadStream(_ context.Context, reader io.Reader, key string, contentType string, _ int64) (uploadapp.StoredObject, error) {
	if strings.TrimSpace(s.uploadDir) == "" {
		return uploadapp.StoredObject{}, errors.New("upload directory is empty")
	}
	cleanKey, err := cleanObjectKey(key)
	if err != nil {
		return uploadapp.StoredObject{}, err
	}
	root, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return uploadapp.StoredObject{}, err
	}
	target := filepath.Join(root, filepath.FromSlash(cleanKey))
	if !isSubpath(root, target) {
		return uploadapp.StoredObject{}, errors.New("upload key escapes upload directory")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return uploadapp.StoredObject{}, err
	}
	// #nosec G304 -- cleanObjectKey and isSubpath constrain target to uploadDir.
	file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return uploadapp.StoredObject{}, err
	}
	written, copyErr := io.Copy(file, reader)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(target)
		return uploadapp.StoredObject{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(target)
		return uploadapp.StoredObject{}, closeErr
	}
	return uploadapp.StoredObject{
		Key:         cleanKey,
		URL:         "/uploads/" + cleanKey,
		Size:        written,
		ContentType: contentType,
	}, nil
}

// LoadImage resolves a local /uploads/images reference into validated image bytes.
func (s *LocalStorage) LoadImage(ctx context.Context, reference string) (answerocrapp.Image, error) {
	if err := ctx.Err(); err != nil {
		return answerocrapp.Image{}, err
	}
	if strings.TrimSpace(s.uploadDir) == "" || !uploadpath.IsImagePath(reference) {
		return answerocrapp.Image{}, answerocrapp.ErrInvalidImage
	}
	key := strings.TrimPrefix(strings.TrimSpace(reference), "/uploads/")
	root, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return answerocrapp.Image{}, fmt.Errorf("%w: resolve uploads directory", answerocrapp.ErrUnavailable)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return answerocrapp.Image{}, fmt.Errorf("%w: resolve uploads directory", answerocrapp.ErrUnavailable)
	}
	target := filepath.Join(root, filepath.FromSlash(key))
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		if os.IsNotExist(err) {
			return answerocrapp.Image{}, answerocrapp.ErrInvalidImage
		}
		return answerocrapp.Image{}, fmt.Errorf("%w: resolve local image", answerocrapp.ErrUnavailable)
	}
	if !isSubpath(resolvedRoot, resolvedTarget) {
		return answerocrapp.Image{}, answerocrapp.ErrInvalidImage
	}
	// #nosec G304 -- uploadpath validation and resolved-root containment constrain the target.
	file, err := os.Open(resolvedTarget)
	if err != nil {
		if os.IsNotExist(err) {
			return answerocrapp.Image{}, answerocrapp.ErrInvalidImage
		}
		return answerocrapp.Image{}, fmt.Errorf("%w: open local image", answerocrapp.ErrUnavailable)
	}
	defer file.Close()
	image, err := readValidatedImage(file, "")
	if err != nil {
		return answerocrapp.Image{}, err
	}
	if err := ctx.Err(); err != nil {
		return answerocrapp.Image{}, err
	}
	return image, nil
}

func cleanObjectKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if strings.ContainsAny(key, "\\?#%:") {
		return "", errors.New("upload key contains unsafe character")
	}
	for _, value := range key {
		if value < 0x20 || value == 0x7f {
			return "", errors.New("upload key contains control character")
		}
	}
	key = strings.TrimPrefix(key, "/")
	if key == "" {
		return "", errors.New("upload key is empty")
	}
	parts := strings.Split(key, "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return "", errors.New("upload key contains parent directory segment")
		}
		cleaned = append(cleaned, part)
	}
	if len(cleaned) == 0 {
		return "", errors.New("upload key is empty")
	}
	if !isAllowedObjectPrefix(cleaned[0]) {
		return "", errors.New("upload key has unsupported prefix")
	}
	return strings.Join(cleaned, "/"), nil
}

func isAllowedObjectPrefix(value string) bool {
	return value == "images" || value == "documents" || value == "videos"
}

func isSubpath(root string, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}
