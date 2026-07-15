package storage

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	answerocrapp "mathstudy/backend-go/internal/application/answerocr"
	"mathstudy/backend-go/internal/platform/config"
)

func TestLocalStorageLoadImageReadsValidatedImage(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "images"), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := testPNG(t)
	if err := os.WriteFile(filepath.Join(root, "images", "answer.png"), content, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	storage := NewLocalStorage(root)
	loaded, err := storage.LoadImage(context.Background(), "/uploads/images/answer.png")
	if err != nil {
		t.Fatalf("LoadImage() error = %v", err)
	}
	if loaded.MIMEType != "image/png" || !bytes.Equal(loaded.Data, content) {
		t.Fatalf("loaded image = mime %q data length %d", loaded.MIMEType, len(loaded.Data))
	}
}

func TestLocalStorageLoadImageRejectsUnsafeAndSpoofedFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "images"), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "images", "fake.png"), []byte("not an image"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	storage := NewLocalStorage(root)

	for _, reference := range []string{
		"https://example.com/images/answer.png",
		"/uploads/documents/answer.png",
		"/uploads/images/../answer.png",
		"/uploads/images/answer.png?token=1",
		"/uploads/images/%2e%2e/answer.png",
		"/uploads/images/missing.png",
		"/uploads/images/fake.png",
	} {
		t.Run(reference, func(t *testing.T) {
			_, err := storage.LoadImage(context.Background(), reference)
			if !errors.Is(err, answerocrapp.ErrInvalidImage) {
				t.Fatalf("LoadImage(%q) error = %v, want ErrInvalidImage", reference, err)
			}
		})
	}
}

func TestLocalStorageLoadImageRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "images"), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	outsideFile := filepath.Join(outside, "answer.png")
	if err := os.WriteFile(outsideFile, testPNG(t), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	link := filepath.Join(root, "images", "linked.png")
	if err := os.Symlink(outsideFile, link); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}

	_, err := NewLocalStorage(root).LoadImage(context.Background(), "/uploads/images/linked.png")
	if !errors.Is(err, answerocrapp.ErrInvalidImage) {
		t.Fatalf("LoadImage(symlink escape) error = %v, want ErrInvalidImage", err)
	}
}

func TestReadValidatedImageEnforcesSizeMIMEAndDecode(t *testing.T) {
	pngData := testPNG(t)
	jpegData := testJPEG(t)
	tests := []struct {
		name        string
		data        []byte
		contentType string
		wantMIME    string
		wantErr     error
	}{
		{name: "PNG", data: pngData, contentType: "image/png; charset=binary", wantMIME: "image/png"},
		{name: "JPEG", data: jpegData, contentType: "image/jpeg", wantMIME: "image/jpeg"},
		{name: "WebP without runtime decoder", data: []byte("RIFF\x16\x00\x00\x00WEBPVP8X\x0a\x00\x00\x00\x00\x00\x00\x00\x00\x00"), contentType: "image/webp", wantErr: answerocrapp.ErrInvalidImage},
		{name: "MIME mismatch", data: pngData, contentType: "image/jpeg", wantErr: answerocrapp.ErrInvalidImage},
		{name: "spoofed bytes", data: []byte("not an image"), contentType: "image/png", wantErr: answerocrapp.ErrInvalidImage},
		{name: "truncated PNG pixels", data: pngData[:len(pngData)/2], contentType: "image/png", wantErr: answerocrapp.ErrInvalidImage},
		{name: "truncated JPEG pixels", data: jpegData[:len(jpegData)-2], contentType: "image/jpeg", wantErr: answerocrapp.ErrInvalidImage},
		{name: "oversized", data: make([]byte, answerocrapp.MaxImageSize+1), wantErr: answerocrapp.ErrInvalidImage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loaded, err := readValidatedImage(bytes.NewReader(tt.data), tt.contentType)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("readValidatedImage() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil || loaded.MIMEType != tt.wantMIME {
				t.Fatalf("readValidatedImage() = %#v, %v", loaded, err)
			}
		})
	}
}

