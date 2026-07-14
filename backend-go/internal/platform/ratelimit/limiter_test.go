package ratelimit

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestNewValidatesConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		limit   int
		window  time.Duration
		maxKeys int
	}{
		{name: "empty prefix", limit: 1, window: time.Minute, maxKeys: 1},
		{name: "invalid limit", prefix: "test", window: time.Minute, maxKeys: 1},
		{name: "invalid window", prefix: "test", limit: 1, maxKeys: 1},
		{name: "invalid capacity", prefix: "test", limit: 1, window: time.Minute},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New(nil, test.prefix, test.limit, test.window, test.maxKeys, nil); err == nil {
				t.Fatal("New() error = nil")
			}
		})
	}
}

func TestLimiterLocalWindowExpiresAndStaysBounded(t *testing.T) {
	now := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	limiter, err := New(nil, "test", 2, time.Minute, 2, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	limiter.local.now = func() time.Time { return now }

	if !limiter.Allow(context.Background(), "a") {
		t.Fatal("first hit should be allowed")
	}
	if !limiter.Allow(context.Background(), "a") {
		t.Fatal("second hit should be allowed")
	}
	if limiter.Allow(context.Background(), "a") {
		t.Fatal("third hit should be rejected")
	}
	if !limiter.Allow(context.Background(), "b") || !limiter.Allow(context.Background(), "c") {
		t.Fatal("new keys should be admitted while the local store evicts old state")
	}
	if size := localSize(limiter.local); size > 2 {
		t.Fatalf("local key count = %d, want <= 2", size)
	}

	now = now.Add(time.Minute + time.Nanosecond)
	if !limiter.Allow(context.Background(), "a") {
		t.Fatal("expired window should allow a new hit")
	}
	if !(*Limiter)(nil).Allow(context.Background(), "a") {
		t.Fatal("nil limiter should fail open")
	}
}

func TestLimiterRedisStateIsSharedAndRepairsMissingTTL(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	first, err := New(client, "shared", 2, time.Minute, 4, nil)
	if err != nil {
		t.Fatalf("New(first) error = %v", err)
	}
	second, err := New(client, "shared", 2, time.Minute, 4, nil)
	if err != nil {
		t.Fatalf("New(second) error = %v", err)
	}
	ctx := context.Background()
	if !first.Allow(ctx, "student-1") || !second.Allow(ctx, "student-1") {
		t.Fatal("first two shared hits should be allowed")
	}
	if first.Allow(ctx, "student-1") {
		t.Fatal("third shared hit should be rejected")
	}
	if ttl := server.TTL("shared:student-1"); ttl <= 0 {
		t.Fatalf("shared counter TTL = %s", ttl)
	}

	server.Set("legacy-counter", "2")
	count, err := IncrementWithExpiry(ctx, client, "legacy-counter", time.Minute)
	if err != nil || count != 3 {
		t.Fatalf("IncrementWithExpiry() = %d, %v", count, err)
	}
	if ttl := server.TTL("legacy-counter"); ttl <= 0 {
		t.Fatalf("repaired counter TTL = %s", ttl)
	}
}

func TestIncrementWithExpiryValidatesInputs(t *testing.T) {
	client := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = client.Close() })
	if _, err := IncrementWithExpiry(context.Background(), nil, "key", time.Minute); err == nil {
		t.Fatal("nil client error = nil")
	}
	var nilContext context.Context
	if _, err := IncrementWithExpiry(nilContext, client, "key", time.Minute); err == nil {
		t.Fatal("nil context error = nil")
	}
	if _, err := IncrementWithExpiry(context.Background(), client, "", time.Minute); err == nil {
		t.Fatal("empty key error = nil")
	}
	if _, err := IncrementWithExpiry(context.Background(), client, "key", 0); err == nil {
		t.Fatal("invalid TTL error = nil")
	}
}

func TestIncrementWithRefreshedExpiryRestartsTTL(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()

	if _, err := IncrementWithRefreshedExpiry(ctx, client, "sliding-counter", time.Minute); err != nil {
		t.Fatal(err)
	}
	server.FastForward(40 * time.Second)
	before := server.TTL("sliding-counter")
	count, err := IncrementWithRefreshedExpiry(ctx, client, "sliding-counter", time.Minute)
	if err != nil || count != 2 {
		t.Fatalf("IncrementWithRefreshedExpiry() = %d, %v", count, err)
	}
	after := server.TTL("sliding-counter")
	if before >= after || after < 59*time.Second {
		t.Fatalf("refreshed TTL before/after = %s/%s", before, after)
	}
}

func TestLimiterLocalFallbackIsConcurrencySafe(t *testing.T) {
	limiter, err := New(nil, "parallel", 25, time.Minute, 128, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	var allowed atomic.Int64
	var group sync.WaitGroup
	for range 200 {
		group.Add(1)
		go func() {
			defer group.Done()
			if limiter.Allow(context.Background(), "same-key") {
				allowed.Add(1)
			}
		}()
	}
	group.Wait()
	if got := allowed.Load(); got != 25 {
		t.Fatalf("allowed = %d, want 25", got)
	}
}

func TestLimiterFallsBackWhenRedisIsUnavailable(t *testing.T) {
	client := goredis.NewClient(&goredis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 5 * time.Millisecond,
		ReadTimeout: 5 * time.Millisecond,
	})
	t.Cleanup(func() { _ = client.Close() })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	limiter, err := New(client, "fallback", 1, time.Minute, 2, logger)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if !limiter.Allow(ctx, "key") {
		t.Fatal("first fallback hit should be allowed")
	}
	if limiter.Allow(ctx, "key") {
		t.Fatal("second fallback hit should be rejected")
	}
}

func BenchmarkLimiterLocalParallel(b *testing.B) {
	limiter, err := New(nil, "benchmark", 60, time.Minute, 4096, nil)
	if err != nil {
		b.Fatal(err)
	}
	keys := make([]string, 1024)
	for index := range keys {
		keys[index] = "key-" + strconv.Itoa(index)
	}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var index int
		for pb.Next() {
			index++
			limiter.Allow(context.Background(), keys[index&(len(keys)-1)])
		}
	})
}

func localSize(limiter *localLimiter) int {
	total := 0
	for index := range limiter.shards {
		shard := &limiter.shards[index]
		shard.mu.Lock()
		total += len(shard.entries)
		shard.mu.Unlock()
	}
	return total
}
