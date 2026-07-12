package xidian

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	xidianapp "mathstudy/backend-go/internal/application/xidian"
	"mathstudy/backend-go/internal/platform/httpjson"
)

type session struct {
	client *http.Client
	config Config
	jar    *trackingJar
}

func newSession(client *http.Client, config Config, cookies []xidianapp.Cookie) *session {
	jar := newTrackingJar()
	for _, cookie := range cookies {
		jar.importCookie(cookie)
	}
	copyClient := *client
	copyClient.Jar = jar
	return &session{client: &copyClient, config: config, jar: jar}
}

func (s *session) request(ctx context.Context, method string, rawURL string, params url.Values, form url.Values, headers map[string]string) (*http.Response, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if err := validateXidianRequestURL(parsed, s.config); err != nil {
		return nil, err
	}
	if len(params) > 0 {
		query := parsed.Query()
		for key, values := range params {
			for _, value := range values {
				query.Add(key, value)
			}
		}
		parsed.RawQuery = query.Encode()
	}
	var response *http.Response
	for attempt := 0; attempt <= s.config.RetryCount; attempt++ {
		var body io.Reader
		if form != nil {
			body = strings.NewReader(form.Encode())
		}
		req, err := http.NewRequestWithContext(ctx, method, parsed.String(), body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", s.config.UserAgent)
		req.Header.Set("Accept", "*/*")
		if form != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		response, err = s.client.Do(req)
		if err == nil {
			return response, nil
		}
		if !isRetryable(err) || attempt == s.config.RetryCount {
			return nil, err
		}
		time.Sleep(time.Duration(1<<attempt) * time.Second)
	}
	return response, nil
}

func (s *session) followRedirects(ctx context.Context, base *url.URL, location string, headers map[string]string) (*http.Response, error) {
	current, err := resolveXidianLocation(base, location)
	if err != nil {
		return nil, err
	}
	var response *http.Response
	for range 10 {
		response, err = s.request(ctx, http.MethodGet, current, nil, nil, headers)
		if err != nil {
			return nil, err
		}
		if response.StatusCode != http.StatusMovedPermanently && response.StatusCode != http.StatusFound {
			return response, nil
		}
		next := response.Header.Get("Location")
		_ = response.Body.Close()
		if next == "" {
			return response, nil
		}
		parsedCurrent, _ := url.Parse(current)
		current, err = resolveXidianLocation(parsedCurrent, next)
		if err != nil {
			return nil, err
		}
	}
	return response, nil
}

func (s *session) getJSON(ctx context.Context, rawURL string, form url.Values, headers map[string]string) (map[string]any, int, http.Header, error) {
	method := http.MethodGet
	if form != nil {
		method = http.MethodPost
	}
	response, err := s.request(ctx, method, rawURL, nil, form, headers)
	if err != nil {
		return nil, 0, nil, err
	}
	defer response.Body.Close()
	var payload map[string]any
	if err := httpjson.DecodeLimited(response.Body, maxXidianJSONBytes, &payload); err != nil {
		return nil, response.StatusCode, response.Header, err
	}
	return payload, response.StatusCode, response.Header, nil
}

func (s *session) exportCookies() []xidianapp.Cookie {
	return s.jar.exportCookies()
}

type trackingJar struct {
	inner *cookiejar.Jar
	mu    sync.Mutex
	all   map[string]*http.Cookie
}

func newTrackingJar() *trackingJar {
	inner, _ := cookiejar.New(nil)
	return &trackingJar{inner: inner, all: map[string]*http.Cookie{}}
}

func (j *trackingJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.inner.SetCookies(u, cookies)
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, cookie := range cookies {
		// #nosec G124 -- the jar preserves upstream Xidian cookie attributes verbatim.
		copyCookie := *cookie
		domain := copyCookie.Domain
		if domain == "" {
			domain = u.Hostname()
			copyCookie.Domain = domain
		}
		if copyCookie.Path == "" {
			copyCookie.Path = "/"
		}
		j.all[domain+"|"+copyCookie.Path+"|"+copyCookie.Name] = &copyCookie
	}
}

func (j *trackingJar) Cookies(u *url.URL) []*http.Cookie {
	return j.inner.Cookies(u)
}

func (j *trackingJar) importCookie(item xidianapp.Cookie) {
	name, _ := item["name"].(string)
	value, _ := item["value"].(string)
	if name == "" {
		return
	}
	domain, _ := item["domain"].(string)
	if domain == "" {
		domain = "ehall.xidian.edu.cn"
	}
	pathValue, _ := item["path"].(string)
	if pathValue == "" {
		pathValue = "/"
	}
	// #nosec G124 -- imported upstream session cookies restore the serialized Secure flag below.
	cookie := &http.Cookie{Name: name, Value: value, Domain: domain, Path: pathValue}
	if secure, ok := item["secure"].(bool); ok {
		cookie.Secure = secure
	}
	u := &url.URL{Scheme: "https", Host: strings.TrimPrefix(domain, "."), Path: pathValue}
	j.SetCookies(u, []*http.Cookie{cookie})
}

func (j *trackingJar) exportCookies() []xidianapp.Cookie {
	j.mu.Lock()
	defer j.mu.Unlock()
	result := make([]xidianapp.Cookie, 0, len(j.all))
	for _, cookie := range j.all {
		result = append(result, xidianapp.Cookie{
			"name":    cookie.Name,
			"value":   cookie.Value,
			"domain":  cookie.Domain,
			"path":    cookie.Path,
			"expires": cookie.Expires.Unix(),
			"secure":  cookie.Secure,
		})
	}
	return result
}

func isRetryable(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}

func resolveXidianLocation(base *url.URL, location string) (string, error) {
	parsed, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	if base != nil {
		parsed = base.ResolveReference(parsed)
	}
	return parsed.String(), nil
}

func validateXidianRequestURL(parsed *url.URL, config Config) error {
	if parsed == nil || parsed.Scheme != "https" || parsed.Host == "" {
		return errors.New("xidian request URL must be absolute HTTPS")
	}
	if parsed.User != nil {
		return errors.New("xidian request URL must not include userinfo")
	}
	for _, baseURL := range []string{config.IDsBase, config.EhallBase, config.YjsptBase} {
		base, err := url.Parse(baseURL)
		if err == nil && strings.EqualFold(parsed.Host, base.Host) {
			return nil
		}
	}
	return fmt.Errorf("xidian request host %q is not configured", parsed.Host)
}
