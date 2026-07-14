package auth

import (
	"context"
	"log/slog"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"mathstudy/backend-go/internal/platform/ratelimit"
)

const (
	loginFailPrefix          = "msp:login_fail:"
	loginLockPrefix          = "msp:login_lock:"
	defaultMaxLocalLoginKeys = 500
)

// LoginLimiter tracks failed login attempts and temporarily locks accounts.
type LoginLimiter struct {
	client      *goredis.Client
	maxAttempts int
	lockout     time.Duration
	logger      *slog.Logger

	mu           sync.Mutex
	localCounts  map[string][]time.Time
	maxLocalKeys int
	now          func() time.Time
}

// NewLoginLimiter creates a Redis-backed limiter with an in-memory fallback.
func NewLoginLimiter(client *goredis.Client, maxAttempts int, lockout time.Duration, logger *slog.Logger) *LoginLimiter {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	if lockout <= 0 {
		lockout = 15 * time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &LoginLimiter{
		client:       client,
		maxAttempts:  maxAttempts,
		lockout:      lockout,
		logger:       logger,
		localCounts:  make(map[string][]time.Time),
		maxLocalKeys: defaultMaxLocalLoginKeys,
		now:          func() time.Time { return time.Now().UTC() },
	}
}

// IsLocked reports whether the username is temporarily locked.
func (l *LoginLimiter) IsLocked(ctx context.Context, username string) bool {
	if l == nil {
		return false
	}
	if l.client != nil {
		locked, err := l.client.Exists(ctx, loginLockPrefix+username).Result()
		if err == nil {
			return locked > 0
		}
		l.logger.Warn("redis login lock check failed, using local fallback", "error", err)
	}
	return l.localLocked(username, l.now())
}

// RecordFailure increments failed attempts and locks the username when the limit is reached.
func (l *LoginLimiter) RecordFailure(ctx context.Context, username string) {
	if l == nil {
		return
	}
	if l.client != nil {
		failKey := loginFailPrefix + username
		count, err := ratelimit.IncrementWithRefreshedExpiry(ctx, l.client, failKey, l.lockout)
		if err == nil {
			if count >= int64(l.maxAttempts) {
				if err := l.client.Set(ctx, loginLockPrefix+username, "1", l.lockout).Err(); err != nil {
					l.logger.Warn("redis login lock set failed", "username", username, "error", err)
				}
			}
			return
		}
		l.logger.Warn("redis login failure record failed, using local fallback", "error", err)
	}
	l.localRecordFailure(username, l.now())
}

// Clear removes failed login state after successful authentication or password reset.
func (l *LoginLimiter) Clear(ctx context.Context, username string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	delete(l.localCounts, username)
	l.mu.Unlock()
	if l.client != nil {
		if err := l.client.Del(ctx, loginFailPrefix+username, loginLockPrefix+username).Err(); err != nil {
			l.logger.Warn("redis login failure clear failed", "username", username, "error", err)
		}
	}
}

// LockoutMinutes returns the configured lockout window rounded to minutes.
func (l *LoginLimiter) LockoutMinutes() int {
	if l == nil || l.lockout <= 0 {
		return 15
	}
	return int(l.lockout / time.Minute)
}

func (l *LoginLimiter) localLocked(username string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	attempts := l.recentAttempts(username, now)
	if len(attempts) == 0 {
		delete(l.localCounts, username)
		return false
	}
	l.localCounts[username] = attempts
	return len(attempts) >= l.maxAttempts
}

func (l *LoginLimiter) localRecordFailure(username string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.localCounts[username]; !exists && len(l.localCounts) >= l.maxLocalKeys {
		l.pruneLocalCounts(now)
		if len(l.localCounts) >= l.maxLocalKeys {
			l.evictOldestLocalCount()
		}
	}
	attempts := append(l.recentAttempts(username, now), now)
	l.localCounts[username] = attempts
	if len(attempts) >= l.maxAttempts {
		l.logger.Warn("account locked after repeated login failures in local fallback", "username", username)
	}
}

func (l *LoginLimiter) recentAttempts(username string, now time.Time) []time.Time {
	attempts := l.localCounts[username]
	cutoff := now.Add(-l.lockout)
	recent := attempts[:0]
	for _, attempt := range attempts {
		if attempt.After(cutoff) {
			recent = append(recent, attempt)
		}
	}
	return recent
}

func (l *LoginLimiter) pruneLocalCounts(now time.Time) {
	for username := range l.localCounts {
		recent := l.recentAttempts(username, now)
		if len(recent) == 0 {
			delete(l.localCounts, username)
			continue
		}
		l.localCounts[username] = recent
	}
}

func (l *LoginLimiter) evictOldestLocalCount() {
	var oldestUsername string
	var oldestAttempt time.Time
	for username, attempts := range l.localCounts {
		if len(attempts) == 0 {
			delete(l.localCounts, username)
			return
		}
		lastAttempt := attempts[len(attempts)-1]
		if oldestUsername == "" || lastAttempt.Before(oldestAttempt) {
			oldestUsername = username
			oldestAttempt = lastAttempt
		}
	}
	if oldestUsername != "" {
		delete(l.localCounts, oldestUsername)
	}
}
