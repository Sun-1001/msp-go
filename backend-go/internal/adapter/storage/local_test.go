package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStorageWritesFileAndReturnsUploadsURL(t *testing.T) {
	root := t.TempDir()
	storage := NewLocalStorage(root)

	object, err := storage.UploadStream(context.Background(), strings.NewReader("data"), "images/file.png", "image/png", 4)
	if err != nil {
		t.Fatalf("UploadStream() error = %v", err)
	}
	if object.Key != "images/file.png" || object.URL != "/uploads/images/file.png" || object.Size != 4 || object.ContentType != "image/png" {
		t.Fatalf("object = %#v", object)
	}
	data, err := os.ReadFile(filepath.Join(root, "images", "file.png"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "data" {
		t.Fatalf("file data = %q", data)
	}
}

func TestLocalStorageRejectsPathTraversal(t *testing.T) {
	storage := NewLocalStorage(t.TempDir())
	if _, err := storage.UploadStream(context.Background(), strings.NewReader("data"), "../outside.txt", "text/plain", 4); err == nil {
		t.Fatal("UploadStream(path traversal) error = nil, want error")
	}
	if _, err := storage.UploadStream(context.Background(), strings.NewReader("data"), `images\..\outside.txt`, "text/plain", 4); err == nil {
		t.Fatal("UploadStream(backslash traversal) error = nil, want error")
	}
}

func TestCleanObjectKeyNormalizesRedundantSeparators(t *testing.T) {
	key, err := cleanObjectKey("/images//nested/./file.png")
	if err != nil {
		t.Fatalf("cleanObjectKey() error = %v", err)
	}
	if key != "images/nested/file.png" {
		t.Fatalf("key = %q", key)
	}
}

func TestCleanObjectKeyRejectsAmbiguousKeys(t *testing.T) {
	cases := []string{
		"images/%2e%2e/file.png",
		"images/file.png?download=1",
		"images/file.png#fragment",
		"images/file:name.png",
		"images/\x00file.png",
		"C:/tmp/file.png",
		"avatars/file.png",
	}
	for _, value := range cases {
		t.Run(value, func(t *testing.T) {
			if _, err := cleanObjectKey(value); err == nil {
				t.Fatal("cleanObjectKey() error = nil, want error")
			}
		})
	}
}
