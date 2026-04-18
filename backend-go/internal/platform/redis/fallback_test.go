package redis

import "testing"

func TestInMemoryFallbackCacheEvictsLeastRecentlyUsed(t *testing.T) {
	cache, err := NewInMemoryFallbackCache(2)
	if err != nil {
		t.Fatalf("NewInMemoryFallbackCache() error = %v", err)
	}

	cache.Set("a", "1")
	cache.Set("b", "2")
	if value, exists := cache.Get("a"); !exists || value != "1" {
		t.Fatalf("Get(a) = %q, %t; want 1, true", value, exists)
	}
	cache.Set("c", "3")

	if cache.Exists("b") {
		t.Fatal("Exists(b) = true, want false after LRU eviction")
	}
	if !cache.Exists("a") || !cache.Exists("c") {
		t.Fatal("expected a and c to remain in cache")
	}
}

func TestInMemoryFallbackCacheDelete(t *testing.T) {
	cache, err := NewInMemoryFallbackCache(1)
	if err != nil {
		t.Fatalf("NewInMemoryFallbackCache() error = %v", err)
	}

	cache.Set("a", "1")
	cache.Delete("a")

	if _, exists := cache.Get("a"); exists {
		t.Fatal("Get(a) exists = true after delete")
	}
}

func TestNewInMemoryFallbackCacheRejectsInvalidSize(t *testing.T) {
	if _, err := NewInMemoryFallbackCache(0); err == nil {
		t.Fatal("NewInMemoryFallbackCache(0) error = nil, want error")
	}
}
