package redis

import (
	"context"
	"errors"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Cache wraps Redis string and hash operations with a stable key prefix.
type Cache struct {
	client *goredis.Client
	prefix string
}

// NewCache creates a prefixed Redis cache.
func NewCache(client *goredis.Client, prefix string) (Cache, error) {
	if client == nil {
		return Cache{}, errors.New("redis client is nil")
	}
	prefix = strings.Trim(strings.TrimSpace(prefix), ":")
	if prefix == "" {
		prefix = "msp"
	}
	return Cache{client: client, prefix: prefix}, nil
}

// Key returns the physical Redis key for a logical cache key.
func (c Cache) Key(key string) string {
	key = strings.TrimLeft(strings.TrimSpace(key), ":")
	if key == "" {
		return c.prefix
	}
	return c.prefix + ":" + key
}

// Get reads a string value and reports whether the key exists.
func (c Cache) Get(ctx context.Context, key string) (string, bool, error) {
	value, err := c.client.Get(ctx, c.Key(key)).Result()
	if errors.Is(err, goredis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

// Set writes a string value with an optional TTL. A zero TTL means no expiration.
func (c Cache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.client.Set(ctx, c.Key(key), value, ttl).Err()
}

// Delete removes one logical key and returns the number of deleted keys.
func (c Cache) Delete(ctx context.Context, key string) (int64, error) {
	return c.client.Del(ctx, c.Key(key)).Result()
}

// Exists checks whether a logical key exists.
func (c Cache) Exists(ctx context.Context, key string) (bool, error) {
	count, err := c.client.Exists(ctx, c.Key(key)).Result()
	return count > 0, err
}

// HGet reads one field from a Redis hash.
func (c Cache) HGet(ctx context.Context, name string, field string) (string, bool, error) {
	value, err := c.client.HGet(ctx, c.Key(name), field).Result()
	if errors.Is(err, goredis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

// HSet writes one or more field/value pairs to a Redis hash.
func (c Cache) HSet(ctx context.Context, name string, values ...any) (int64, error) {
	return c.client.HSet(ctx, c.Key(name), values...).Result()
}

// HGetAll reads all fields from a Redis hash.
func (c Cache) HGetAll(ctx context.Context, name string) (map[string]string, error) {
	return c.client.HGetAll(ctx, c.Key(name)).Result()
}

// HDelete removes fields from a Redis hash.
func (c Cache) HDelete(ctx context.Context, name string, fields ...string) (int64, error) {
	return c.client.HDel(ctx, c.Key(name), fields...).Result()
}
