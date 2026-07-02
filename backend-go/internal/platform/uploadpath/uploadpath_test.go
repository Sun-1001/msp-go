package uploadpath

import (
	"strings"
	"testing"
)

func TestIsImagePath(t *testing.T) {
	valid := []string{
		"/uploads/images/file.png",
		" /uploads/images/nested/file.webp ",
	}
	for _, value := range valid {
		t.Run("valid "+value, func(t *testing.T) {
			if !IsImagePath(value) {
				t.Fatalf("IsImagePath(%q) = false, want true", value)
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
			if IsImagePath(value) {
				t.Fatalf("IsImagePath(%q) = true, want false", value)
			}
		})
	}
}

func TestIsResourcePath(t *testing.T) {
	valid := []string{
		"/uploads/documents/file.pdf",
		" /uploads/videos/nested/file.mp4 ",
	}
	for _, value := range valid {
		t.Run("valid "+value, func(t *testing.T) {
			if !IsResourcePath(value) {
				t.Fatalf("IsResourcePath(%q) = false, want true", value)
			}
		})
	}

	invalid := []string{
		"/uploads/images/file.png",
		"/uploads/documents/../secret.pdf",
		"/uploads/videos/file.mp4?token=1",
		"/uploads/videos/file.mp4#fragment",
		`/uploads/documents\file.pdf`,
		"/uploads/documents/%2e%2e/file.pdf",
	}
	for _, value := range invalid {
		t.Run("invalid "+value, func(t *testing.T) {
			if IsResourcePath(value) {
				t.Fatalf("IsResourcePath(%q) = true, want false", value)
			}
		})
	}
}

func TestCleanServablePath(t *testing.T) {
	valid := map[string]string{
		"images/file.txt":     "images/file.txt",
		"/documents/file.pdf": "documents/file.pdf",
		"videos/nested/a.mp4": "videos/nested/a.mp4",
	}
	for value, want := range valid {
		t.Run("valid "+value, func(t *testing.T) {
			got, ok := CleanServablePath(value)
			if !ok || got != want {
				t.Fatalf("CleanServablePath(%q) = %q, %v; want %q, true", value, got, ok, want)
			}
		})
	}

	invalid := []string{
		"",
		"images/",
		"documents",
		"secret.txt",
		"images/../secret.txt",
		"images/%2e%2e/secret.txt",
		`images\file.txt`,
		"images//file.txt",
		"images/./file.txt",
		"images/" + strings.Repeat("a", maxLocalURLLength),
	}
	for _, value := range invalid {
		t.Run("invalid "+value, func(t *testing.T) {
			if got, ok := CleanServablePath(value); ok {
				t.Fatalf("CleanServablePath(%q) = %q, true; want false", value, got)
			}
		})
	}
}

func TestIsDocumentKey(t *testing.T) {
	if !IsDocumentKey("documents/file.pdf") {
		t.Fatal("IsDocumentKey(documents/file.pdf) = false, want true")
	}
	if IsDocumentKey("images/file.png") || IsDocumentKey("videos/file.mp4") || IsDocumentKey("documents/../secret.pdf") || IsDocumentKey("documents/") {
		t.Fatal("IsDocumentKey accepted a non-document key")
	}
}
