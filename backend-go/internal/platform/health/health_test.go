package health

import (
	"context"
	"errors"
	"testing"
	"time"
)

type pingerFunc func(context.Context) error

func (f pingerFunc) Ping(ctx context.Context) error {
	return f(ctx)
}

func TestCheckerDetailedReportsDegradedDependency(t *testing.T) {
	checker := NewChecker(
		"test",
		pingerFunc(func(context.Context) error { return nil }),
		pingerFunc(func(context.Context) error { return errors.New("redis down") }),
	)

	status := checker.Detailed(context.Background())

	if status.Status != "degraded" {
		t.Fatalf("Status = %q, want degraded", status.Status)
	}
	if status.Components["postgres"].Status != "healthy" {
		t.Fatalf("postgres status = %q", status.Components["postgres"].Status)
	}
	if status.Components["redis"].Status != "unhealthy" {
		t.Fatalf("redis status = %q", status.Components["redis"].Status)
	}
}

func TestCheckerSimple(t *testing.T) {
	checker := NewChecker("0.1.0", nil, nil)
	got := checker.Simple()

	if got["status"] != "healthy" || got["version"] != "0.1.0" {
		t.Fatalf("Simple() = %#v", got)
	}
}

func TestCheckerDetailedChecksDependenciesConcurrently(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	pinger := pingerFunc(func(ctx context.Context) error {
		started <- struct{}{}
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	checker := NewChecker("test", pinger, pinger)
	done := make(chan DetailedStatus, 1)
	go func() {
		done <- checker.Detailed(context.Background())
	}()

	for range 2 {
		select {
		case <-started:
		case <-time.After(500 * time.Millisecond):
			close(release)
			t.Fatal("dependency checks did not start concurrently")
		}
	}
	close(release)
	select {
	case status := <-done:
		if status.Status != "healthy" {
			t.Fatalf("Status = %q", status.Status)
		}
	case <-time.After(time.Second):
		t.Fatal("Detailed() did not return after dependency checks completed")
	}
}
