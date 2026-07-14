package auth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
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

func TestLoginLimiterRedisStateIsSharedAndExpiring(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	first := NewLoginLimiter(client, 2, time.Minute, nil)
	second := NewLoginLimiter(client, 2, time.Minute, nil)
	ctx := context.Background()

	first.RecordFailure(ctx, "alice")
	server.FastForward(40 * time.Second)
	second.RecordFailure(ctx, "alice")
	if !first.IsLocked(ctx, "alice") {
		t.Fatal("shared account was not locked")
	}
	for _, key := range []string{loginFailPrefix + "alice", loginLockPrefix + "alice"} {
		if ttl := server.TTL(key); ttl < 59*time.Second {
			t.Fatalf("TTL(%q) = %s", key, ttl)
		}
	}
	second.Clear(ctx, "alice")
	if first.IsLocked(ctx, "alice") {
		t.Fatal("shared account remained locked after clear")
	}
}

func TestLoginLimiterBoundsAndPrunesLocalKeys(t *testing.T) {
	now := time.Date(2026, 7, 14, 1, 0, 0, 0, time.UTC)
	limiter := NewLoginLimiter(nil, 2, time.Minute, nil)
	limiter.maxLocalKeys = 2
	limiter.now = func() time.Time { return now }

	limiter.RecordFailure(context.Background(), "alice")
	now = now.Add(time.Second)
	limiter.RecordFailure(context.Background(), "bob")
	now = now.Add(time.Second)
	limiter.RecordFailure(context.Background(), "carol")
	if len(limiter.localCounts) != 2 {
		t.Fatalf("local key count = %d, want 2", len(limiter.localCounts))
	}
	if _, exists := limiter.localCounts["alice"]; exists {
		t.Fatal("oldest local key was not evicted")
	}

	now = now.Add(2 * time.Minute)
	limiter.RecordFailure(context.Background(), "dave")
	if len(limiter.localCounts) != 1 {
		t.Fatalf("pruned local key count = %d, want 1", len(limiter.localCounts))
	}
}
