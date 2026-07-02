package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestS3StorageSignsAndUploadsPathStyleObject(t *testing.T) {
	var method string
	var path string
	var authHeader string
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.EscapedPath()
		authHeader = r.Header.Get("Authorization")
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		if r.Header.Get("X-Amz-Content-Sha256") == "" || r.Header.Get("X-Amz-Date") == "" || r.Header.Get("X-Amz-Acl") != "public-read" {
			t.Fatalf("headers = %#v", r.Header)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	storage, err := NewS3Storage(S3Config{
		EndpointURL:   server.URL,
		AccessKey:     "access",
		SecretKey:     "secret",
		BucketName:    "bucket",
		Region:        "us-east-1",
		PublicURLBase: "https://cdn.example.com/base",
	}, server.Client())
	if err != nil {
		t.Fatalf("NewS3Storage() error = %v", err)
	}
	storage.now = func() time.Time { return time.Date(2026, time.May, 6, 10, 0, 0, 0, time.UTC) }

	object, err := storage.UploadStream(context.Background(), strings.NewReader("data"), "documents/file name.pdf", "application/pdf", 4)
	if err != nil {
		t.Fatalf("UploadStream() error = %v", err)
	}
	if method != http.MethodPut || path != "/bucket/documents/file%20name.pdf" || body != "data" {
		t.Fatalf("request = method %q path %q body %q", method, path, body)
	}
	if !strings.Contains(authHeader, "AWS4-HMAC-SHA256") || !strings.Contains(authHeader, "SignedHeaders=content-type;host;x-amz-acl;x-amz-content-sha256;x-amz-date") {
		t.Fatalf("Authorization = %q", authHeader)
	}
	if object.URL != "https://cdn.example.com/base/documents/file%20name.pdf" || object.Size != 4 {
		t.Fatalf("object = %#v", object)
	}
}

func TestS3StorageReturnsPresignedURLForPrivateBucket(t *testing.T) {
	storage, err := NewS3Storage(S3Config{
		EndpointURL:   "https://s3.example.com",
		AccessKey:     "access",
		SecretKey:     "secret",
		BucketName:    "bucket",
		Region:        "us-east-1",
		PrivateBucket: true,
		URLExpire:     time.Hour,
	}, http.DefaultClient)
	if err != nil {
		t.Fatalf("NewS3Storage() error = %v", err)
	}
	storage.now = func() time.Time { return time.Date(2026, time.May, 6, 10, 0, 0, 0, time.UTC) }

	url := storage.downloadURL("images/file.png")
	if !strings.HasPrefix(url, "https://s3.example.com/bucket/images/file.png?") {
		t.Fatalf("url = %q", url)
	}
	for _, fragment := range []string{"X-Amz-Algorithm=AWS4-HMAC-SHA256", "X-Amz-Expires=3600", "X-Amz-SignedHeaders=host", "X-Amz-Signature="} {
		if !strings.Contains(url, fragment) {
			t.Fatalf("url %q missing %q", url, fragment)
		}
	}
}

func TestS3StorageRejectsMissingConfig(t *testing.T) {
	if _, err := NewS3Storage(S3Config{}, http.DefaultClient); err == nil {
		t.Fatal("NewS3Storage(empty) error = nil, want error")
	}
}

func TestS3StorageRejectsAmbiguousEndpointURLs(t *testing.T) {
	cases := []S3Config{
		{EndpointURL: "ftp://s3.example.com"},
		{EndpointURL: "https://user:pass@s3.example.com"},
		{EndpointURL: "https://s3.example.com?target=internal"},
		{EndpointURL: "https://s3.example.com#fragment"},
		{EndpointURL: "https://s3.example.com", PublicURLBase: "https://user:pass@cdn.example.com/base"},
		{EndpointURL: "https://s3.example.com", PublicURLBase: "https://cdn.example.com/base?token=static"},
	}
	for _, cfg := range cases {
		t.Run(cfg.EndpointURL+"|"+cfg.PublicURLBase, func(t *testing.T) {
			cfg.AccessKey = "access"
			cfg.SecretKey = "secret"
			cfg.BucketName = "bucket"
			cfg.Region = "us-east-1"
			if _, err := NewS3Storage(cfg, http.DefaultClient); err == nil {
				t.Fatal("NewS3Storage() error = nil, want invalid URL error")
			}
		})
	}
}
