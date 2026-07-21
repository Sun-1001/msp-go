package xidian

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	xidianapp "mathstudy/backend-go/internal/application/xidian"
)

func TestStartBindingParsesLoginPageAndCaptcha(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/authserver/login":
			_, _ = w.Write([]byte(`<form><input type="hidden" name="lt" value="token"><input id="pwdEncryptSalt" value="1234567890abcdef"></form>`))
		case "/authserver/common/openSliderCaptcha.htl":
			_ = json.NewEncoder(w).Encode(map[string]any{"bigImage": "big", "smallImage": "piece", "y": 17})
		default:
			http.NotFound(w, r)
		}
	}))

	challenge, err := client.StartBinding(context.Background())
	if err != nil {
		t.Fatalf("StartBinding() error = %v", err)
	}
	if challenge.CaptchaBig != "big" || challenge.CaptchaPiece != "piece" || challenge.PieceY != 17 {
		t.Fatalf("challenge = %#v", challenge)
	}
	if challenge.State.PasswordSalt != "1234567890abcdef" || challenge.State.HiddenInputs["lt"] != "token" {
		t.Fatalf("state = %#v", challenge.State)
	}
}

func TestStartBindingRejectsOversizedLoginPage(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/authserver/login":
			_, _ = w.Write([]byte(strings.Repeat("x", maxXidianHTMLBytes+1)))
		default:
			http.NotFound(w, r)
		}
	}))

	if _, err := client.StartBinding(context.Background()); !errors.Is(err, errXidianResponseTooLarge) {
		t.Fatalf("StartBinding() error = %v, want errXidianResponseTooLarge", err)
	}
}

func TestCompleteBindingSubmitsEncryptedPassword(t *testing.T) {
	var loginBody string
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/authserver/common/verifySliderCaptcha.htl":
			_ = json.NewEncoder(w).Encode(map[string]any{"errorCode": 1})
		case "/authserver/login":
			if r.Method != http.MethodPost {
				http.NotFound(w, r)
				return
			}
			_ = r.ParseForm()
			loginBody = r.PostForm.Encode()
			http.Redirect(w, r, "/new/index.html", http.StatusFound)
		case "/new/index.html":
			_, _ = w.Write([]byte("ok"))
		default:
			http.NotFound(w, r)
		}
	}))

	err := client.CompleteBinding(context.Background(), xidianapp.ChallengeState{
		PasswordSalt: "1234567890abcdef",
		HiddenInputs: map[string]string{"lt": "token", "pwdEncryptSalt": "1234567890abcdef"},
	}, xidianapp.LoginInput{Username: "student", Password: "plain", SliderPosition: 0.5})
	if err != nil {
		t.Fatalf("CompleteBinding() error = %v", err)
	}
	if !strings.Contains(loginBody, "username=student") || strings.Contains(loginBody, "password=plain") || !strings.Contains(loginBody, "lt=token") {
		t.Fatalf("login body = %q", loginBody)
	}
}

func TestCompleteBindingRejectsRedirectOutsideConfiguredHosts(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/authserver/common/verifySliderCaptcha.htl":
			_ = json.NewEncoder(w).Encode(map[string]any{"errorCode": 1})
		case "/authserver/login":
			http.Redirect(w, r, "https://evil.example.com/new/index.html", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))

	err := client.CompleteBinding(context.Background(), xidianapp.ChallengeState{
		PasswordSalt: "1234567890abcdef",
		HiddenInputs: map[string]string{"lt": "token"},
		ServiceURL:   "https://ehall.example.com/login",
		Cookies:      nil,
		CreatedAt:    time.Now(),
	}, xidianapp.LoginInput{Username: "student", Password: "plain", SliderPosition: 0.5})
	if err == nil {
		t.Fatal("CompleteBinding() error = nil, want redirect host error")
	}
}

func TestNewClientRejectsMissingBaseURLs(t *testing.T) {
	if _, err := NewClient(Config{}); err == nil {
		t.Fatal("NewClient(empty) error = nil, want error")
	}
}

func TestNewClientRejectsUnsafeBaseURLs(t *testing.T) {
	cases := []Config{
		{IDsBase: "http://ids.example.com", EhallBase: "https://ehall.example.com"},
		{IDsBase: "https://127.0.0.1:8443", EhallBase: "https://ehall.example.com"},
		{IDsBase: "https://ids.example.com", EhallBase: "https://10.0.0.1"},
		{IDsBase: "https://ids.example.com", EhallBase: "https://ehall.example.com?target=internal"},
	}
	for _, cfg := range cases {
		t.Run(cfg.IDsBase+"|"+cfg.EhallBase, func(t *testing.T) {
			if _, err := NewClient(cfg); err == nil {
				t.Fatal("NewClient() error = nil, want unsafe base URL error")
			}
		})
	}
}

func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	httpClient := &http.Client{Transport: roundTripHandler{handler: handler}}
	client, err := NewClient(Config{
		IDsBase:        "https://ids.example.com",
		EhallBase:      "https://ehall.example.com",
		UserAgent:      "test-agent",
		ConnectTimeout: time.Second,
		ReadTimeout:    time.Second,
		CaptchaWidth:   280,
	}, httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

type roundTripHandler struct {
	handler http.Handler
}

func (h roundTripHandler) RoundTrip(r *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	h.handler.ServeHTTP(recorder, r)
	response := recorder.Result()
	response.Request = r
	return response, nil
}