func TestReadValidatedImageRejectsExcessiveDimensions(t *testing.T) {
	var buffer bytes.Buffer
	canvas := image.NewRGBA(image.Rect(0, 0, maxOCRImageDimension+1, 1))
	if err := png.Encode(&buffer, canvas); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	_, err := readValidatedImage(bytes.NewReader(buffer.Bytes()), "image/png")
	if !errors.Is(err, answerocrapp.ErrInvalidImage) {
		t.Fatalf("readValidatedImage() error = %v, want ErrInvalidImage", err)
	}
}

func TestS3StorageLoadImageUsesConfiguredNamespaceAndFreshDownloadURL(t *testing.T) {
	pngData := testPNG(t)
	var requestURL string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requestURL = r.URL.String()
		return imageHTTPResponse(r, http.StatusOK, "image/png", pngData), nil
	})}
	storage, err := NewS3Storage(S3Config{
		EndpointURL:   "https://s3.example.com/root",
		AccessKey:     "access",
		SecretKey:     "secret",
		BucketName:    "bucket",
		Region:        "us-east-1",
		PrivateBucket: true,
		URLExpire:     time.Hour,
	}, client)
	if err != nil {
		t.Fatalf("NewS3Storage() error = %v", err)
	}
	storage.now = func() time.Time { return time.Date(2026, time.July, 15, 10, 0, 0, 0, time.UTC) }
	reference := storage.downloadURL("images/answer.png") + "&ignored=client-value"

	loaded, err := storage.LoadImage(context.Background(), reference)
	if err != nil {
		t.Fatalf("LoadImage() error = %v", err)
	}
	if loaded.MIMEType != "image/png" || strings.Contains(requestURL, "ignored=client-value") {
		t.Fatalf("loaded=%#v requestURL=%q", loaded, requestURL)
	}
	for _, fragment := range []string{"https://s3.example.com/root/bucket/images/answer.png?", "X-Amz-Signature="} {
		if !strings.Contains(requestURL, fragment) {
			t.Fatalf("request URL %q missing %q", requestURL, fragment)
		}
	}
}

func TestS3StorageLoadImageRejectsForeignOrAmbiguousReferences(t *testing.T) {
	storage, err := NewS3Storage(S3Config{
		EndpointURL:   "https://s3.example.com",
		AccessKey:     "access",
		SecretKey:     "secret",
		BucketName:    "bucket",
		Region:        "us-east-1",
		PublicURLBase: "https://cdn.example.com/base",
	}, &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return imageHTTPResponse(r, http.StatusOK, "image/png", testPNG(t)), nil
	})})
	if err != nil {
		t.Fatalf("NewS3Storage() error = %v", err)
	}
	for _, reference := range []string{
		"https://evil.example.com/base/images/answer.png",
		"https://cdn.example.com/base-other/images/answer.png",
		"https://cdn.example.com/base/documents/answer.png",
		"https://cdn.example.com/base/images/%2e%2e/answer.png",
		"https://cdn.example.com/base/images/answer.png?redirect=1",
		"https://user@cdn.example.com/base/images/answer.png",
		"https://cdn.example.com/base/images/answer.png#fragment",
	} {
		t.Run(reference, func(t *testing.T) {
			_, err := storage.LoadImage(context.Background(), reference)
			if !errors.Is(err, answerocrapp.ErrInvalidImage) {
				t.Fatalf("LoadImage(%q) error = %v, want ErrInvalidImage", reference, err)
			}
		})
	}
}

