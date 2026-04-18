package health

import (
	"context"
	"errors"
	"testing"
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
