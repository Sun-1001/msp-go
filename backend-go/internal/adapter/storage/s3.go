package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	uploadapp "mathstudy/backend-go/internal/application/upload"
)

// S3Config contains the S3-compatible object storage settings.
type S3Config struct {
	EndpointURL   string
	AccessKey     string
	SecretKey     string
	BucketName    string
	Region        string
	PublicURLBase string
	PrivateBucket bool
	URLExpire     time.Duration
}

// S3Storage uploads files to an S3-compatible path-style endpoint.
type S3Storage struct {
	cfg      S3Config
	endpoint *url.URL
	client   *http.Client
	now      func() time.Time
}

// NewS3Storage creates an S3-compatible upload adapter using AWS Signature V4.
func NewS3Storage(cfg S3Config, client *http.Client) (*S3Storage, error) {
	if strings.TrimSpace(cfg.Region) == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.URLExpire <= 0 {
		cfg.URLExpire = time.Hour
	}
	missing := make([]string, 0)
	for key, value := range map[string]string{
		"S3_ENDPOINT_URL": cfg.EndpointURL,
		"S3_ACCESS_KEY":   cfg.AccessKey,
		"S3_SECRET_KEY":   cfg.SecretKey,
		"S3_BUCKET_NAME":  cfg.BucketName,
		"S3_REGION":       cfg.Region,
	} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("S3 storage config missing: %s", strings.Join(missing, ", "))
	}
	endpoint, err := normalizeStorageBaseURL("S3_ENDPOINT_URL", cfg.EndpointURL)
	if err != nil {
		return nil, err
	}
	cfg.EndpointURL = endpoint.String()
	if strings.TrimSpace(cfg.PublicURLBase) != "" {
		publicBase, err := normalizeStorageBaseURL("S3_PUBLIC_URL_BASE", cfg.PublicURLBase)
		if err != nil {
			return nil, err
		}
		cfg.PublicURLBase = publicBase.String()
	}
	return &S3Storage{
		cfg:      cfg,
		endpoint: endpoint,
		client:   defaultTimeout(client),
		now:      func() time.Time { return time.Now().UTC() },
	}, nil
}

func normalizeStorageBaseURL(name string, value string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(value), "/"))
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("%s must include scheme and host", name)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("%s must use http or https", name)
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("%s must not include username or password", name)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("%s must not include query or fragment", name)
	}
	return parsed, nil
}

