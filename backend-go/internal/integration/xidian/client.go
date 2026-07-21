package xidian

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	xidianapp "mathstudy/backend-go/internal/application/xidian"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/outbound"
)

const (
	maxXidianHTMLBytes = 2 << 20
	maxXidianJSONBytes = 5 << 20
)

var errXidianResponseTooLarge = errors.New("xidian response body too large")

// Config contains Xidian portal HTTP settings.
type Config struct {
	IDsBase        string
	EhallBase      string
	UserAgent      string
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	RetryCount     int
	CaptchaWidth   int
}

// Client implements the Xidian IDS account verification flow.
type Client struct {
	config Config
	client *http.Client
	now    func() time.Time
}

// NewClient creates a Xidian integration client.
func NewClient(config Config, clients ...*http.Client) (*Client, error) {
	if strings.TrimSpace(config.IDsBase) == "" || strings.TrimSpace(config.EhallBase) == "" {
		return nil, errors.New("xidian base urls must not be empty")
	}
	if config.UserAgent == "" {
		config.UserAgent = "Mozilla/5.0"
	}
	if config.ConnectTimeout <= 0 {
		config.ConnectTimeout = 10 * time.Second
	}
	if config.ReadTimeout <= 0 {
		config.ReadTimeout = 30 * time.Second
	}
	if config.CaptchaWidth <= 0 {
		config.CaptchaWidth = 280
	}
	var err error
	if config.IDsBase, err = normalizeXidianBaseURL("XIDIAN_IDS_BASE", config.IDsBase); err != nil {
		return nil, err
	}
	if config.EhallBase, err = normalizeXidianBaseURL("XIDIAN_EHALL_BASE", config.EhallBase); err != nil {
		return nil, err
	}
	return &Client{
		config: config,
		client: xidianHTTPClient(config.ConnectTimeout+config.ReadTimeout, clients...),
		now:    func() time.Time { return time.Now().UTC() },
	}, nil
}

func normalizeXidianBaseURL(name string, value string) (string, error) {
	baseURL, err := outbound.NormalizePublicHTTPSBaseURL(value)
	if err != nil {
		return "", fmt.Errorf("%s %w", name, err)
	}
	return baseURL, nil
}

func xidianHTTPClient(timeout time.Duration, clients ...*http.Client) *http.Client {
	if timeout <= 0 {
		timeout = 40 * time.Second
	}
	if len(clients) > 0 && clients[0] != nil {
		copy := *clients[0]
		if copy.Timeout == 0 {
			copy.Timeout = timeout
		}
		copy.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
		return &copy
	}
	return outbound.NewPublicHTTPSClient(timeout)
}

// StartBinding fetches IDS login state and opens a slider captcha.
func (c *Client) StartBinding(ctx context.Context) (xidianapp.Challenge, error) {
	session := newSession(c.client, c.config, nil)
	serviceURL := c.config.EhallBase + "/login?service=" + c.config.EhallBase + "/new/index.html"
	loginURL := c.config.IDsBase + "/authserver/login"
	loginResponse, err := session.request(ctx, http.MethodGet, loginURL, url.Values{"service": {serviceURL}}, nil, nil)
	if err != nil {
		return xidianapp.Challenge{}, err
	}
	defer loginResponse.Body.Close()
	if loginResponse.StatusCode >= 400 {
		return xidianapp.Challenge{}, xidianapp.ServiceError{Code: "LOGIN_PAGE_INVALID", Message: "无法解析登录页面", Status: 400}
	}
	loginHTML, err := readLimitedString(loginResponse.Body, maxXidianHTMLBytes)
	if err != nil {
		return xidianapp.Challenge{}, err
	}
	page := parseLoginPage(loginHTML)
	if page.PasswordSalt == "" {
		return xidianapp.Challenge{}, xidianapp.ServiceError{Code: "LOGIN_PAGE_INVALID", Message: "无法解析登录页面", Status: 400}
	}
	captchaURL := c.config.IDsBase + "/authserver/common/openSliderCaptcha.htl"
	captchaResponse, err := session.request(ctx, http.MethodGet, captchaURL, url.Values{"_": {strconv.FormatInt(c.now().UnixMilli(), 10)}}, nil, nil)
	if err != nil {
		return xidianapp.Challenge{}, err
	}
	defer captchaResponse.Body.Close()
	var captcha map[string]any
	if err := httpjson.DecodeLimited(captchaResponse.Body, maxXidianJSONBytes, &captcha); err != nil {
		return xidianapp.Challenge{}, err
	}
	return xidianapp.Challenge{
		CaptchaBig:   stringFromMap(captcha, "bigImage"),
		CaptchaPiece: stringFromMap(captcha, "smallImage"),
		PieceY:       intFromAny(firstPresent(captcha, "y", "offsetY", "top"), 0),
		State: xidianapp.ChallengeState{
			ServiceURL:   serviceURL,
			HiddenInputs: page.HiddenInputs,
			PasswordSalt: page.PasswordSalt,
			Cookies:      session.exportCookies(),
			CreatedAt:    c.now(),
		},
	}, nil
}

