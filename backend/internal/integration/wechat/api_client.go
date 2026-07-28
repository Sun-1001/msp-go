package wechat

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"mathstudy/backend/internal/platform/outbound"
)

const (
	wechatAPIBaseURL           = "https://api.weixin.qq.com"
	stableTokenPath            = "/cgi-bin/stable_token"
	customMessagePath          = "/cgi-bin/message/custom/send"
	templateMessagePath        = "/cgi-bin/message/template/send"
	defaultAPITimeout          = 10 * time.Second
	maxAPITimeout              = 60 * time.Second
	maxAPIResponseBytes        = 64 << 10
	maxAppSecretBytes          = 512
	maxAccessTokenBytes        = 4096
	maxCustomerTextBytes       = 2048
	maxTemplateIDBytes         = 256
	maxTemplateFieldKeyBytes   = 64
	maxTemplateFieldValueBytes = 4096
	maxTemplateFields          = 32
	maxTemplatePayloadBytes    = 32 << 10
	accessTokenRefreshLeadTime = 5 * time.Minute
	minimumAccessTokenLockTTL  = 20 * time.Second
	accessTokenLockSafety      = 10 * time.Second
	accessTokenLockWaitGrace   = 2 * time.Second
	accessTokenLockPoll        = 50 * time.Millisecond
	localLockPoll              = 10 * time.Millisecond
	lockReleaseTimeout         = 2 * time.Second
	maxAccessTokenLifetime     = 24 * time.Hour
)

var (
	errAPIClientInvalidConfig  = errors.New("invalid WeChat API client configuration")
	errTokenCacheUnavailable   = errors.New("WeChat access-token cache is unavailable")
	errTokenRefreshUnavailable = errors.New("WeChat access-token refresh is unavailable")
)

// TokenStore coordinates one Official Account access token across processes.
type TokenStore interface {
	GetAccessToken(context.Context, string) (string, bool, error)
	SetAccessToken(context.Context, string, string, time.Duration) error
	DeleteAccessTokenIfMatch(context.Context, string, string) error
	AcquireAccessTokenLock(context.Context, string, string, time.Duration) (bool, error)
	ReleaseAccessTokenLock(context.Context, string, string) error
}

// APIError is a credential-free summary of a failed WeChat API operation.
type APIError struct {
	Operation  string
	Code       int
	HTTPStatus int
	Retryable  bool
}

func (e *APIError) Error() string {
	if e == nil {
		return "WeChat API request failed"
	}
	switch {
	case e.Code != 0 && e.HTTPStatus != 0:
		return fmt.Sprintf("WeChat API %s failed with HTTP %d and code %d", e.Operation, e.HTTPStatus, e.Code)
	case e.Code != 0:
		return fmt.Sprintf("WeChat API %s failed with code %d", e.Operation, e.Code)
	case e.HTTPStatus != 0:
		return fmt.Sprintf("WeChat API %s failed with HTTP %d", e.Operation, e.HTTPStatus)
	default:
		return "WeChat API " + e.Operation + " failed"
	}
}

// WechatProviderCode exposes the credential-free provider code for delivery retry policy.
func (e *APIError) WechatProviderCode() int {
	if e == nil {
		return 0
	}
	return e.Code
}

// WechatHTTPStatus exposes the provider HTTP status for delivery retry policy.
func (e *APIError) WechatHTTPStatus() int {
	if e == nil {
		return 0
	}
	return e.HTTPStatus
}

// WechatRetryable reports whether the API client classified the failure as transient.
func (e *APIError) WechatRetryable() bool {
	return e != nil && e.Retryable
}

// APIClient obtains stable access tokens and sends customer-service or template messages.
// baseURL and clock are fields so same-package temporary tests can replace them.
type APIClient struct {
	appID               string
	appSecret           string
	http                *http.Client
	tokens              TokenStore
	baseURL             string
	clock               func() time.Time
	accessTokenLockTTL  time.Duration
	accessTokenLockWait time.Duration
	refreshMu           sync.Mutex
}

