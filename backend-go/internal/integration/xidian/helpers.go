package xidian

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	xidianapp "mathstudy/backend-go/internal/application/xidian"
)

type loginPage struct {
	HiddenInputs   map[string]string
	ContinueInputs map[string]string
	PasswordSalt   string
	ErrorMessage   string
}

var (
	inputTagPattern     = regexp.MustCompile(`(?is)<input\b[^>]*>`)
	formContinuePattern = regexp.MustCompile(`(?is)<form\b[^>]*(?:id|name)=["']continue["'][^>]*>(.*?)</form>`)
	attrPattern         = regexp.MustCompile(`(?is)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*("([^"]*)"|'([^']*)'|([^\s>]+))`)
	errorPattern        = regexp.MustCompile(`(?is)<[^>]*(?:id|class)=["'][^"']*showErrorTip[^"']*["'][^>]*>(.*?)</[^>]+>`)
	htmlTagPattern      = regexp.MustCompile(`(?is)<[^>]+>`)
	integerPattern      = regexp.MustCompile(`[0-9]+`)
	sessionIDPattern    = regexp.MustCompile(`;jsessionid=.*?\?`)
)

func parseLoginPage(rawHTML string) loginPage {
	page := loginPage{HiddenInputs: map[string]string{}, ContinueInputs: map[string]string{}}
	for _, tag := range inputTagPattern.FindAllString(rawHTML, -1) {
		attrs := parseAttrs(tag)
		name := attrs["name"]
		if name == "" {
			name = attrs["id"]
		}
		if name == "" {
			continue
		}
		value := attrs["value"]
		if attrs["id"] == "pwdEncryptSalt" {
			page.PasswordSalt = value
		}
		if strings.EqualFold(attrs["type"], "hidden") {
			page.HiddenInputs[name] = value
		}
	}
	if match := formContinuePattern.FindStringSubmatch(rawHTML); len(match) == 2 {
		for _, tag := range inputTagPattern.FindAllString(match[1], -1) {
			attrs := parseAttrs(tag)
			name := attrs["name"]
			if name == "" {
				name = attrs["id"]
			}
			if name != "" {
				page.ContinueInputs[name] = attrs["value"]
			}
		}
	}
	if match := errorPattern.FindStringSubmatch(rawHTML); len(match) == 2 {
		page.ErrorMessage = strings.TrimSpace(html.UnescapeString(htmlTagPattern.ReplaceAllString(match[1], " ")))
	}
	return page
}

func parseAttrs(tag string) map[string]string {
	attrs := map[string]string{}
	for _, match := range attrPattern.FindAllStringSubmatch(tag, -1) {
		value := match[3]
		if value == "" {
			value = match[4]
		}
		if value == "" {
			value = match[5]
		}
		attrs[strings.ToLower(match[1])] = html.UnescapeString(value)
	}
	return attrs
}

func aesEncryptPassword(password string, salt string) (string, error) {
	if len(salt) != aes.BlockSize {
		return "", fmt.Errorf("invalid Xidian password salt length %d", len(salt))
	}
	prefix := "xidianscriptsxduxidianscriptsxduxidianscriptsxduxidianscriptsxdu"
	data := []byte(prefix + password)
	padding := aes.BlockSize - len(data)%aes.BlockSize
	data = append(data, bytes.Repeat([]byte{byte(padding)}, padding)...)
	block, err := aes.NewCipher([]byte(salt))
	if err != nil {
		return "", err
	}
	encrypted := make([]byte, len(data))
	// #nosec G407 -- Xidian's legacy login protocol mandates this fixed CBC IV.
	cipher.NewCBCEncrypter(block, []byte("xidianscriptsxdu")).CryptBlocks(encrypted, data)
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func ensureNotRedirected(status int, headers http.Header) error {
	if status == http.StatusMovedPermanently || status == http.StatusFound {
		if strings.Contains(headers.Get("Location"), "authserver/login") {
			return xidianapp.ServiceError{Code: "CAPTCHA_REQUIRED", Message: "会话已过期，请重新验证", Status: 409}
		}
	}
	return nil
}

func ehallHeaders() map[string]string {
	return map[string]string{
		"Referer":         "http://ehall.xidian.edu.cn/new/index_xd.html",
		"Host":            "ehall.xidian.edu.cn",
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"Accept-Encoding": "identity",
		"Connection":      "Keep-Alive",
		"Content-Type":    "application/x-www-form-urlencoded; charset=UTF-8",
	}
}

func jsonString(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func buildWeekList(value any) []bool {
	text := fmt.Sprint(value)
	result := make([]bool, len(text))
	for i, ch := range text {
		result[i] = ch == '1'
	}
	return result
}

func intFromAny(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed)
		}
	case string:
		parsed, err := strconv.Atoi(typed)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func stringFromMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, _ := values[key].(string)
	return value
}

func firstPresent(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok && value != nil {
			return value
		}
	}
	return nil
}

func mapValue(values map[string]any, keys ...string) map[string]any {
	current := values
	for index, key := range keys {
		value, ok := current[key]
		if !ok {
			return map[string]any{}
		}
		if index == len(keys)-1 {
			nested, _ := value.(map[string]any)
			if nested == nil {
				return map[string]any{}
			}
			return nested
		}
		current, _ = value.(map[string]any)
		if current == nil {
			return map[string]any{}
		}
	}
	return current
}

func rowsFrom(values map[string]any, keys ...string) []any {
	container := mapValue(values, keys...)
	rows, _ := container["rows"].([]any)
	return rows
}

func rowMap(row any) map[string]any {
	values, _ := row.(map[string]any)
	if values == nil {
		return map[string]any{}
	}
	return values
}

func stringField(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := row[key]; ok && value != nil && fmt.Sprint(value) != "" {
			return value
		}
	}
	return nil
}
