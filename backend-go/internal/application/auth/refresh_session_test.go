package auth

import (
	"context"
	"testing"
	"time"
)

func TestRefreshSessionStoreLocalFallbackConsumesOnce(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	store := NewRefreshSessionStore(nil, nil)
	store.now = func() time.Time { return now }

	if err := store.Remember(context.Background(), "user-1", "jti-1", now.Add(time.Hour)); err != nil {
		t.Fatalf("Remember() error = %v", err)
	}
	active, err := store.Consume(context.Background(), "user-1", "jti-1")
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if !active {
		t.Fatal("Consume() active = false, want true")
	}
	active, err = store.Consume(context.Background(), "user-1", "jti-1")
	if err != nil {
		t.Fatalf("Consume(reuse) error = %v", err)
	}
	if active {
		t.Fatal("Consume(reuse) active = true, want false")
	}
}

func TestRefreshSessionStoreStrictRequiresRedis(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	store := NewRefreshSessionStore(nil, nil, WithStrictRefreshSessions(true))
	store.now = func() time.Time { return now }

	if err := store.Remember(context.Background(), "user-1", "jti-1", now.Add(time.Hour)); err == nil {
		t.Fatal("Remember(strict without redis) error = nil, want error")
	}
	if _, err := store.Consume(context.Background(), "user-1", "jti-1"); err == nil {
		t.Fatal("Consume(strict without redis) error = nil, want error")
	}
	if err := store.Revoke(context.Background(), "jti-1"); err == nil {
		t.Fatal("Revoke(strict without redis) error = nil, want error")
	}
}

func TestRefreshSessionStoreBoundsAndPrunesLocalSessions(t *testing.T) {
	now := time.Date(2026, 7, 14, 2, 0, 0, 0, time.UTC)
	store := NewRefreshSessionStore(nil, nil, WithMaxLocalRefreshSessions(2))
	store.now = func() time.Time { return now }
	ctx := context.Background()

	for index, ttl := range []time.Duration{time.Hour, 2 * time.Hour, 3 * time.Hour} {
		jti := "jti-" + string(rune('1'+index))
		if err := store.Remember(ctx, "user-1", jti, now.Add(ttl)); err != nil {
			t.Fatalf("Remember(%q) error = %v", jti, err)
		}
	}
	if len(store.sessions) != 2 {
		t.Fatalf("local session count = %d, want 2", len(store.sessions))
	}
	if active, err := store.Consume(ctx, "user-1", "jti-1"); err != nil || active {
		t.Fatalf("evicted Consume() = %t, %v", active, err)
	}

	now = now.Add(4 * time.Hour)
	if err := store.Remember(ctx, "user-1", "jti-4", now.Add(time.Hour)); err != nil {
		t.Fatalf("Remember(after expiry) error = %v", err)
	}
	if len(store.sessions) != 1 {
		t.Fatalf("pruned local session count = %d, want 1", len(store.sessions))
	}
}