// NewAPIClient creates a client for one Official Account.
func NewAPIClient(appID, appSecret string, httpClient *http.Client, tokens TokenStore) (*APIClient, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" || len(appID) > maxAppIDBytes ||
		strings.TrimSpace(appSecret) == "" || len(appSecret) > maxAppSecretBytes || tokens == nil {
		return nil, errAPIClientInvalidConfig
	}
	if httpClient == nil {
		httpClient = outbound.NewPublicHTTPSClient(defaultAPITimeout)
	} else {
		clone := *httpClient
		if clone.Timeout <= 0 {
			clone.Timeout = defaultAPITimeout
		}
		if clone.Timeout > maxAPITimeout {
			return nil, errAPIClientInvalidConfig
		}
		clone.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
		httpClient = &clone
	}
	lockTTL := httpClient.Timeout + accessTokenLockSafety
	if lockTTL < minimumAccessTokenLockTTL {
		lockTTL = minimumAccessTokenLockTTL
	}
	return &APIClient{
		appID:               appID,
		appSecret:           appSecret,
		http:                httpClient,
		tokens:              tokens,
		baseURL:             wechatAPIBaseURL,
		clock:               func() time.Time { return time.Now().UTC() },
		accessTokenLockTTL:  lockTTL,
		accessTokenLockWait: lockTTL + accessTokenLockWaitGrace,
	}, nil
}

// SendText sends one server-controlled customer-service text message. An
// invalid access token is refreshed and the send is retried exactly once.
func (c *APIClient) SendText(ctx context.Context, openID, content string) error {
	if c == nil || c.http == nil || c.tokens == nil {
		return errAPIClientInvalidConfig
	}
	if ctx == nil {
		return errors.New("WeChat send context is nil")
	}
	if !validBoundedText(openID, 1, maxUserNameBytes) ||
		!validBoundedText(content, 1, maxCustomerTextBytes) {
		return errors.New("invalid WeChat customer message")
	}

	return c.sendWithAccessTokenRetry(ctx, func(token string) error {
		return c.sendTextOnce(ctx, token, openID, content)
	})
}

// SendTemplate sends a server-controlled template message. Callers provide
// only template data fields; links, colors, and mini-program targets are not exposed.
func (c *APIClient) SendTemplate(ctx context.Context, openID, templateID string, data map[string]string) error {
	if c == nil || c.http == nil || c.tokens == nil {
		return errAPIClientInvalidConfig
	}
	if ctx == nil {
		return errors.New("WeChat send context is nil")
	}
	if !validBoundedText(openID, 1, maxUserNameBytes) ||
		!validTemplateIdentifier(templateID, maxTemplateIDBytes) ||
		len(data) == 0 || len(data) > maxTemplateFields {
		return errors.New("invalid WeChat template message")
	}
	fields := make(map[string]templateField, len(data))
	for key, value := range data {
		if !validTemplateIdentifier(key, maxTemplateFieldKeyBytes) ||
			!utf8.ValidString(value) || len(value) > maxTemplateFieldValueBytes {
			return errors.New("invalid WeChat template message")
		}
		fields[key] = templateField{Value: value}
	}
	payload, err := json.Marshal(templateMessageRequest{
		ToUser:     openID,
		TemplateID: templateID,
		Data:       fields,
	})
	if err != nil || len(payload) > maxTemplatePayloadBytes {
		return errors.New("invalid WeChat template message")
	}
	return c.sendWithAccessTokenRetry(ctx, func(token string) error {
		return c.sendTemplateOnce(ctx, token, payload)
	})
}

func (c *APIClient) sendWithAccessTokenRetry(ctx context.Context, send func(string) error) error {
	token, err := c.accessToken(ctx, "")
	if err != nil {
		return err
	}
	err = send(token)
	if !isInvalidAccessTokenError(err) {
		return err
	}
	if err := c.tokens.DeleteAccessTokenIfMatch(ctx, c.appID, token); err != nil {
		return errTokenCacheUnavailable
	}
	refreshed, err := c.accessToken(ctx, token)
	if err != nil {
		return err
	}
	return send(refreshed)
}

func (c *APIClient) accessToken(ctx context.Context, staleToken string) (string, error) {
	if token, found, err := c.cachedToken(ctx, staleToken); err != nil {
		return "", err
	} else if found {
		return token, nil
	}
	if err := c.lockLocalRefresh(ctx); err != nil {
		return "", err
	}
	defer c.refreshMu.Unlock()

	if token, found, err := c.cachedToken(ctx, staleToken); err != nil {
		return "", err
	} else if found {
		return token, nil
	}
	return c.refreshDistributed(ctx, staleToken)
}

