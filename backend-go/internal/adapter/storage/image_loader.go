package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	answerocrapp "mathstudy/backend-go/internal/application/answerocr"
)

const (
	maxOCRImageDimension = 16_384
	maxOCRImagePixels    = 36_000_000
)

type downloadReference struct {
	base        *url.URL
	allowQuery  bool
	downloadURL func(string) string
	storageName string
	httpClient  *http.Client
}

func (r downloadReference) load(ctx context.Context, reference string) (answerocrapp.Image, error) {
	key, err := imageKeyFromRemoteReference(reference, r.base, r.allowQuery)
	if err != nil {
		return answerocrapp.Image{}, err
	}
	requestURL := r.downloadURL(key)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return answerocrapp.Image{}, fmt.Errorf("%w: build %s image request", answerocrapp.ErrUnavailable, r.storageName)
	}
	response, err := withoutRedirects(r.httpClient).Do(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return answerocrapp.Image{}, answerocrapp.ErrTimeout
		}
		if errors.Is(err, context.Canceled) {
			return answerocrapp.Image{}, context.Canceled
		}
		return answerocrapp.Image{}, fmt.Errorf("%w: download %s image", answerocrapp.ErrUnavailable, r.storageName)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return answerocrapp.Image{}, answerocrapp.ErrInvalidImage
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return answerocrapp.Image{}, fmt.Errorf("%w: download %s image status %d", answerocrapp.ErrUnavailable, r.storageName, response.StatusCode)
	}
	if response.ContentLength > answerocrapp.MaxImageSize {
		return answerocrapp.Image{}, answerocrapp.ErrInvalidImage
	}
	return readValidatedImage(response.Body, response.Header.Get("Content-Type"))
}

func imageKeyFromRemoteReference(reference string, base *url.URL, allowQuery bool) (string, error) {
	if base == nil {
		return "", answerocrapp.ErrUnavailable
	}
	reference = strings.TrimSpace(reference)
	parsed, err := url.Parse(reference)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return "", answerocrapp.ErrInvalidImage
	}
	if parsed.Scheme != base.Scheme || parsed.Host != base.Host {
		return "", answerocrapp.ErrInvalidImage
	}
	if !allowQuery && parsed.RawQuery != "" {
		return "", answerocrapp.ErrInvalidImage
	}

	basePath := strings.TrimRight(base.EscapedPath(), "/")
	prefix := basePath + "/"
	escapedPath := parsed.EscapedPath()
	if !strings.HasPrefix(escapedPath, prefix) {
		return "", answerocrapp.ErrInvalidImage
	}
	escapedKey := strings.TrimPrefix(escapedPath, prefix)
	key, err := url.PathUnescape(escapedKey)
	if err != nil || key == "" {
		return "", answerocrapp.ErrInvalidImage
	}
	cleanKey, err := cleanObjectKey(key)
	if err != nil || cleanKey != key || !strings.HasPrefix(cleanKey, "images/") {
		return "", answerocrapp.ErrInvalidImage
	}
	if escapedPath != prefix+awsEncode(cleanKey, false) {
		return "", answerocrapp.ErrInvalidImage
	}
	return cleanKey, nil
}

func readValidatedImage(reader io.Reader, declaredContentType string) (answerocrapp.Image, error) {
	if reader == nil {
		return answerocrapp.Image{}, answerocrapp.ErrInvalidImage
	}
	data, err := io.ReadAll(io.LimitReader(reader, answerocrapp.MaxImageSize+1))
	if err != nil {
		return answerocrapp.Image{}, fmt.Errorf("%w: read image", answerocrapp.ErrUnavailable)
	}
	if len(data) == 0 || len(data) > answerocrapp.MaxImageSize {
		return answerocrapp.Image{}, answerocrapp.ErrInvalidImage
	}
	actualType := strings.ToLower(strings.TrimSpace(http.DetectContentType(data)))
	if !supportedOCRImageType(actualType) {
		return answerocrapp.Image{}, answerocrapp.ErrInvalidImage
	}
	if declaredContentType != "" {
		declaredType, _, err := mime.ParseMediaType(declaredContentType)
		if err != nil || strings.ToLower(strings.TrimSpace(declaredType)) != actualType {
			return answerocrapp.Image{}, answerocrapp.ErrInvalidImage
		}
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || imageFormatMIME(format) != actualType || !validOCRImageDimensions(config.Width, config.Height) {
		return answerocrapp.Image{}, answerocrapp.ErrInvalidImage
	}
	decoded, decodedFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil || imageFormatMIME(decodedFormat) != actualType {
		return answerocrapp.Image{}, answerocrapp.ErrInvalidImage
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != config.Width || bounds.Dy() != config.Height || !validOCRImageDimensions(bounds.Dx(), bounds.Dy()) {
		return answerocrapp.Image{}, answerocrapp.ErrInvalidImage
	}
	return answerocrapp.Image{Data: data, MIMEType: actualType}, nil
}

func supportedOCRImageType(contentType string) bool {
	switch contentType {
	case "image/gif", "image/jpeg", "image/png":
		return true
	default:
		return false
	}
}

func imageFormatMIME(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "gif":
		return "image/gif"
	case "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	default:
		return ""
	}
}

func validOCRImageDimensions(width int, height int) bool {
	return width > 0 && height > 0 &&
		width <= maxOCRImageDimension && height <= maxOCRImageDimension &&
		int64(width)*int64(height) <= maxOCRImagePixels
}

func withoutRedirects(client *http.Client) *http.Client {
	if client == nil {
		client = defaultTimeout(nil)
	}
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &copy
}
