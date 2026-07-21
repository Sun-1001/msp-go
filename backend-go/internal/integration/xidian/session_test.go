package xidian

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestSessionRetryBackoffStopsWhenContextIsCanceled(t *testing.T) {
	attempted := make(chan struct{}, 1)
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		attempted <- struct{}{}
		return nil, timeoutError{}
	})}
	session := newSession(client, Config{
		IDsBase:    "https://ids.example.com",
		EhallBase:  "https://ehall.example.com",
		RetryCount: 3,
	}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := session.request(ctx, http.MethodGet, "https://ids.example.com/login", nil, nil, nil)
		done <- err
	}()

	select {
	case <-attempted:
	case <-time.After(time.Second):
		t.Fatal("initial request did not run")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("request error = %v, want context.Canceled", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("retry backoff did not stop after context cancellation")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }
