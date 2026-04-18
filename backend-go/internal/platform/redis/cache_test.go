package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestCacheStringOperations(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	defer client.Close()

	cache, err := NewCache(client, "msp:test")
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	if err := cache.Set(ctx, "answer", "42", time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	value, exists, err := cache.Get(ctx, "answer")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !exists || value != "42" {
		t.Fatalf("Get() = %q, %t; want 42, true", value, exists)
	}
	if server.TTL(cache.Key("answer")) <= 0 {
		t.Fatal("expected Redis key TTL to be set")
	}

	deleted, err := cache.Delete(ctx, "answer")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("Delete() = %d, want 1", deleted)
	}
	_, exists, err = cache.Get(ctx, "answer")
	if err != nil {
		t.Fatalf("Get() after delete error = %v", err)
	}
	if exists {
		t.Fatal("Get() exists = true after delete")
	}
}

func TestCacheHashOperations(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	defer client.Close()

	cache, err := NewCache(client, "msp")
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	if changed, err := cache.HSet(ctx, "profile", "name", "Ada", "role", "student"); err != nil || changed != 2 {
		t.Fatalf("HSet() = %d, %v; want 2, nil", changed, err)
	}
	value, exists, err := cache.HGet(ctx, "profile", "name")
	if err != nil {
		t.Fatalf("HGet() error = %v", err)
	}
	if !exists || value != "Ada" {
		t.Fatalf("HGet() = %q, %t; want Ada, true", value, exists)
	}
	values, err := cache.HGetAll(ctx, "profile")
	if err != nil {
		t.Fatalf("HGetAll() error = %v", err)
	}
	if values["role"] != "student" {
		t.Fatalf("HGetAll()[role] = %q", values["role"])
	}
	deleted, err := cache.HDelete(ctx, "profile", "role")
	if err != nil {
		t.Fatalf("HDelete() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("HDelete() = %d, want 1", deleted)
	}
}

func TestNewCacheRejectsNilClient(t *testing.T) {
	if _, err := NewCache(nil, "msp"); err == nil {
		t.Fatal("NewCache(nil) error = nil, want error")
	}
}
