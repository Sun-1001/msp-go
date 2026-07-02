package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	uploadapp "mathstudy/backend-go/internal/application/upload"
	"mathstudy/backend-go/internal/platform/outbound"
)

// QiniuConfig contains Qiniu Kodo object storage settings.
type QiniuConfig struct {
	AccessKey     string
	SecretKey     string
	BucketName    string
	Domain        string
	PrivateBucket bool
	URLExpire     time.Duration
	UploadURL     string
}

// QiniuStorage uploads files to Qiniu Kodo using server-side upload tokens.
type QiniuStorage struct {
	cfg    QiniuConfig
	client *http.Client
	now    func() time.Time
}

// NewQiniuStorage creates a Qiniu upload adapter.
func NewQiniuStorage(cfg QiniuConfig, client *http.Client) (*QiniuStorage, error) {
	if cfg.URLExpire <= 0 {
		cfg.URLExpire = time.Hour
	}
	if strings.TrimSpace(cfg.UploadURL) == "" {
		cfg.UploadURL = "https://upload.qiniup.com"
	}
	missing := make([]string, 0)
	for key, value := range map[string]string{
		"QINIU_ACCESS_KEY":  cfg.AccessKey,
		"QINIU_SECRET_KEY":  cfg.SecretKey,
		"QINIU_BUCKET_NAME": cfg.BucketName,
		"QINIU_DOMAIN":      cfg.Domain,
		"QINIU_UPLOAD_URL":  cfg.UploadURL,
	} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("Qiniu storage config missing: %s", strings.Join(missing, ", "))
	}
	domain, err := normalizeStorageBaseURL("QINIU_DOMAIN", cfg.Domain)
	if err != nil {
		return nil, err
	}
	cfg.Domain = domain.String()
	uploadURL, err := outbound.NormalizePublicHTTPSBaseURL(cfg.UploadURL)
	if err != nil {
		return nil, fmt.Errorf("QINIU_UPLOAD_URL %w", err)
	}
	cfg.UploadURL = uploadURL
	return &QiniuStorage{
		cfg:    cfg,
		client: qiniuHTTPClient(client),
		now:    func() time.Time { return time.Now().UTC() },
	}, nil
}

// UploadStream uploads a single object and returns its public or private URL.
func (s *QiniuStorage) UploadStream(ctx context.Context, reader io.Reader, key string, contentType string, size int64) (uploadapp.StoredObject, error) {
	cleanKey, err := cleanObjectKey(key)
	if err != nil {
		return uploadapp.StoredObject{}, err
	}
	bodyReader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	copyErr := make(chan error, 1)
	go func() {
		copyErr <- writeQiniuMultipart(multipartWriter, writer, reader, cleanKey, contentType, s.uploadToken(cleanKey))
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.UploadURL, bodyReader)
	if err != nil {
		_ = bodyReader.Close()
		<-copyErr
		return uploadapp.StoredObject{}, err
	}
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	response, err := s.client.Do(req)
	if err != nil {
		_ = bodyReader.Close()
		if writeErr := <-copyErr; writeErr != nil && !errors.Is(writeErr, io.ErrClosedPipe) {
			return uploadapp.StoredObject{}, writeErr
		}
		return uploadapp.StoredObject{}, err
	}
	defer response.Body.Close()
	if writeErr := <-copyErr; writeErr != nil {
		return uploadapp.StoredObject{}, writeErr
	}
	if response.StatusCode != http.StatusOK {
		message := readErrorBody(response.Body)
		if message == "" {
			message = response.Status
		}
		return uploadapp.StoredObject{}, fmt.Errorf("Qiniu upload failed: %s", message)
	}
	return uploadapp.StoredObject{
		Key:         cleanKey,
		URL:         s.downloadURL(cleanKey),
		Size:        size,
		ContentType: contentType,
	}, nil
}

func writeQiniuMultipart(writer *multipart.Writer, pipe *io.PipeWriter, reader io.Reader, key string, contentType string, token string) error {
	var err error
	defer func() {
		if err != nil {
			_ = pipe.CloseWithError(err)
			return
		}
		_ = pipe.Close()
	}()
	if err = writer.WriteField("token", token); err != nil {
		return err
	}
	if err = writer.WriteField("key", key); err != nil {
		return err
	}
	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="`+escapeMultipartFilename(path.Base(key))+`"`)
	partHeader.Set("Content-Type", contentType)
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return err
	}
	if _, err = io.Copy(part, reader); err != nil {
		return err
	}
	return writer.Close()
}

func (s *QiniuStorage) uploadToken(key string) string {
	policy := map[string]any{
		"scope":    s.cfg.BucketName + ":" + key,
		"deadline": s.now().Add(time.Hour).Unix(),
	}
	data, _ := json.Marshal(policy)
	encodedPolicy := qiniuBase64(data)
	signature := qiniuHMACSHA1(s.cfg.SecretKey, encodedPolicy)
	return s.cfg.AccessKey + ":" + signature + ":" + encodedPolicy
}

func (s *QiniuStorage) downloadURL(key string) string {
	base := strings.TrimRight(s.cfg.Domain, "/") + "/" + awsEncode(key, false)
	if !s.cfg.PrivateBucket {
		return base
	}
	deadline := strconv.FormatInt(s.now().Add(s.cfg.URLExpire).Unix(), 10)
	separator := "?"
	if strings.Contains(base, "?") {
		separator = "&"
	}
	urlWithDeadline := base + separator + "e=" + deadline
	token := s.cfg.AccessKey + ":" + qiniuHMACSHA1Raw(s.cfg.SecretKey, []byte(urlWithDeadline))
	return urlWithDeadline + "&token=" + token
}

func qiniuHMACSHA1(secret string, encodedPolicy string) string {
	return qiniuHMACSHA1Raw(secret, []byte(encodedPolicy))
}

func qiniuHMACSHA1Raw(secret string, data []byte) string {
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write(data)
	return qiniuBase64(mac.Sum(nil))
}

func qiniuBase64(data []byte) string {
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(data)
}

func escapeMultipartFilename(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", `"`, "\\\"")
	return replacer.Replace(value)
}

func qiniuHTTPClient(client *http.Client) *http.Client {
	if client != nil {
		return defaultTimeout(client)
	}
	return outbound.NewPublicHTTPSClient(5 * time.Minute)
}