func TestQiniuStorageLoadImageUsesConfiguredDomainAndFreshSignature(t *testing.T) {
	pngData := testPNG(t)
	var requestURL string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requestURL = r.URL.String()
		return imageHTTPResponse(r, http.StatusOK, "image/png", pngData), nil
	})}
	storage, err := NewQiniuStorage(QiniuConfig{
		AccessKey:     "access",
		SecretKey:     "secret",
		BucketName:    "bucket",
		Domain:        "https://cdn.example.com/base",
		PrivateBucket: true,
		URLExpire:     time.Hour,
		UploadURL:     "https://upload.example.com",
	}, client)
	if err != nil {
		t.Fatalf("NewQiniuStorage() error = %v", err)
	}
	storage.now = func() time.Time { return time.Date(2026, time.July, 15, 10, 0, 0, 0, time.UTC) }
	reference := storage.downloadURL("images/answer.png") + "&ignored=client-value"

	loaded, err := storage.LoadImage(context.Background(), reference)
	if err != nil {
		t.Fatalf("LoadImage() error = %v", err)
	}
	if loaded.MIMEType != "image/png" || strings.Contains(requestURL, "ignored=client-value") {
		t.Fatalf("loaded=%#v requestURL=%q", loaded, requestURL)
	}
	if !strings.Contains(requestURL, "https://cdn.example.com/base/images/answer.png?e=") || !strings.Contains(requestURL, "&token=access:") {
		t.Fatalf("request URL = %q", requestURL)
	}
}

func TestRemoteImageLoadDoesNotFollowRedirects(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		response := imageHTTPResponse(r, http.StatusFound, "text/plain", nil)
		response.Header.Set("Location", "https://evil.example.com/image.png")
		return response, nil
	})}
	storage, err := NewQiniuStorage(QiniuConfig{
		AccessKey:  "access",
		SecretKey:  "secret",
		BucketName: "bucket",
		Domain:     "https://cdn.example.com",
		UploadURL:  "https://upload.example.com",
	}, client)
	if err != nil {
		t.Fatalf("NewQiniuStorage() error = %v", err)
	}

	_, err = storage.LoadImage(context.Background(), "https://cdn.example.com/images/answer.png")
	if !errors.Is(err, answerocrapp.ErrUnavailable) || calls != 1 {
		t.Fatalf("LoadImage() error=%v calls=%d", err, calls)
	}
}

func TestRemoteImageLoadRejectsOversizedContentLength(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		response := imageHTTPResponse(r, http.StatusOK, "image/png", testPNG(t))
		response.ContentLength = answerocrapp.MaxImageSize + 1
		return response, nil
	})}
	storage, err := NewQiniuStorage(QiniuConfig{
		AccessKey:  "access",
		SecretKey:  "secret",
		BucketName: "bucket",
		Domain:     "https://cdn.example.com",
		UploadURL:  "https://upload.example.com",
	}, client)
	if err != nil {
		t.Fatalf("NewQiniuStorage() error = %v", err)
	}
	_, err = storage.LoadImage(context.Background(), "https://cdn.example.com/images/answer.png")
	if !errors.Is(err, answerocrapp.ErrInvalidImage) {
		t.Fatalf("LoadImage() error = %v, want ErrInvalidImage", err)
	}
}

func TestNewUploadStorageReturnsCombinedBackend(t *testing.T) {
	backend, err := NewUploadStorage(config.Config{StorageBackend: "local", UploadsDir: t.TempDir()}, nil)
	if err != nil {
		t.Fatalf("NewUploadStorage() error = %v", err)
	}
	if backend == nil {
		t.Fatal("NewUploadStorage() backend = nil")
	}
}

func imageHTTPResponse(request *http.Request, status int, contentType string, data []byte) *http.Response {
	header := make(http.Header)
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	return &http.Response{
		StatusCode:    status,
		Status:        http.StatusText(status),
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(data)),
		ContentLength: int64(len(data)),
		Request:       request,
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	canvas := image.NewRGBA(image.Rect(0, 0, 2, 2))
	canvas.Set(0, 0, color.Black)
	canvas.Set(1, 0, color.White)
	if err := png.Encode(&buffer, canvas); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return buffer.Bytes()
}

func testJPEG(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	canvas := image.NewRGBA(image.Rect(0, 0, 2, 2))
	canvas.Set(0, 0, color.Black)
	canvas.Set(1, 0, color.White)
	if err := jpeg.Encode(&buffer, canvas, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}
	return buffer.Bytes()
}
