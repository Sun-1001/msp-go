package upload

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"mathstudy/backend-go/internal/platform/identifier"
	"mathstudy/backend-go/internal/platform/uploadpath"
)

const (
	// MaxImageSize keeps parity with the Python image upload limit.
	MaxImageSize = 10 * 1024 * 1024
	// MaxResourceSize keeps parity with the Python video/document upload limit.
	MaxResourceSize = 500 * 1024 * 1024
)

var (
	// ErrInvalidContentType is returned when a file MIME type is not allowed.
	ErrInvalidContentType = errors.New("invalid content type")
	// ErrFileTooLarge is returned when a file exceeds its endpoint limit.
	ErrFileTooLarge = errors.New("file too large")
)

// Storage persists uploaded bytes and returns an externally usable URL.
type Storage interface {
	UploadStream(context.Context, io.Reader, string, string, int64) (StoredObject, error)
}

// StoredObject stores the result returned by a storage adapter.
type StoredObject struct {
	Key         string
	URL         string
	Size        int64
	ContentType string
}

// FileMeta stores trusted metadata supplied by the HTTP multipart layer.
type FileMeta struct {
	ContentType string
	Size        int64
}

// Response is the Python-compatible upload response.
type Response struct {
	FileID      string `json:"file_id"`
	URL         string `json:"url"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// Service validates and stores uploaded files.
type Service struct {
	storage Storage
	newID   func() (string, error)
}

// NewService creates an upload service.
func NewService(storage Storage) (*Service, error) {
	if storage == nil {
		return nil, errors.New("upload storage is nil")
	}
	return &Service{storage: storage, newID: newUUID}, nil
}

// SaveImage validates and stores one image file.
func (s *Service) SaveImage(ctx context.Context, reader io.Reader, meta FileMeta) (Response, error) {
	content, contentType, extension, err := validateImageContent(reader, meta)
	if err != nil {
		return Response{}, err
	}
	return s.store(ctx, bytes.NewReader(content), contentType, int64(len(content)), MaxImageSize, extension, "images")
}

// SaveResourceFile validates and stores one video or document resource.
func (s *Service) SaveResourceFile(ctx context.Context, reader io.Reader, meta FileMeta) (Response, error) {
	prefix := "documents"
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(meta.ContentType)), "video/") {
		prefix = "videos"
	}
	return s.saveResource(ctx, reader, meta, MaxResourceSize, prefix)
}

func (s *Service) saveResource(ctx context.Context, reader io.Reader, meta FileMeta, maxSize int64, prefix string) (Response, error) {
	contentType := strings.ToLower(strings.TrimSpace(meta.ContentType))
	extension, ok := allowedResourceTypes()[contentType]
	if !ok {
		return Response{}, ErrInvalidContentType
	}
	if meta.Size > maxSize {
		return Response{}, ErrFileTooLarge
	}
	if reader == nil {
		return Response{}, errors.New("upload reader is nil")
	}
	checkedReader, err := validateResourceContent(reader, contentType)
	if err != nil {
		return Response{}, err
	}
	return s.store(ctx, checkedReader, contentType, meta.Size, maxSize, extension, prefix)
}

func (s *Service) store(ctx context.Context, reader io.Reader, contentType string, sizeHint int64, maxSize int64, extension string, prefix string) (Response, error) {
	fileID, err := s.newID()
	if err != nil {
		return Response{}, err
	}
	filename := fileID + extension
	key := prefix + "/" + filename
	limited := &maxBytesReader{reader: reader, remaining: maxSize}
	counted := &countingReader{reader: limited}
	stored, err := s.storage.UploadStream(ctx, counted, key, contentType, sizeHint)
	if err != nil {
		return Response{}, err
	}
	size := stored.Size
	if size == 0 && counted.n > 0 {
		size = counted.n
	}
	return Response{
		FileID:      fileID,
		URL:         stored.URL,
		Filename:    filename,
		ContentType: contentType,
		Size:        size,
	}, nil
}

func validateImageContent(reader io.Reader, meta FileMeta) ([]byte, string, string, error) {
	if reader == nil {
		return nil, "", "", errors.New("upload reader is nil")
	}
	if meta.Size > MaxImageSize {
		return nil, "", "", ErrFileTooLarge
	}
	content, err := io.ReadAll(io.LimitReader(reader, MaxImageSize+1))
	if err != nil {
		return nil, "", "", err
	}
	if int64(len(content)) > MaxImageSize {
		return nil, "", "", ErrFileTooLarge
	}
	if len(content) == 0 {
		return nil, "", "", ErrInvalidContentType
	}
	contentType := strings.ToLower(strings.TrimSpace(http.DetectContentType(content)))
	if isWebP(content) {
		contentType = "image/webp"
	}
	extension, ok := allowedImageTypes()[contentType]
	if !ok {
		return nil, "", "", ErrInvalidContentType
	}
	if contentType != "image/webp" {
		if _, _, err := image.DecodeConfig(bytes.NewReader(content)); err != nil {
			return nil, "", "", ErrInvalidContentType
		}
	}
	return content, contentType, extension, nil
}

func isWebP(content []byte) bool {
	return len(content) >= 12 &&
		string(content[0:4]) == "RIFF" &&
		string(content[8:12]) == "WEBP"
}

func validateResourceContent(reader io.Reader, contentType string) (io.Reader, error) {
	prefix, err := readPrefix(reader, 512)
	if err != nil {
		return nil, err
	}
	if !resourceContentMatches(contentType, prefix) {
		return nil, ErrInvalidContentType
	}
	checked := io.MultiReader(bytes.NewReader(prefix), reader)
	if isTextResourceContentType(contentType) {
		return &textResourceReader{reader: checked}, nil
	}
	return checked, nil
}

func readPrefix(reader io.Reader, limit int) ([]byte, error) {
	buffer := make([]byte, limit)
	n, err := io.ReadFull(reader, buffer)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	return buffer[:n], nil
}

func resourceContentMatches(contentType string, prefix []byte) bool {
	switch contentType {
	case "application/pdf":
		return bytes.HasPrefix(prefix, []byte("%PDF-"))
	case "application/msword", "application/vnd.ms-powerpoint":
		return bytes.HasPrefix(prefix, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1})
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return bytes.HasPrefix(prefix, []byte("PK\x03\x04")) || bytes.HasPrefix(prefix, []byte("PK\x05\x06")) || bytes.HasPrefix(prefix, []byte("PK\x07\x08"))
	case "video/mp4", "video/quicktime":
		return hasISOBaseMediaFileType(prefix)
	case "video/avi":
		return bytes.HasPrefix(prefix, []byte("RIFF")) && len(prefix) >= 12 && string(prefix[8:12]) == "AVI "
	case "video/webm", "video/x-matroska":
		return bytes.HasPrefix(prefix, []byte{0x1a, 0x45, 0xdf, 0xa3})
	case "text/plain", "text/markdown":
		return utf8.Valid(prefix) && !bytes.Contains(prefix, []byte{0})
	default:
		return len(prefix) > 0
	}
}

func isTextResourceContentType(contentType string) bool {
	return contentType == "text/plain" || contentType == "text/markdown"
}

func hasISOBaseMediaFileType(prefix []byte) bool {
	searchLimit := min(len(prefix), 64)
	return bytes.Contains(prefix[:searchLimit], []byte("ftyp"))
}

// IsSafeImagePath reports whether value is a local URL path produced by SaveImage.
func IsSafeImagePath(value string) bool {
	return uploadpath.IsImagePath(value)
}

type maxBytesReader struct {
	reader    io.Reader
	remaining int64
}

func (r *maxBytesReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		var probe [1]byte
		n, err := r.reader.Read(probe[:])
		if n > 0 {
			return 0, ErrFileTooLarge
		}
		return 0, err
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.reader.Read(p)
	r.remaining -= int64(n)
	return n, err
}

type textResourceReader struct {
	reader  io.Reader
	pending []byte
}

func (r *textResourceReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		if bytes.Contains(p[:n], []byte{0}) {
			return 0, ErrInvalidContentType
		}
		combined := make([]byte, len(r.pending)+n)
		copy(combined, r.pending)
		copy(combined[len(r.pending):], p[:n])
		validLen, validateErr := validUTF8PrefixLen(combined, err == nil)
		if validateErr != nil {
			return 0, validateErr
		}
		r.pending = append(r.pending[:0], combined[validLen:]...)
	}
	if err == io.EOF && len(r.pending) > 0 {
		return 0, ErrInvalidContentType
	}
	return n, err
}

func validUTF8PrefixLen(data []byte, allowIncomplete bool) (int, error) {
	index := 0
	for index < len(data) {
		if !utf8.FullRune(data[index:]) {
			if allowIncomplete {
				return index, nil
			}
			return 0, ErrInvalidContentType
		}
		r, size := utf8.DecodeRune(data[index:])
		if r == utf8.RuneError && size == 1 {
			return 0, ErrInvalidContentType
		}
		index += size
	}
	return index, nil
}

type countingReader struct {
	reader io.Reader
	n      int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.n += int64(n)
	return n, err
}

func allowedImageTypes() map[string]string {
	return map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/gif":  ".gif",
		"image/webp": ".webp",
	}
}

func allowedResourceTypes() map[string]string {
	return map[string]string{
		"video/mp4":          ".mp4",
		"video/avi":          ".avi",
		"video/quicktime":    ".mov",
		"video/x-matroska":   ".mkv",
		"video/webm":         ".webm",
		"application/pdf":    ".pdf",
		"application/msword": ".doc",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   ".docx",
		"application/vnd.ms-powerpoint":                                             ".ppt",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
		"text/plain":    ".txt",
		"text/markdown": ".md",
	}
}

func newUUID() (string, error) {
	return identifier.NewUUID()
}
