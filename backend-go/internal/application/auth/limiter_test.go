package auth

import (
	"context"
	"testing"
	"time"
)

func TestLoginLimiterLocalFallbackLocksAndClears(t *testing.T) {
	limiter := NewLoginLimiter(nil, 2, time.Minute, nil)
	ctx := context.Background()

	if limiter.IsLocked(ctx, "alice") {
		t.Fatal("new limiter reported locked account")
	}
	limiter.RecordFailure(ctx, "alice")
	if limiter.IsLocked(ctx, "alice") {
		t.Fatal("account locked after one failure")
	}
	limiter.RecordFailure(ctx, "alice")
	if !limiter.IsLocked(ctx, "alice") {
		t.Fatal("account not locked after max failures")
	}
	limiter.Clear(ctx, "alice")
	if limiter.IsLocked(ctx, "alice") {
		t.Fatal("account still locked after clear")
	}
}
