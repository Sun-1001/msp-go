package auth

import (
	"context"
	"log/slog"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	loginFailPrefix = "msp:login_fail:"
	loginLockPrefix = "msp:login_lock:"
)

// LoginLimiter tracks failed login attempts and temporarily locks accounts.
type LoginLimiter struct {
	client      *goredis.Client
	maxAttempts int
	lockout     time.Duration
	logger      *slog.Logger

	mu          sync.Mutex
	localCounts map[string][]time.Time
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
		client:      client,
		maxAttempts: maxAttempts,
		lockout:     lockout,
		logger:      logger,
		localCounts: make(map[string][]time.Time),
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
	return l.localLocked(username, time.Now())
}

// RecordFailure increments failed attempts and locks the username when the limit is reached.
func (l *LoginLimiter) RecordFailure(ctx context.Context, username string) {
	if l == nil {
		return
	}
	if l.client != nil {
		failKey := loginFailPrefix + username
		count, err := l.client.Incr(ctx, failKey).Result()
		if err == nil {
			_ = l.client.Expire(ctx, failKey, l.lockout).Err()
			if int(count) >= l.maxAttempts {
				if err := l.client.Set(ctx, loginLockPrefix+username, "1", l.lockout).Err(); err != nil {
					l.logger.Warn("redis login lock set failed", "username", username, "error", err)
				}
			}
			return
		}
		l.logger.Warn("redis login failure record failed, using local fallback", "error", err)
	}
	l.localRecordFailure(username, time.Now())
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
	l.localCounts[username] = attempts
	return len(attempts) >= l.maxAttempts
}

func (l *LoginLimiter) localRecordFailure(username string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

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
