package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const refreshSessionPrefix = "msp:refresh_session:"

const defaultMaxLocalRefreshSessions = 500

// RefreshSessionStore tracks server-issued refresh token IDs so refresh tokens
// can be revoked and rotated instead of remaining valid until JWT expiration.
type RefreshSessionStore struct {
	client *goredis.Client
	logger *slog.Logger
	strict bool

	mu               sync.Mutex
	sessions         map[string]refreshSession
	now              func() time.Time
	maxLocalSessions int
}

type refreshSession struct {
	UserID    string
	ExpiresAt time.Time
}

// RefreshSessionStoreOption customizes refresh session persistence behavior.
type RefreshSessionStoreOption func(*RefreshSessionStore)

// WithStrictRefreshSessions disables local fallback when Redis is unavailable.
func WithStrictRefreshSessions(strict bool) RefreshSessionStoreOption {
	return func(s *RefreshSessionStore) {
		s.strict = strict
	}
}

// WithMaxLocalRefreshSessions bounds refresh state retained during Redis degradation.
func WithMaxLocalRefreshSessions(maxSessions int) RefreshSessionStoreOption {
	return func(s *RefreshSessionStore) {
		if maxSessions > 0 {
			s.maxLocalSessions = maxSessions
		}
	}
}

// NewRefreshSessionStore creates a Redis-backed refresh session store with a
// local fallback for development and degraded Redis availability.
func NewRefreshSessionStore(client *goredis.Client, logger *slog.Logger, options ...RefreshSessionStoreOption) *RefreshSessionStore {
	if logger == nil {
		logger = slog.Default()
	}
	store := &RefreshSessionStore{
		client:           client,
		logger:           logger,
		sessions:         make(map[string]refreshSession),
		now:              func() time.Time { return time.Now().UTC() },
		maxLocalSessions: defaultMaxLocalRefreshSessions,
	}
	for _, option := range options {
		if option != nil {
			option(store)
		}
	}
	return store
}

// Remember records one refresh token ID until its JWT expiration.
func (s *RefreshSessionStore) Remember(ctx context.Context, userID, jti string, expiresAt time.Time) error {
	if s == nil {
		return nil
	}
	now := s.now()
	if userID == "" || jti == "" || !expiresAt.After(now) {
		return errInvalidToken
	}
	if s.strict && s.client == nil {
		return errors.New("strict refresh session store requires redis client")
	}

	ttl := expiresAt.Sub(now)
	if s.client != nil {
		if err := s.client.Set(ctx, refreshSessionPrefix+jti, userID, ttl).Err(); err == nil {
			s.localRevoke(jti)
			return nil
		} else {
			if s.strict {
				return fmt.Errorf("remember refresh session in redis: %w", err)
			}
			s.logger.Warn("redis refresh session remember failed, using local fallback", "error", err)
		}
	}
	s.localRemember(userID, jti, expiresAt)
	return nil
}

// Consume validates and removes a refresh token ID. A consumed token cannot be
// used again, which turns refresh into one-time rotation.
func (s *RefreshSessionStore) Consume(ctx context.Context, userID, jti string) (bool, error) {
	if s == nil {
		return true, nil
	}
	if userID == "" || jti == "" {
		return false, nil
	}
	if s.strict && s.client == nil {
		return false, errors.New("strict refresh session store requires redis client")
	}

	if s.client != nil {
		value, err := s.client.GetDel(ctx, refreshSessionPrefix+jti).Result()
		switch {
		case err == nil:
			s.localRevoke(jti)
			return value == userID, nil
		case errors.Is(err, goredis.Nil):
			if s.strict {
				return false, nil
			}
			return s.localConsume(userID, jti), nil
		default:
			if s.strict {
				return false, fmt.Errorf("consume refresh session in redis: %w", err)
			}
			s.logger.Warn("redis refresh session consume failed, using local fallback", "error", err)
		}
	}
	return s.localConsume(userID, jti), nil
}

// Revoke removes a refresh token ID without requiring it to be consumed by a
// successful refresh flow.
func (s *RefreshSessionStore) Revoke(ctx context.Context, jti string) error {
	if s == nil || jti == "" {
		return nil
	}
	if s.strict && s.client == nil {
		return errors.New("strict refresh session store requires redis client")
	}
	s.localRevoke(jti)
	if s.client != nil {
		if err := s.client.Del(ctx, refreshSessionPrefix+jti).Err(); err != nil {
			if s.strict {
				return fmt.Errorf("revoke refresh session in redis: %w", err)
			}
			s.logger.Warn("redis refresh session revoke failed", "error", err)
			return err
		}
	}
	return nil
}

func (s *RefreshSessionStore) localRemember(userID, jti string, expiresAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.sessions[jti]; !exists && len(s.sessions) >= s.maxLocalSessions {
		s.pruneLocalSessionsLocked(s.now())
		if len(s.sessions) >= s.maxLocalSessions {
			s.evictEarliestLocalSessionLocked()
		}
	}
	s.sessions[jti] = refreshSession{UserID: userID, ExpiresAt: expiresAt}
}

func (s *RefreshSessionStore) localConsume(userID, jti string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[jti]
	if !ok {
		return false
	}
	delete(s.sessions, jti)
	return session.UserID == userID && session.ExpiresAt.After(s.now())
}

func (s *RefreshSessionStore) localRevoke(jti string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, jti)
}

func (s *RefreshSessionStore) pruneLocalSessionsLocked(now time.Time) {
	for jti, session := range s.sessions {
		if !session.ExpiresAt.After(now) {
			delete(s.sessions, jti)
		}
	}
}

func (s *RefreshSessionStore) evictEarliestLocalSessionLocked() {
	var earliestJTI string
	var earliestExpiry time.Time
	for jti, session := range s.sessions {
		if earliestJTI == "" || session.ExpiresAt.Before(earliestExpiry) {
			earliestJTI = jti
			earliestExpiry = session.ExpiresAt
		}
	}
	if earliestJTI != "" {
		delete(s.sessions, earliestJTI)
	}
}