func (c *APIClient) cachedToken(ctx context.Context, staleToken string) (string, bool, error) {
	token, found, err := c.tokens.GetAccessToken(ctx, c.appID)
	if err != nil {
		return "", false, errTokenCacheUnavailable
	}
	if !found || !validAccessToken(token) || token == staleToken {
		return "", false, nil
	}
	return token, true, nil
}

func (c *APIClient) refreshDistributed(ctx context.Context, staleToken string) (string, error) {
	owner, err := c.newLockOwner()
	if err != nil {
		return "", errTokenRefreshUnavailable
	}
	waitCtx, cancel := context.WithTimeout(ctx, c.accessTokenLockWait)
	defer cancel()

	for {
		if token, found, cacheErr := c.cachedToken(waitCtx, staleToken); cacheErr != nil {
			return "", cacheErr
		} else if found {
			return token, nil
		}
		acquired, lockErr := c.tokens.AcquireAccessTokenLock(waitCtx, c.appID, owner, c.accessTokenLockTTL)
		if lockErr != nil {
			return "", errTokenCacheUnavailable
		}
		if acquired {
			return c.refreshAsOwner(ctx, owner, staleToken)
		}
		if err := waitForContext(waitCtx, accessTokenLockPoll); err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "", errTokenRefreshUnavailable
		}
	}
}

func (c *APIClient) refreshAsOwner(ctx context.Context, owner, staleToken string) (string, error) {
	defer c.releaseRefreshLock(ctx, owner)
	if token, found, err := c.cachedToken(ctx, staleToken); err != nil {
		return "", err
	} else if found {
		return token, nil
	}

	token, expiresIn, err := c.requestStableToken(ctx, staleToken != "")
	if err != nil {
		return "", err
	}
	ttl, ok := accessTokenCacheTTL(expiresIn)
	if !ok {
		return "", &APIError{Operation: "obtain stable token"}
	}
	if err := c.tokens.SetAccessToken(ctx, c.appID, token, ttl); err != nil {
		return "", errTokenCacheUnavailable
	}
	return token, nil
}

func (c *APIClient) releaseRefreshLock(ctx context.Context, owner string) {
	releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), lockReleaseTimeout)
	defer cancel()
	_ = c.tokens.ReleaseAccessTokenLock(releaseCtx, c.appID, owner)
}

func (c *APIClient) lockLocalRefresh(ctx context.Context) error {
	for !c.refreshMu.TryLock() {
		if err := waitForContext(ctx, localLockPoll); err != nil {
			return err
		}
	}
	return nil
}