// CompleteBinding verifies captcha and submits the IDS login form.
func (c *Client) CompleteBinding(ctx context.Context, state xidianapp.ChallengeState, input xidianapp.LoginInput) error {
	session := newSession(c.client, c.config, state.Cookies)
	verifyURL := c.config.IDsBase + "/authserver/common/verifySliderCaptcha.htl"
	verifyData := url.Values{
		"canvasLength": {strconv.Itoa(c.config.CaptchaWidth)},
		"moveLength":   {strconv.Itoa(int(input.SliderPosition * float64(c.config.CaptchaWidth)))},
	}
	verifyResponse, err := session.request(ctx, http.MethodPost, verifyURL, nil, verifyData, map[string]string{
		"Content-Type": "application/x-www-form-urlencoded;charset=utf-8",
		"Origin":       c.config.IDsBase,
	})
	if err != nil {
		return err
	}
	defer verifyResponse.Body.Close()
	var verifyPayload map[string]any
	if err := httpjson.DecodeLimited(verifyResponse.Body, maxXidianJSONBytes, &verifyPayload); err != nil {
		return err
	}
	if intFromAny(verifyPayload["errorCode"], 0) != 1 {
		return xidianapp.ServiceError{Code: "CAPTCHA_FAILED", Message: "验证码校验失败", Status: 400}
	}
	if state.PasswordSalt == "" {
		return xidianapp.ServiceError{Code: "LOGIN_PAGE_INVALID", Message: "登录参数缺失", Status: 400}
	}
	encryptedPassword, err := aesEncryptPassword(input.Password, state.PasswordSalt)
	if err != nil {
		return err
	}
	form := url.Values{}
	for key, value := range state.HiddenInputs {
		if key != "pwdEncryptSalt" {
			form.Set(key, value)
		}
	}
	form.Set("username", input.Username)
	form.Set("password", encryptedPassword)
	form.Set("rememberMe", "true")
	form.Set("cllt", "userNameLogin")
	form.Set("dllt", "generalLogin")
	form.Set("_eventId", "submit")

	loginURL := c.config.IDsBase + "/authserver/login"
	response, err := session.request(ctx, http.MethodPost, loginURL, nil, form, nil)
	if err != nil {
		return err
	}
	if err := c.handleLoginResponse(ctx, session, response); err != nil {
		return err
	}
	return nil
}

func (c *Client) handleLoginResponse(ctx context.Context, session *session, response *http.Response) error {
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusMovedPermanently, http.StatusFound:
		location := response.Header.Get("Location")
		if location == "" {
			return xidianapp.ServiceError{Code: "LOGIN_FAILED", Message: "登录失败，请重试", Status: 400}
		}
		_, err := session.followRedirects(ctx, response.Request.URL, location, nil)
		return err
	case http.StatusOK:
		body, err := readLimitedString(response.Body, maxXidianHTMLBytes)
		if err != nil {
			return err
		}
		page := parseLoginPage(body)
		if page.ErrorMessage != "" {
			return xidianapp.ServiceError{Code: "PASSWORD_WRONG", Message: page.ErrorMessage, Status: 401}
		}
		if len(page.ContinueInputs) == 0 {
			return xidianapp.ServiceError{Code: "LOGIN_FAILED", Message: "登录失败，请重试", Status: 400}
		}
		form := url.Values{}
		for key, value := range page.ContinueInputs {
			form.Set(key, value)
		}
		next, err := session.request(ctx, http.MethodPost, c.config.IDsBase+"/authserver/login", nil, form, nil)
		if err != nil {
			return err
		}
		defer next.Body.Close()
		if next.StatusCode == http.StatusMovedPermanently || next.StatusCode == http.StatusFound {
			location := next.Header.Get("Location")
			if location != "" {
				_, err = session.followRedirects(ctx, next.Request.URL, location, nil)
				return err
			}
		}
		return xidianapp.ServiceError{Code: "LOGIN_FAILED", Message: "登录失败，请重试", Status: 400}
	case http.StatusUnauthorized:
		body, err := readLimitedString(response.Body, maxXidianHTMLBytes)
		if err != nil {
			return err
		}
		page := parseLoginPage(body)
		message := page.ErrorMessage
		if message == "" {
			message = "用户名或密码有误"
		}
		return xidianapp.ServiceError{Code: "PASSWORD_WRONG", Message: message, Status: 401}
	default:
		return xidianapp.ServiceError{Code: "LOGIN_FAILED", Message: "登录失败，请稍后重试", Status: 400}
	}
}

func readLimitedString(reader io.Reader, maxBytes int64) (string, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > maxBytes {
		return "", errXidianResponseTooLarge
	}
	return string(data), nil
}
