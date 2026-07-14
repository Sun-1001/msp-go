package ratelimit

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	maxLocalShards     = 32
	redisWarningPeriod = 30 * time.Second
)

var incrementWithExpiryScript = goredis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if ARGV[2] == "1" or redis.call("PTTL", KEYS[1]) < 0 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return count
`)

// Limiter applies a Redis-backed fixed window with a bounded, sharded local
// sliding-window fallback when Redis is unavailable.
type Limiter struct {
	client *goredis.Client
	prefix string
	limit  int64
	window time.Duration
	local  *localLimiter
	logger *slog.Logger

	lastRedisWarning atomic.Int64
}

// New creates a distributed limiter. A nil Redis client uses only the local fallback.
func New(client *goredis.Client, prefix string, limit int, window time.Duration, maxLocalKeys int, logger *slog.Logger) (*Limiter, error) {
	prefix = strings.Trim(strings.TrimSpace(prefix), ":")
	if prefix == "" {
		return nil, errors.New("rate limit key prefix is empty")
	}
	if limit <= 0 {
		return nil, errors.New("rate limit must be greater than 0")
	}
	if window <= 0 {
		return nil, errors.New("rate limit window must be greater than 0")
	}
	if maxLocalKeys <= 0 {
		return nil, errors.New("rate limit local key capacity must be greater than 0")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Limiter{
		client: client,
		prefix: prefix + ":",
		limit:  int64(limit),
		window: window,
		local:  newLocalLimiter(limit, window, maxLocalKeys),
		logger: logger,
	}, nil
}

// Allow records one hit and reports whether it remains within the configured limit.
func (l *Limiter) Allow(ctx context.Context, key string) bool {
	if l == nil {
		return true
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = "unknown"
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if l.client != nil {
		count, err := IncrementWithExpiry(ctx, l.client, l.prefix+key, l.window)
		if err == nil {
			l.local.delete(key)
			return count <= l.limit
		}
		l.warnRedisFallback(err)
	}
	return l.local.allow(key)
}

// IncrementWithExpiry atomically increments a Redis counter and ensures it has a TTL.
// It also repairs counters left without a TTL by an interrupted legacy INCR/EXPIRE flow.
func IncrementWithExpiry(ctx context.Context, client *goredis.Client, key string, ttl time.Duration) (int64, error) {
	return incrementWithExpiry(ctx, client, key, ttl, false)
}

// IncrementWithRefreshedExpiry atomically increments a counter and restarts its TTL.
func IncrementWithRefreshedExpiry(ctx context.Context, client *goredis.Client, key string, ttl time.Duration) (int64, error) {
	return incrementWithExpiry(ctx, client, key, ttl, true)
}

func incrementWithExpiry(ctx context.Context, client *goredis.Client, key string, ttl time.Duration, refreshTTL bool) (int64, error) {
	if client == nil {
		return 0, errors.New("redis client is nil")
	}
	if ctx == nil {
		return 0, errors.New("redis counter context is nil")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return 0, errors.New("redis counter key is empty")
	}
	if ttl <= 0 {
		return 0, errors.New("redis counter TTL must be greater than 0")
	}
	ttlMilliseconds := ttl.Milliseconds()
	if ttlMilliseconds < 1 {
		ttlMilliseconds = 1
	}
	refresh := 0
	if refreshTTL {
		refresh = 1
	}
	count, err := incrementWithExpiryScript.Run(ctx, client, []string{key}, ttlMilliseconds, refresh).Int64()
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (l *Limiter) warnRedisFallback(err error) {
	now := time.Now().UnixNano()
	last := l.lastRedisWarning.Load()
	if last != 0 && time.Duration(now-last) < redisWarningPeriod {
		return
	}
	if l.lastRedisWarning.CompareAndSwap(last, now) {
		l.logger.Warn("redis rate limit failed, using local fallback", "error", err)
	}
}

type localLimiter struct {
	limit  int
	window time.Duration
	now    func() time.Time
	shards []localShard
}

type localShard struct {
	mu      sync.Mutex
	maxKeys int
	entries map[string]localEntry
}

type localEntry struct {
	hits []time.Time
}

func newLocalLimiter(limit int, window time.Duration, maxKeys int) *localLimiter {
	shardCount := 1
	for shardCount < maxLocalShards && shardCount*2 <= maxKeys {
		shardCount *= 2
	}
	maxKeysPerShard := maxKeys / shardCount
	shards := make([]localShard, shardCount)
	for index := range shards {
		shards[index] = localShard{
			maxKeys: maxKeysPerShard,
			entries: make(map[string]localEntry, maxKeysPerShard),
		}
	}
	return &localLimiter{
		limit:  limit,
		window: window,
		now:    func() time.Time { return time.Now().UTC() },
		shards: shards,
	}
}

func (l *localLimiter) allow(key string) bool {
	now := l.now()
	cutoff := now.Add(-l.window)
	shard := l.shard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	entry, exists := shard.entries[key]
	entry.hits = recentHits(entry.hits, cutoff)
	if len(entry.hits) >= l.limit {
		shard.entries[key] = entry
		return false
	}
	if !exists && len(shard.entries) >= shard.maxKeys {
		pruneExpiredEntries(shard.entries, cutoff)
		if len(shard.entries) >= shard.maxKeys {
			evictOldestEntry(shard.entries)
		}
	}
	entry.hits = append(entry.hits, now)
	shard.entries[key] = entry
	return true
}

func (l *localLimiter) delete(key string) {
	shard := l.shard(key)
	shard.mu.Lock()
	delete(shard.entries, key)
	shard.mu.Unlock()
}

func (l *localLimiter) shard(key string) *localShard {
	// #nosec G115 -- construction guarantees 1..maxLocalShards power-of-two shards.
	return &l.shards[fnv1a(key)&uint64(len(l.shards)-1)]
}

func recentHits(hits []time.Time, cutoff time.Time) []time.Time {
	firstRecent := 0
	for firstRecent < len(hits) && !hits[firstRecent].After(cutoff) {
		firstRecent++
	}
	return hits[firstRecent:]
}

func pruneExpiredEntries(entries map[string]localEntry, cutoff time.Time) {
	for key, entry := range entries {
		entry.hits = recentHits(entry.hits, cutoff)
		if len(entry.hits) == 0 {
			delete(entries, key)
			continue
		}
		entries[key] = entry
	}
}

func evictOldestEntry(entries map[string]localEntry) {
	var oldestKey string
	var oldest time.Time
	for key, entry := range entries {
		if len(entry.hits) == 0 {
			delete(entries, key)
			return
		}
		lastHit := entry.hits[len(entry.hits)-1]
		if oldestKey == "" || lastHit.Before(oldest) {
			oldestKey = key
			oldest = lastHit
		}
	}
	if oldestKey != "" {
		delete(entries, oldestKey)
	}
}

func fnv1a(value string) uint64 {
	const (
		offset = uint64(14695981039346656037)
		prime  = uint64(1099511628211)
	)
	hash := offset
	for index := 0; index < len(value); index++ {
		hash ^= uint64(value[index])
		hash *= prime
	}
	return hash
}
