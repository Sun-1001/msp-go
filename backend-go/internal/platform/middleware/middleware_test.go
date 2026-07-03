package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSAllowsCredentialsOnlyForExplicitOrigins(t *testing.T) {
	handler := CORS([]string{"https://app.example.com"}, []string{"GET"}, []string{"Authorization"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Origin", "https://app.example.com")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Access-Control-Allow-Credentials = %q", got)
	}
}

func TestCORSWildcardDoesNotAllowCredentials(t *testing.T) {
	handler := CORS([]string{"*"}, []string{"GET"}, []string{"Authorization"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Origin", "https://app.example.com")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want empty", got)
	}
}

func TestRequestIDPreservesSafeClientHeader(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Context().Value(responseKey{}); got != "trace-123_A.B:/span" {
			t.Fatalf("request id context = %#v", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Request-ID", " trace-123_A.B:/span ")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("X-Request-ID"); got != "trace-123_A.B:/span" {
		t.Fatalf("X-Request-ID = %q", got)
	}
}

func TestRequestIDReplacesUnsafeClientHeader(t *testing.T) {
	original := readRequestIDRandom
	readRequestIDRandom = func(data []byte) (int, error) {
		for i := range data {
			data[i] = byte(i)
		}
		return len(data), nil
	}
	defer func() {
		readRequestIDRandom = original
	}()
	for _, tc := range []struct {
		name   string
		header string
	}{
		{name: "space", header: "bad value"},
		{name: "control", header: "bad\rvalue"},
		{name: "too-long", header: strings.Repeat("a", maxRequestIDLength+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Context().Value(responseKey{}); got != "000102030405060708090a0b0c0d0e0f" {
					t.Fatalf("request id context = %#v", got)
				}
			}))
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("X-Request-ID", tc.header)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			if got := recorder.Header().Get("X-Request-ID"); got != "000102030405060708090a0b0c0d0e0f" {
				t.Fatalf("X-Request-ID = %q", got)
			}
		})
	}
}

func TestNewRequestIDFallbackIsUnique(t *testing.T) {
	original := readRequestIDRandom
	readRequestIDRandom = func([]byte) (int, error) {
		return 0, errors.New("random unavailable")
	}
	requestIDFallbackSerial.Store(0)
	defer func() {
		readRequestIDRandom = original
		requestIDFallbackSerial.Store(0)
	}()

	first := newRequestID()
	second := newRequestID()

	if first == second {
		t.Fatalf("fallback request IDs should be unique, got %q", first)
	}
	if len(first) != 32 || len(second) != 32 {
		t.Fatalf("fallback request IDs = %q, %q; want 32 hex characters", first, second)
	}
}
