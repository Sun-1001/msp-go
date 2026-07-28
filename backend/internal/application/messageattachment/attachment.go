package messageattachment

import (
	"encoding/json"
	"errors"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	uploadapp "mathstudy/backend/internal/application/upload"
)

const (
	MaxAttachments       = 5
	maxURLRunes          = 2048
	maxNameRunes         = 255
	maxContentTypeRunes  = 255
	maxImageSizeBytes    = 10 * 1024 * 1024
	maxDocumentSizeBytes = 50 * 1024 * 1024
)

var ErrInvalid = errors.New("invalid message attachment")

// Attachment is the shared message-center attachment contract.
type Attachment struct {
	URL         string `json:"url"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// Normalize validates client-provided attachment metadata and removes exact URL duplicates.
func Normalize(values []Attachment) ([]Attachment, error) {
	if len(values) > MaxAttachments {
		return nil, ErrInvalid
	}
	result := make([]Attachment, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value.URL = strings.TrimSpace(value.URL)
		value.Name = strings.TrimSpace(value.Name)
		value.Kind = strings.ToLower(strings.TrimSpace(value.Kind))
		value.ContentType = strings.ToLower(strings.TrimSpace(value.ContentType))
		if !validAttachment(value) {
			return nil, ErrInvalid
		}
		if _, exists := seen[value.URL]; exists {
			continue
		}
		seen[value.URL] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

// Encode serializes already-normalized attachments for JSONB columns.
func Encode(values []Attachment) ([]byte, error) {
	if values == nil {
		values = []Attachment{}
	}
	return json.Marshal(values)
}

// Decode supports both the structured contract and legacy notice string URLs.
func Decode(raw []byte) ([]Attachment, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []Attachment{}, nil
	}
	var structured []Attachment
	if err := json.Unmarshal(raw, &structured); err == nil {
		if structured == nil {
			return []Attachment{}, nil
		}
		return structured, nil
	}
	var legacy []string
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return nil, err
	}
	result := make([]Attachment, 0, len(legacy))
	for _, rawURL := range legacy {
		url := strings.TrimSpace(rawURL)
		if url == "" {
			continue
		}
		kind := "file"
		if uploadapp.IsSafeImagePath(url) {
			kind = "image"
		}
		name := path.Base(url)
		if name == "." || name == "/" || name == "" {
			name = "附件"
		}
		result = append(result, Attachment{URL: url, Name: name, Kind: kind})
	}
	return result, nil
}

func validAttachment(value Attachment) bool {
	if value.URL == "" || value.Name == "" || value.ContentType == "" || value.Size <= 0 ||
		utf8.RuneCountInString(value.URL) > maxURLRunes ||
		utf8.RuneCountInString(value.Name) > maxNameRunes ||
		utf8.RuneCountInString(value.ContentType) > maxContentTypeRunes ||
		containsUnsafeText(value.URL) || containsUnsafeFilename(value.Name) || containsUnsafeText(value.ContentType) {
		return false
	}
	switch value.Kind {
	case "image":
		return value.Size <= maxImageSizeBytes && uploadapp.IsSafeImagePath(value.URL) &&
			uploadapp.IsAllowedImageContentType(value.ContentType)
	case "file":
		return value.Size <= maxDocumentSizeBytes && uploadapp.IsSafeDocumentPath(value.URL) &&
			uploadapp.IsAllowedDocumentContentType(value.ContentType)
	default:
		return false
	}
}

func containsUnsafeText(value string) bool {
	return !utf8.ValidString(value) || strings.ContainsRune(value, '\x00')
}

func containsUnsafeFilename(value string) bool {
	if containsUnsafeText(value) || strings.ContainsAny(value, "/\\") {
		return true
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
