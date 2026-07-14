package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestServeHTTPShutsDownAfterSignal(t *testing.T) {
	server := newFakeLifecycleServer()
	stopCh := make(chan os.Signal, 1)
	done := make(chan error, 1)
	go func() {
		done <- serveHTTP(server, stopCh, time.Second, discardLogger())
	}()

	waitForLifecycleEvent(t, server.listenStarted, "server start")
	stopCh <- os.Interrupt
	waitForLifecycleEvent(t, server.shutdownCalled, "server shutdown")
	if !server.shutdownHadDeadline {
		t.Fatal("Shutdown() context had no deadline")
	}
	if err := <-done; err != nil {
		t.Fatalf("serveHTTP() error = %v", err)
	}
}

func TestServeHTTPReturnsListenError(t *testing.T) {
	wantErr := errors.New("listen failed")
	server := newFakeLifecycleServer()
	server.listenResult <- wantErr

	err := serveHTTP(server, make(chan os.Signal), time.Second, discardLogger())
	if !errors.Is(err, wantErr) {
		t.Fatalf("serveHTTP() error = %v, want %v", err, wantErr)
	}
	select {
	case <-server.shutdownCalled:
		t.Fatal("Shutdown() called after a listen failure")
	default:
	}
}

func TestServeHTTPReturnsShutdownError(t *testing.T) {
	wantErr := errors.New("shutdown failed")
	server := newFakeLifecycleServer()
	server.shutdownErr = wantErr
	stopCh := make(chan os.Signal, 1)
	stopCh <- os.Interrupt

	err := serveHTTP(server, stopCh, time.Second, discardLogger())
	if !errors.Is(err, wantErr) {
		t.Fatalf("serveHTTP() error = %v, want %v", err, wantErr)
	}
}

func TestServeHTTPShutsDownAfterServerClosed(t *testing.T) {
	server := newFakeLifecycleServer()
	server.listenResult <- http.ErrServerClosed

	if err := serveHTTP(server, make(chan os.Signal), time.Second, discardLogger()); err != nil {
		t.Fatalf("serveHTTP() error = %v", err)
	}
	waitForLifecycleEvent(t, server.shutdownCalled, "server shutdown")
}

type fakeLifecycleServer struct {
	listenStarted       chan struct{}
	listenResult        chan error
	shutdownCalled      chan struct{}
	shutdownErr         error
	shutdownHadDeadline bool
}

func newFakeLifecycleServer() *fakeLifecycleServer {
	return &fakeLifecycleServer{
		listenStarted:  make(chan struct{}),
		listenResult:   make(chan error, 1),
		shutdownCalled: make(chan struct{}),
	}
}

func (s *fakeLifecycleServer) ListenAndServe() error {
	close(s.listenStarted)
	return <-s.listenResult
}

func (s *fakeLifecycleServer) Shutdown(ctx context.Context) error {
	_, s.shutdownHadDeadline = ctx.Deadline()
	close(s.shutdownCalled)
	select {
	case s.listenResult <- http.ErrServerClosed:
	default:
	}
	return s.shutdownErr
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func waitForLifecycleEvent(t *testing.T, event <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-event:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}
