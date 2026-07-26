package redisadapter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	wechatapp "mathstudy/backend-go/internal/application/wechat"
)

const (
	wechatKeyPrefix             = "msp:wechat:"
	wechatEventProcessingPrefix = "processing:"
	wechatEventCompletedPrefix  = "done:"
	wechatTicketAvailablePrefix = "available:"
	wechatTicketReservedPrefix  = "reserved:"
	maxWechatEventReplyBytes    = 4096
)

var (
	consumeWechatTicketScript = goredis.NewScript(`
local value = redis.call('GET', KEYS[1])
if not value then
  return false
end
local available_prefix = ARGV[1]
local reservation_prefix = ARGV[2] .. ARGV[3] .. ':'
local user_id
if string.sub(value, 1, string.len(available_prefix)) == available_prefix then
  user_id = string.sub(value, string.len(available_prefix) + 1)
elseif string.sub(value, 1, string.len(reservation_prefix)) == reservation_prefix then
  return string.sub(value, string.len(reservation_prefix) + 1)
elseif string.sub(value, 1, string.len(ARGV[2])) == ARGV[2] then
  return false
else
  user_id = value
end
if not user_id or user_id == '' then
  return false
end
local ttl = redis.call('PTTL', KEYS[1])
if ttl <= 0 then
  return false
end
redis.call('PSETEX', KEYS[1], ttl, reservation_prefix .. user_id)
return user_id
`)
	deleteWechatValueIfMatchScript = goredis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`)
	claimWechatEventScript = goredis.NewScript(`
local current = redis.call('GET', KEYS[1])
if current then
  return current
end
redis.call('PSETEX', KEYS[1], ARGV[1], ARGV[2])
return 'acquired'
`)
	completeWechatEventScript = goredis.NewScript(`
if redis.call('GET', KEYS[1]) ~= ARGV[1] then
  return 0
end
redis.call('PSETEX', KEYS[1], ARGV[2], ARGV[3])
return 1
`)
)

// WechatStateStore persists callback, binding-ticket, and token state in Redis.
type WechatStateStore struct {
	client *goredis.Client
}

// NewWechatStateStore creates the shared WeChat state adapter.
func NewWechatStateStore(client *goredis.Client) (*WechatStateStore, error) {
	if client == nil {
		return nil, errors.New("wechat redis client is nil")
	}
	return &WechatStateStore{client: client}, nil
}

// StoreBindingTicket stores a ticket digest exactly once.
func (s *WechatStateStore) StoreBindingTicket(ctx context.Context, digest, userID string, ttl time.Duration) (bool, error) {
	if strings.TrimSpace(digest) == "" || strings.TrimSpace(userID) == "" || ttl <= 0 {
		return false, errors.New("invalid wechat binding ticket state")
	}
	stored, err := s.client.SetNX(ctx, wechatKeyPrefix+"bind-ticket:"+digest, wechatTicketAvailablePrefix+userID, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("set binding ticket: %w", err)
	}
	return stored, nil
}

// ConsumeBindingTicket reserves a one-time ticket for one callback event. The
// same event may replay it after a process failure; a different event may not.
func (s *WechatStateStore) ConsumeBindingTicket(ctx context.Context, digest, eventKey string) (string, bool, error) {
	if strings.TrimSpace(digest) == "" || strings.TrimSpace(eventKey) == "" {
		return "", false, errors.New("invalid wechat binding ticket digest")
	}
	value, err := consumeWechatTicketScript.Run(
		ctx,
		s.client,
		[]string{wechatKeyPrefix + "bind-ticket:" + digest},
		wechatTicketAvailablePrefix,
		wechatTicketReservedPrefix,
		eventKey,
	).Text()
	if errors.Is(err, goredis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("consume binding ticket: %w", err)
	}
	return value, true, nil
}

// ClaimEvent acquires a short processing lease or reads an existing event state.
func (s *WechatStateStore) ClaimEvent(ctx context.Context, digest, owner string, ttl time.Duration) (wechatapp.EventClaim, error) {
	if strings.TrimSpace(digest) == "" || strings.TrimSpace(owner) == "" || ttl <= 0 {
		return wechatapp.EventClaim{}, errors.New("invalid wechat callback claim")
	}
	state, err := claimWechatEventScript.Run(
		ctx,
		s.client,
		[]string{wechatKeyPrefix + "event:" + digest},
		redisTTLMilliseconds(ttl),
		wechatEventProcessingPrefix+owner,
	).Text()
	if err != nil {
		return wechatapp.EventClaim{}, fmt.Errorf("claim callback event: %w", err)
	}
	switch {
	case state == "acquired":
		return wechatapp.EventClaim{Acquired: true}, nil
	case strings.HasPrefix(state, wechatEventProcessingPrefix):
		return wechatapp.EventClaim{}, nil
	case strings.HasPrefix(state, wechatEventCompletedPrefix):
		return wechatapp.EventClaim{Completed: true, Reply: strings.TrimPrefix(state, wechatEventCompletedPrefix)}, nil
	default:
		return wechatapp.EventClaim{}, errors.New("invalid wechat callback state")
	}
}

// CompleteEvent replaces the owner's processing lease with a replayable result.
func (s *WechatStateStore) CompleteEvent(ctx context.Context, digest, owner, reply string, ttl time.Duration) (bool, error) {
	if strings.TrimSpace(digest) == "" || strings.TrimSpace(owner) == "" || len(reply) > maxWechatEventReplyBytes || ttl <= 0 {
		return false, errors.New("invalid completed wechat callback state")
	}
	completed, err := completeWechatEventScript.Run(
		ctx,
		s.client,
		[]string{wechatKeyPrefix + "event:" + digest},
		wechatEventProcessingPrefix+owner,
		redisTTLMilliseconds(ttl),
		wechatEventCompletedPrefix+reply,
	).Int64()
	if err != nil {
		return false, fmt.Errorf("complete callback event: %w", err)
	}
	return completed == 1, nil
}

// ReleaseEvent removes only the current owner's failed processing lease.
func (s *WechatStateStore) ReleaseEvent(ctx context.Context, digest, owner string) error {
	if strings.TrimSpace(digest) == "" || strings.TrimSpace(owner) == "" {
		return errors.New("invalid wechat callback claim digest")
	}
	if err := deleteWechatValueIfMatchScript.Run(
		ctx,
		s.client,
		[]string{wechatKeyPrefix + "event:" + digest},
		wechatEventProcessingPrefix+owner,
	).Err(); err != nil && !errors.Is(err, goredis.Nil) {
		return fmt.Errorf("release callback event: %w", err)
	}
	return nil
}

// GetAccessToken reads the stable access token for one AppID.
func (s *WechatStateStore) GetAccessToken(ctx context.Context, appID string) (string, bool, error) {
	value, err := s.client.Get(ctx, accessTokenKey(appID)).Result()
	if errors.Is(err, goredis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get access token: %w", err)
	}
	return value, true, nil
}

// SetAccessToken caches a stable access token with a bounded lifetime.
func (s *WechatStateStore) SetAccessToken(ctx context.Context, appID, token string, ttl time.Duration) error {
	if strings.TrimSpace(appID) == "" || strings.TrimSpace(token) == "" || ttl <= 0 {
		return errors.New("invalid wechat access token state")
	}
	if err := s.client.Set(ctx, accessTokenKey(appID), token, ttl).Err(); err != nil {
		return fmt.Errorf("set access token: %w", err)
	}
	return nil
}

// DeleteAccessTokenIfMatch invalidates only the token observed by the caller.
func (s *WechatStateStore) DeleteAccessTokenIfMatch(ctx context.Context, appID, token string) error {
	if strings.TrimSpace(appID) == "" || strings.TrimSpace(token) == "" {
		return errors.New("invalid wechat access token invalidation")
	}
	if err := deleteWechatValueIfMatchScript.Run(ctx, s.client, []string{accessTokenKey(appID)}, token).Err(); err != nil && !errors.Is(err, goredis.Nil) {
		return fmt.Errorf("invalidate access token: %w", err)
	}
	return nil
}

// AcquireAccessTokenLock attempts to become the distributed refresh owner.
func (s *WechatStateStore) AcquireAccessTokenLock(ctx context.Context, appID, owner string, ttl time.Duration) (bool, error) {
	if strings.TrimSpace(appID) == "" || strings.TrimSpace(owner) == "" || ttl <= 0 {
		return false, errors.New("invalid wechat access token lock")
	}
	acquired, err := s.client.SetNX(ctx, accessTokenLockKey(appID), owner, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("acquire access token lock: %w", err)
	}
	return acquired, nil
}

// ReleaseAccessTokenLock releases the lock only for its current owner.
func (s *WechatStateStore) ReleaseAccessTokenLock(ctx context.Context, appID, owner string) error {
	if strings.TrimSpace(appID) == "" || strings.TrimSpace(owner) == "" {
		return errors.New("invalid wechat access token lock release")
	}
	if err := deleteWechatValueIfMatchScript.Run(ctx, s.client, []string{accessTokenLockKey(appID)}, owner).Err(); err != nil && !errors.Is(err, goredis.Nil) {
		return fmt.Errorf("release access token lock: %w", err)
	}
	return nil
}

func accessTokenKey(appID string) string {
	return wechatKeyPrefix + "access-token:" + strings.TrimSpace(appID)
}

func accessTokenLockKey(appID string) string {
	return wechatKeyPrefix + "access-token-lock:" + strings.TrimSpace(appID)
}

func redisTTLMilliseconds(ttl time.Duration) int64 {
	milliseconds := ttl.Milliseconds()
	if milliseconds < 1 {
		return 1
	}
	return milliseconds
}
