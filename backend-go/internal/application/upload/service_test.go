package upload

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestSaveImageStoresAllowedContentType(t *testing.T) {
	content := validPNGBytes(t)
	storage := &fakeStorage{object: StoredObject{URL: "/uploads/images/id-1.png", Size: int64(len(content)), ContentType: "image/png"}}
	service := newTestService(storage, "id-1")

	response, err := service.SaveImage(context.Background(), bytes.NewReader(content), FileMeta{ContentType: " image/png ", Size: int64(len(content))})
	if err != nil {
		t.Fatalf("SaveImage() error = %v", err)
	}
	if storage.key != "images/id-1.png" || storage.contentType != "image/png" || !bytes.Equal(storage.data, content) {
		t.Fatalf("storage call = key %q contentType %q data %q", storage.key, storage.contentType, storage.data)
	}
	if response.FileID != "id-1" || response.Filename != "id-1.png" || response.URL != "/uploads/images/id-1.png" || response.Size != int64(len(content)) {
		t.Fatalf("response = %#v", response)
	}
}

func TestSaveResourceFileUsesVideoAndDocumentPrefixes(t *testing.T) {
	videoContent := "\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"
	storage := &fakeStorage{object: StoredObject{URL: "/uploads/videos/id-2.mp4", Size: int64(len(videoContent)), ContentType: "video/mp4"}}
	service := newTestService(storage, "id-2")

	response, err := service.SaveResourceFile(context.Background(), strings.NewReader(videoContent), FileMeta{ContentType: "video/mp4", Size: int64(len(videoContent))})
	if err != nil {
		t.Fatalf("SaveResourceFile(video) error = %v", err)
	}
	if storage.key != "videos/id-2.mp4" || response.Filename != "id-2.mp4" {
		t.Fatalf("video upload = key %q response %#v", storage.key, response)
	}

	storage.object = StoredObject{URL: "/uploads/documents/id-3.pdf", Size: 3, ContentType: "application/pdf"}
	service.newID = func() (string, error) { return "id-3", nil }
	response, err = service.SaveResourceFile(context.Background(), strings.NewReader("%PDF-1.7\nbody"), FileMeta{ContentType: "application/pdf", Size: 13})
	if err != nil {
		t.Fatalf("SaveResourceFile(document) error = %v", err)
	}
	if storage.key != "documents/id-3.pdf" || response.Filename != "id-3.pdf" {
		t.Fatalf("document upload = key %q response %#v", storage.key, response)
	}
}

func TestSaveResourceFileRejectsSpoofedDocumentContent(t *testing.T) {
	service := newTestService(&fakeStorage{}, "id-1")
	if _, err := service.SaveResourceFile(context.Background(), strings.NewReader("not a pdf"), FileMeta{ContentType: "application/pdf", Size: 9}); !errors.Is(err, ErrInvalidContentType) {
		t.Fatalf("spoofed pdf error = %v, want ErrInvalidContentType", err)
	}
	if _, err := service.SaveResourceFile(context.Background(), strings.NewReader("not a video"), FileMeta{ContentType: "video/mp4", Size: 11}); !errors.Is(err, ErrInvalidContentType) {
		t.Fatalf("spoofed video error = %v, want ErrInvalidContentType", err)
	}
	if _, err := service.SaveResourceFile(context.Background(), strings.NewReader("hello\x00world"), FileMeta{ContentType: "text/plain", Size: 11}); !errors.Is(err, ErrInvalidContentType) {
		t.Fatalf("text with nul error = %v, want ErrInvalidContentType", err)
	}
}

func TestSaveResourceFileRejectsBinaryAfterTextPrefix(t *testing.T) {
	service := newTestService(&fakeStorage{}, "id-1")
	content := strings.Repeat("a", 600) + "\x00binary"

	if _, err := service.SaveResourceFile(context.Background(), strings.NewReader(content), FileMeta{ContentType: "text/plain", Size: int64(len(content))}); !errors.Is(err, ErrInvalidContentType) {
		t.Fatalf("text with late binary error = %v, want ErrInvalidContentType", err)
	}
}

func TestTextResourceReaderValidatesUTF8AcrossReadBoundaries(t *testing.T) {
	content := []byte("逐字节读取高等数学")
	reader := &textResourceReader{reader: &singleByteReader{data: content}}
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll(valid UTF-8) error = %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("ReadAll(valid UTF-8) = %q", got)
	}

	invalid := &textResourceReader{reader: &singleByteReader{data: []byte{'a', 0xe4, 0xb8}}}
	if _, err := io.ReadAll(invalid); !errors.Is(err, ErrInvalidContentType) {
		t.Fatalf("ReadAll(truncated UTF-8) error = %v", err)
	}

	malformed := &textResourceReader{reader: &singleByteReader{data: []byte{'a', 0xe4, 0x20, 0xad}}}
	if _, err := io.ReadAll(malformed); !errors.Is(err, ErrInvalidContentType) {
		t.Fatalf("ReadAll(malformed UTF-8) error = %v", err)
	}
}