// UploadStream uploads a single object and returns its public or presigned URL.
func (s *S3Storage) UploadStream(ctx context.Context, reader io.Reader, key string, contentType string, size int64) (uploadapp.StoredObject, error) {
	cleanKey, err := cleanObjectKey(key)
	if err != nil {
		return uploadapp.StoredObject{}, err
	}
	objectURL, canonicalURI := s.objectURL(cleanKey)
	payloadHash := "UNSIGNED-PAYLOAD"
	now := s.now().UTC()
	headers := map[string]string{
		"content-type":         contentType,
		"host":                 s.endpoint.Host,
		"x-amz-acl":            s.acl(),
		"x-amz-content-sha256": payloadHash,
		"x-amz-date":           now.Format("20060102T150405Z"),
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, objectURL, reader)
	if err != nil {
		return uploadapp.StoredObject{}, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Amz-Acl", headers["x-amz-acl"])
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	req.Header.Set("X-Amz-Date", headers["x-amz-date"])
	req.Header.Set("Authorization", s.authorization(http.MethodPut, canonicalURI, "", headers, payloadHash, now))
	if size > 0 {
		req.ContentLength = size
	}

	response, err := s.client.Do(req)
	if err != nil {
		return uploadapp.StoredObject{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message := readErrorBody(response.Body)
		if message == "" {
			message = response.Status
		}
		return uploadapp.StoredObject{}, fmt.Errorf("S3 upload failed: %s", message)
	}
	return uploadapp.StoredObject{
		Key:         cleanKey,
		URL:         s.downloadURL(cleanKey),
		Size:        size,
		ContentType: contentType,
	}, nil
}

func (s *S3Storage) acl() string {
	if s.cfg.PrivateBucket {
		return "private"
	}
	return "public-read"
}

func (s *S3Storage) objectURL(key string) (string, string) {
	path := strings.TrimRight(s.endpoint.EscapedPath(), "/")
	path += "/" + awsEncode(s.cfg.BucketName, true) + "/" + awsEncode(key, false)
	return s.endpoint.Scheme + "://" + s.endpoint.Host + path, path
}

func (s *S3Storage) downloadURL(key string) string {
	if !s.cfg.PrivateBucket {
		base := strings.TrimRight(s.cfg.PublicURLBase, "/")
		if base == "" {
			base = strings.TrimRight(s.cfg.EndpointURL, "/") + "/" + awsEncode(s.cfg.BucketName, true)
		}
		return base + "/" + awsEncode(key, false)
	}
	now := s.now().UTC()
	_, canonicalURI := s.objectURL(key)
	scope := s.credentialScope(now)
	params := map[string]string{
		"X-Amz-Algorithm":     "AWS4-HMAC-SHA256",
		"X-Amz-Credential":    s.cfg.AccessKey + "/" + scope,
		"X-Amz-Date":          now.Format("20060102T150405Z"),
		"X-Amz-Expires":       strconv.FormatInt(int64(s.cfg.URLExpire/time.Second), 10),
		"X-Amz-SignedHeaders": "host",
	}
	canonicalQuery := canonicalQueryString(params)
	headers := map[string]string{"host": s.endpoint.Host}
	signature := s.signature(http.MethodGet, canonicalURI, canonicalQuery, headers, "UNSIGNED-PAYLOAD", now)
	objectURL, _ := s.objectURL(key)
	return objectURL + "?" + canonicalQuery + "&X-Amz-Signature=" + signature
}

func (s *S3Storage) authorization(method string, canonicalURI string, canonicalQuery string, headers map[string]string, payloadHash string, now time.Time) string {
	_, signedHeaders := canonicalHeaders(headers)
	return "AWS4-HMAC-SHA256 " +
		"Credential=" + s.cfg.AccessKey + "/" + s.credentialScope(now) + ", " +
		"SignedHeaders=" + signedHeaders + ", " +
		"Signature=" + s.signature(method, canonicalURI, canonicalQuery, headers, payloadHash, now)
}

func (s *S3Storage) signature(method string, canonicalURI string, canonicalQuery string, headers map[string]string, payloadHash string, now time.Time) string {
	canonicalHeaderString, signedHeaders := canonicalHeaders(headers)
	canonicalRequest := strings.Join([]string{
		method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaderString,
		signedHeaders,
		payloadHash,
	}, "\n")
	requestHash := sha256Hex([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		now.Format("20060102T150405Z"),
		s.credentialScope(now),
		requestHash,
	}, "\n")
	signingKey := s.signingKey(now)
	return hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))
}

func (s *S3Storage) signingKey(now time.Time) []byte {
	dateKey := hmacSHA256([]byte("AWS4"+s.cfg.SecretKey), []byte(now.Format("20060102")))
	regionKey := hmacSHA256(dateKey, []byte(s.cfg.Region))
	serviceKey := hmacSHA256(regionKey, []byte("s3"))
	return hmacSHA256(serviceKey, []byte("aws4_request"))
}

func (s *S3Storage) credentialScope(now time.Time) string {
	return now.Format("20060102") + "/" + s.cfg.Region + "/s3/aws4_request"
}

func canonicalHeaders(headers map[string]string) (string, string) {
	keys := make([]string, 0, len(headers))
	normalized := make(map[string]string, len(headers))
	for key, value := range headers {
		lower := strings.ToLower(strings.TrimSpace(key))
		if lower == "" {
			continue
		}
		keys = append(keys, lower)
		normalized[lower] = strings.Join(strings.Fields(value), " ")
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteByte(':')
		builder.WriteString(normalized[key])
		builder.WriteByte('\n')
	}
	return builder.String(), strings.Join(keys, ";")
}

func canonicalQueryString(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, awsEncode(key, true)+"="+awsEncode(params[key], true))
	}
	return strings.Join(parts, "&")
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}

func awsEncode(value string, encodeSlash bool) string {
	const hexChars = "0123456789ABCDEF"
	var builder strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' {
			builder.WriteByte(c)
			continue
		}
		if c == '/' && !encodeSlash {
			builder.WriteByte('/')
			continue
		}
		builder.WriteByte('%')
		builder.WriteByte(hexChars[c>>4])
		builder.WriteByte(hexChars[c&15])
	}
	return builder.String()
}

func readErrorBody(reader io.Reader) string {
	data, _ := io.ReadAll(io.LimitReader(reader, 4096))
	return strings.TrimSpace(string(data))
}
