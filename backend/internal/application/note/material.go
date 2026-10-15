package note

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	resourceapp "mathstudy/backend/internal/application/resource"
	uploadapp "mathstudy/backend/internal/application/upload"
)

const MaxMaterialBytes = 10 << 20

var ErrStorage = errors.New("私有材料存储不可用，请检查存储配置后重试")

type Material struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"-"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	ByteSize    int64     `json:"byte_size"`
	URI         string    `json:"-"`
	StorageKey  string    `json:"-"`
	CreatedAt   time.Time `json:"created_at"`
}

// Reuses the resource module's private document boundary; public buckets are rejected.
type MaterialStorage interface {
	StoreDocument(context.Context, io.Reader, string, string, int64) (resourceapp.ObjectSource, error)
	Open(context.Context, resourceapp.ObjectSource) (io.ReadCloser, resourceapp.ObjectMetadata, error)
}

type privateUpload struct{ storage MaterialStorage }

func (p privateUpload) UploadStream(ctx context.Context, reader io.Reader, key, contentType string, size int64) (uploadapp.StoredObject, error) {
	source, err := p.storage.StoreDocument(ctx, reader, key, contentType, size)
	if err != nil {
		return uploadapp.StoredObject{}, ErrStorage
	}
	return uploadapp.StoredObject{Key: source.StorageKey, URL: source.URI, Size: size, ContentType: contentType}, nil
}

func (s *Service) UploadMaterial(ctx context.Context, owner, filename string, reader io.Reader) (Material, error) {
	filename = filepath.Base(strings.ReplaceAll(filename, "\\", "/"))
	if filename == "." || utf8.RuneCountInString(filename) > 255 || strings.ContainsAny(filename, "\r\n\x00") {
		return Material{}, ErrInvalid
	}
	contentType := map[string]string{".pdf": "application/pdf", ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document", ".txt": "text/plain", ".md": "text/markdown"}[strings.ToLower(filepath.Ext(filename))]
	if contentType == "" {
		return Material{}, uploadapp.ErrInvalidContentType
	}
	data, err := io.ReadAll(io.LimitReader(reader, MaxMaterialBytes+1))
	if err != nil {
		return Material{}, err
	}
	if len(data) > MaxMaterialBytes {
		return Material{}, uploadapp.ErrFileTooLarge
	}
	if len(data) == 0 {
		return Material{}, ErrInvalid
	}
	uploader, err := uploadapp.NewService(privateUpload{s.storage})
	if err != nil {
		return Material{}, err
	}
	result, err := uploader.SaveMessageFile(ctx, bytes.NewReader(data), uploadapp.FileMeta{Size: int64(len(data)), ContentType: contentType})
	if err != nil {
		return Material{}, err
	}
	return s.repo.SaveMaterial(ctx, Material{ID: result.FileID, OwnerID: owner, Filename: filename, ContentType: contentType, ByteSize: result.Size, URI: result.URL, StorageKey: "documents/" + result.Filename})
}
func (s *Service) ListMaterials(ctx context.Context, owner string) ([]Material, error) {
	return s.repo.ListMaterials(ctx, owner)
}
func (s *Service) OpenMaterial(ctx context.Context, owner, id string) (Material, io.ReadCloser, error) {
	if !ValidID(id) {
		return Material{}, nil, ErrNotFound
	}
	material, err := s.repo.GetMaterial(ctx, owner, id)
	if err != nil {
		return Material{}, nil, err
	}
	reader, _, err := s.storage.Open(ctx, resourceapp.ObjectSource{URI: material.URI, StorageKey: material.StorageKey})
	if err != nil {
		return Material{}, nil, ErrStorage
	}
	return material, reader, nil
}