func BenchmarkTextResourceReader(b *testing.B) {
	content := bytes.Repeat([]byte("高等数学 calculus\n"), 4096)
	buffer := make([]byte, 32*1024)
	b.SetBytes(int64(len(content)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		reader := textResourceReader{reader: bytes.NewReader(content)}
		for {
			_, err := reader.Read(buffer)
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

func TestSaveResourceFilePreservesPrefixBytesAfterValidation(t *testing.T) {
	storage := &fakeStorage{object: StoredObject{URL: "/uploads/documents/id-1.pdf", ContentType: "application/pdf"}}
	service := newTestService(storage, "id-1")
	content := "%PDF-1.7\nbody"

	_, err := service.SaveResourceFile(context.Background(), strings.NewReader(content), FileMeta{ContentType: "application/pdf", Size: int64(len(content))})
	if err != nil {
		t.Fatalf("SaveResourceFile() error = %v", err)
	}
	if string(storage.data) != content {
		t.Fatalf("stored data = %q, want %q", string(storage.data), content)
	}
}

func TestIsSafeImagePath(t *testing.T) {
	valid := []string{
		"/uploads/images/file.png",
		" /uploads/images/nested/file.webp ",
	}
	for _, value := range valid {
		t.Run("valid "+value, func(t *testing.T) {
			if !IsSafeImagePath(value) {
				t.Fatalf("IsSafeImagePath(%q) = false, want true", value)
			}
		})
	}

	invalid := []string{
		"",
		"https://example.com/file.png",
		"/uploads/file.png",
		"/uploads/documents/file.pdf",
		"/uploads/images/../documents/file.pdf",
		"/uploads/images/file.png?download=1",
		"/uploads/images/file.png#fragment",
		`/uploads/images\file.png`,
		"/uploads/images/%2e%2e/file.png",
	}
	for _, value := range invalid {
		t.Run("invalid "+value, func(t *testing.T) {
			if IsSafeImagePath(value) {
				t.Fatalf("IsSafeImagePath(%q) = true, want false", value)
			}
		})
	}
}

func TestSaveRejectsInvalidContentTypeAndLargeFiles(t *testing.T) {
	service := newTestService(&fakeStorage{}, "id-1")
	if _, err := service.SaveImage(context.Background(), strings.NewReader("data"), FileMeta{ContentType: "image/svg+xml"}); !errors.Is(err, ErrInvalidContentType) {
		t.Fatalf("invalid type error = %v, want ErrInvalidContentType", err)
	}
	if _, err := service.SaveImage(context.Background(), strings.NewReader("not a png"), FileMeta{ContentType: "image/png"}); !errors.Is(err, ErrInvalidContentType) {
		t.Fatalf("spoofed type error = %v, want ErrInvalidContentType", err)
	}
	if _, err := service.SaveImage(context.Background(), strings.NewReader("data"), FileMeta{ContentType: "image/png", Size: MaxImageSize + 1}); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("meta size error = %v, want ErrFileTooLarge", err)
	}
	oversized := bytes.NewReader(bytes.Repeat([]byte("x"), MaxImageSize+1))
	if _, err := service.SaveImage(context.Background(), oversized, FileMeta{ContentType: "image/png"}); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("read size error = %v, want ErrFileTooLarge", err)
	}
}

func TestSaveReturnsStorageAndIDErrors(t *testing.T) {
	content := validPNGBytes(t)
	service := newTestService(&fakeStorage{err: errors.New("store failed")}, "id-1")
	if _, err := service.SaveImage(context.Background(), bytes.NewReader(content), FileMeta{ContentType: "image/png", Size: int64(len(content))}); err == nil || err.Error() != "store failed" {
		t.Fatalf("storage error = %v", err)
	}

	service = newTestService(&fakeStorage{}, "id-1")
	service.newID = func() (string, error) { return "", errors.New("id failed") }
	if _, err := service.SaveImage(context.Background(), bytes.NewReader(content), FileMeta{ContentType: "image/png", Size: int64(len(content))}); err == nil || err.Error() != "id failed" {
		t.Fatalf("id error = %v", err)
	}
}

func TestNewServiceRejectsNilStorage(t *testing.T) {
	if _, err := NewService(nil); err == nil {
		t.Fatal("NewService(nil) error = nil, want error")
	}
}

func newTestService(storage Storage, id string) *Service {
	service, err := NewService(storage)
	if err != nil {
		panic(err)
	}
	service.newID = func() (string, error) { return id, nil }
	return service
}

func validPNGBytes(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatalf("decode png fixture: %v", err)
	}
	return data
}

type fakeStorage struct {
	object      StoredObject
	err         error
	data        []byte
	key         string
	contentType string
}

type singleByteReader struct {
	data   []byte
	offset int
}

func (r *singleByteReader) Read(target []byte) (int, error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	if len(target) == 0 {
		return 0, nil
	}
	target[0] = r.data[r.offset]
	r.offset++
	return 1, nil
}

func (s *fakeStorage) UploadStream(_ context.Context, reader io.Reader, key string, contentType string, _ int64) (StoredObject, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return StoredObject{}, err
	}
	s.data = append([]byte(nil), data...)
	s.key = key
	s.contentType = contentType
	if s.err != nil {
		return StoredObject{}, s.err
	}
	if s.object.Key == "" {
		s.object.Key = key
	}
	if s.object.Size == 0 {
		s.object.Size = int64(len(data))
	}
	if s.object.ContentType == "" {
		s.object.ContentType = contentType
	}
	return s.object, nil
}
