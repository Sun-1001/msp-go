package redisadapter

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestNewAIRiskSlotStoreValidatesClient(t *testing.T) {
	if _, err := NewAIRiskSlotStore(nil); err == nil {
		t.Fatal("NewAIRiskSlotStore(nil) error = nil")
	}
}

func TestAIRiskSlotStoreEnforcesConcurrencyQuotaReleaseAndTTL(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store, err := NewAIRiskSlotStore(client)
	if err != nil {
		t.Fatalf("NewAIRiskSlotStore() error = %v", err)
	}
	ctx := context.Background()
	ttl := time.Minute

	first, err := store.Acquire(ctx, "student-1", "lease-1", 2, 2, 0, ttl)
	if err != nil || !first.Allowed {
		t.Fatalf("first Acquire() = %#v, %v", first, err)
	}
	nonMetered, err := store.Acquire(ctx, "student-1", "lease-exercise", 2, 0, 0, ttl)
	if err != nil || !nonMetered.Allowed {
		t.Fatalf("non-metered Acquire() = %#v, %v", nonMetered, err)
	}
	concurrency, err := store.Acquire(ctx, "student-1", "lease-3", 2, 2, 0, ttl)
	if err != nil || concurrency.Allowed || concurrency.Reason != "concurrency" {
		t.Fatalf("concurrency Acquire() = %#v, %v", concurrency, err)
	}
	if err := store.Release(ctx, "student-1", "lease-exercise"); err != nil {
		t.Fatalf("Release(non-metered) error = %v", err)
	}
	quota, err := store.Acquire(ctx, "student-1", "lease-2", 2, 2, 1, ttl)
	if err != nil || quota.Allowed || quota.Reason != "quota" {
		t.Fatalf("quota Acquire() = %#v, %v", quota, err)
	}
	if err := store.Release(ctx, "student-1", "lease-1"); err != nil {
		t.Fatalf("Release(metered) error = %v", err)
	}
	allowed, err := store.Acquire(ctx, "student-1", "lease-2", 2, 2, 1, ttl)
	if err != nil || !allowed.Allowed {
		t.Fatalf("Acquire(after release) = %#v, %v", allowed, err)
	}

	server.FastForward(ttl + 2*time.Second)
	afterTTL, err := store.Acquire(ctx, "student-1", "lease-after-ttl", 1, 0, 0, ttl)
	if err != nil || !afterTTL.Allowed {
		t.Fatalf("Acquire(after TTL) = %#v, %v", afterTTL, err)
	}
	if err := store.Release(ctx, "student-1", "missing"); err != nil {
		t.Fatalf("idempotent Release() error = %v", err)
	}
}

func TestAIRiskSlotStoreValidatesInputs(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store, err := NewAIRiskSlotStore(client)
	if err != nil {
		t.Fatal(err)
	}
	var nilContext context.Context
	tests := []struct {
		name      string
		ctx       context.Context
		studentID string
		leaseID   string
		max       int
		daily     int
		used      int
		ttl       time.Duration
	}{
		{name: "nil context", studentID: "student", leaseID: "lease", max: 1, ttl: time.Minute},
		{name: "empty student", ctx: context.Background(), leaseID: "lease", max: 1, ttl: time.Minute},
		{name: "empty lease", ctx: context.Background(), studentID: "student", max: 1, ttl: time.Minute},
		{name: "invalid max", ctx: context.Background(), studentID: "student", leaseID: "lease", ttl: time.Minute},
		{name: "negative daily", ctx: context.Background(), studentID: "student", leaseID: "lease", max: 1, daily: -1, ttl: time.Minute},
		{name: "negative used", ctx: context.Background(), studentID: "student", leaseID: "lease", max: 1, used: -1, ttl: time.Minute},
		{name: "invalid ttl", ctx: context.Background(), studentID: "student", leaseID: "lease", max: 1},
	}
	tests[0].ctx = nilContext
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := store.Acquire(test.ctx, test.studentID, test.leaseID, test.max, test.daily, test.used, test.ttl); err == nil {
				t.Fatal("Acquire() error = nil")
			}
		})
	}
	if err := store.Release(nilContext, "student", "lease"); err == nil {
		t.Fatal("Release(nil context) error = nil")
	}
	if err := store.Release(context.Background(), "", "lease"); err == nil {
		t.Fatal("Release(empty student) error = nil")
	}
}
