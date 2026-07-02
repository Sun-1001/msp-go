package storage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestQiniuStorageUploadsMultipartData(t *testing.T) {
	var fields url.Values
	var fileData string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm() error = %v", err)
		}
		fields = url.Values(r.MultipartForm.Value)
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile() error = %v", err)
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		fileData = string(data)
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"key":"images/file.png"}`)),
			Request:    r,
		}, nil
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
	storage.now = func() time.Time { return time.Date(2026, time.May, 6, 10, 0, 0, 0, time.UTC) }

	object, err := storage.UploadStream(context.Background(), strings.NewReader("data"), "images/file.png", "image/png", 4)
	if err != nil {
		t.Fatalf("UploadStream() error = %v", err)
	}
	if fields.Get("key") != "images/file.png" || fields.Get("token") == "" || fileData != "data" {
		t.Fatalf("fields = %#v fileData = %q", fields, fileData)
	}
	if object.URL != "https://cdn.example.com/images/file.png" || object.Size != 4 {
		t.Fatalf("object = %#v", object)
	}
}

func TestQiniuStorageReturnsPrivateDownloadURL(t *testing.T) {
	storage, err := NewQiniuStorage(QiniuConfig{
		AccessKey:     "access",
		SecretKey:     "secret",
		BucketName:    "bucket",
		Domain:        "https://cdn.example.com",
		PrivateBucket: true,
		URLExpire:     time.Hour,
		UploadURL:     "https://upload.qiniup.com",
	}, http.DefaultClient)
	if err != nil {
		t.Fatalf("NewQiniuStorage() error = %v", err)
	}
	storage.now = func() time.Time { return time.Date(2026, time.May, 6, 10, 0, 0, 0, time.UTC) }

	url := storage.downloadURL("documents/file.pdf")
	if !strings.HasPrefix(url, "https://cdn.example.com/documents/file.pdf?e=") || !strings.Contains(url, "&token=access:") {
		t.Fatalf("url = %q", url)
	}
}

func TestQiniuStoragePreservesDownloadDomainBasePath(t *testing.T) {
	storage, err := NewQiniuStorage(QiniuConfig{
		AccessKey:  "access",
		SecretKey:  "secret",
		BucketName: "bucket",
		Domain:     "https://cdn.example.com/base/",
		UploadURL:  "https://upload.qiniup.com",
	}, http.DefaultClient)
	if err != nil {
		t.Fatalf("NewQiniuStorage() error = %v", err)
	}

	url := storage.downloadURL("documents/file name.pdf")
	if url != "https://cdn.example.com/base/documents/file%20name.pdf" {
		t.Fatalf("url = %q", url)
	}
}

func TestQiniuStorageRejectsMissingConfig(t *testing.T) {
	if _, err := NewQiniuStorage(QiniuConfig{}, http.DefaultClient); err == nil {
		t.Fatal("NewQiniuStorage(empty) error = nil, want error")
	}
}

func TestQiniuStorageRejectsAmbiguousDownloadDomain(t *testing.T) {
	cases := []string{
		"ftp://cdn.example.com",
		"https://user:pass@cdn.example.com",
		"https://cdn.example.com/base?token=static",
		"https://cdn.example.com/base#fragment",
	}
	for _, domain := range cases {
		t.Run(domain, func(t *testing.T) {
			_, err := NewQiniuStorage(QiniuConfig{
				AccessKey:  "access",
				SecretKey:  "secret",
				BucketName: "bucket",
				Domain:     domain,
				UploadURL:  "https://upload.qiniup.com",
			}, http.DefaultClient)
			if err == nil {
				t.Fatal("NewQiniuStorage() error = nil, want invalid domain error")
			}
		})
	}
}

func TestQiniuStorageRejectsUnsafeUploadURL(t *testing.T) {
	cases := []string{
		"http://upload.example.com",
		"https://127.0.0.1:9000",
		"https://10.0.0.8/upload",
		"https://upload.example.com?bucket=internal",
	}
	for _, uploadURL := range cases {
		t.Run(uploadURL, func(t *testing.T) {
			_, err := NewQiniuStorage(QiniuConfig{
				AccessKey:  "access",
				SecretKey:  "secret",
				BucketName: "bucket",
				Domain:     "https://cdn.example.com",
				UploadURL:  uploadURL,
			}, http.DefaultClient)
			if err == nil {
				t.Fatal("NewQiniuStorage() error = nil, want error")
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