func (c *APIClient) newLockOwner() (string, error) {
	var random [16]byte
	if _, err := io.ReadFull(cryptorand.Reader, random[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d-%s", c.clock().UnixNano(), hex.EncodeToString(random[:])), nil
}

func (c *APIClient) requestStableToken(ctx context.Context, forceRefresh bool) (string, int64, error) {
	payload, err := json.Marshal(stableTokenRequest{
		GrantType:    "client_credential",
		AppID:        c.appID,
		Secret:       c.appSecret,
		ForceRefresh: forceRefresh,
	})
	if err != nil {
		return "", 0, &APIError{Operation: "encode stable token request"}
	}
	var response stableTokenResponse
	if err := c.postJSON(ctx, stableTokenPath, "", payload, &response, "obtain stable token"); err != nil {
		return "", 0, err
	}
	if response.ErrCode != 0 {
		return "", 0, newProviderError("obtain stable token", response.ErrCode, 0)
	}
	token := strings.TrimSpace(response.AccessToken)
	if !validAccessToken(token) || response.ExpiresIn <= 0 {
		return "", 0, &APIError{Operation: "obtain stable token"}
	}
	return token, response.ExpiresIn, nil
}

func (c *APIClient) sendTextOnce(ctx context.Context, token, openID, content string) error {
	payload, err := json.Marshal(customerTextRequest{
		ToUser:  openID,
		MsgType: "text",
		Text:    customerText{Content: content},
	})
	if err != nil {
		return &APIError{Operation: "encode customer message"}
	}
	var response providerResponse
	if err := c.postJSON(ctx, customMessagePath, token, payload, &response, "send customer message"); err != nil {
		return err
	}
	if response.ErrCode == nil {
		return &APIError{Operation: "send customer message"}
	}
	if *response.ErrCode != 0 {
		return newProviderError("send customer message", *response.ErrCode, 0)
	}
	return nil
}

func (c *APIClient) sendTemplateOnce(ctx context.Context, token string, payload []byte) error {
	var response providerResponse
	if err := c.postJSON(ctx, templateMessagePath, token, payload, &response, "send template message"); err != nil {
		return err
	}
	if response.ErrCode == nil {
		return &APIError{Operation: "send template message"}
	}
	if *response.ErrCode != 0 {
		return newProviderError("send template message", *response.ErrCode, 0)
	}
	return nil
}

func (c *APIClient) postJSON(ctx context.Context, path, accessToken string, payload []byte, target any, operation string) error {
	endpoint := strings.TrimRight(c.baseURL, "/") + path
	if accessToken != "" {
		endpoint += "?access_token=" + url.QueryEscape(accessToken)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return &APIError{Operation: operation}
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("Accept", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return &APIError{Operation: operation, Retryable: true}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxAPIResponseBytes+1))
	if err != nil || len(body) > maxAPIResponseBytes {
		return &APIError{Operation: operation, HTTPStatus: response.StatusCode, Retryable: response.StatusCode >= 500}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var provider providerResponse
		if json.Unmarshal(body, &provider) == nil && provider.ErrCode != nil && *provider.ErrCode != 0 {
			return newProviderError(operation, *provider.ErrCode, response.StatusCode)
		}
		return &APIError{
			Operation:  operation,
			HTTPStatus: response.StatusCode,
			Retryable:  response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500,
		}
	}
	if err := json.Unmarshal(body, target); err != nil {
		return &APIError{Operation: operation, HTTPStatus: response.StatusCode}
	}
	return nil
}

type stableTokenRequest struct {
	GrantType    string `json:"grant_type"`
	AppID        string `json:"appid"`
	Secret       string `json:"secret"`
	ForceRefresh bool   `json:"force_refresh"`
}

type stableTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
}

type providerResponse struct {
	ErrCode *int `json:"errcode"`
}

type customerTextRequest struct {
	ToUser  string       `json:"touser"`
	MsgType string       `json:"msgtype"`
	Text    customerText `json:"text"`
}

type customerText struct {
	Content string `json:"content"`
}

type templateMessageRequest struct {
	ToUser     string                   `json:"touser"`
	TemplateID string                   `json:"template_id"`
	Data       map[string]templateField `json:"data"`
}

type templateField struct {
	Value string `json:"value"`
}

func accessTokenCacheTTL(expiresIn int64) (time.Duration, bool) {
	if expiresIn <= 0 || expiresIn > int64(maxAccessTokenLifetime/time.Second) {
		return 0, false
	}
	lifetime := time.Duration(expiresIn) * time.Second
	if lifetime > accessTokenRefreshLeadTime {
		return lifetime - accessTokenRefreshLeadTime, true
	}
	ttl := lifetime * 4 / 5
	return ttl, ttl > 0
}

func validAccessToken(token string) bool {
	return token != "" && len(token) <= maxAccessTokenBytes &&
		!strings.ContainsAny(token, " \t\r\n") && utf8.ValidString(token)
}

func validTemplateIdentifier(value string, maxBytes int) bool {
	return validBoundedText(value, 1, maxBytes) && strings.TrimSpace(value) == value &&
		!strings.ContainsAny(value, "\t\r\n")
}

func isInvalidAccessTokenError(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.Code {
	case 40001, 40014, 42001:
		return true
	default:
		return false
	}
}

func newProviderError(operation string, code, status int) *APIError {
	return &APIError{
		Operation:  operation,
		Code:       code,
		HTTPStatus: status,
		Retryable:  code == -1 || code == 45009 || code == 45011 || status == http.StatusTooManyRequests || status >= 500,
	}
}

func waitForContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
