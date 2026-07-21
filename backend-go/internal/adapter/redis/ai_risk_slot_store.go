package redisadapter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	airiskapp "mathstudy/backend-go/internal/application/airisk"
)

var acquireAIRiskSlotScript = goredis.NewScript(`
redis.replicate_commands()
local active_key = KEYS[1]
local metered_key = KEYS[2]
local max_concurrency = tonumber(ARGV[1])
local daily_limit = tonumber(ARGV[2])
local used_today = tonumber(ARGV[3])
local ttl_ms = tonumber(ARGV[4])
local lease_id = ARGV[5]

local now_parts = redis.call('TIME')
local now_ms = tonumber(now_parts[1]) * 1000 + math.floor(tonumber(now_parts[2]) / 1000)
local expires_at = now_ms + ttl_ms

redis.call('ZREMRANGEBYSCORE', active_key, '-inf', now_ms)
redis.call('ZREMRANGEBYSCORE', metered_key, '-inf', now_ms)

if redis.call('ZSCORE', active_key, lease_id) ~= false then
  redis.call('ZADD', active_key, expires_at, lease_id)
  if daily_limit > 0 then
    redis.call('ZADD', metered_key, expires_at, lease_id)
  end
  redis.call('PEXPIRE', active_key, ttl_ms + 1000)
  redis.call('PEXPIRE', metered_key, ttl_ms + 1000)
  return {1, 0}
end

local active_count = redis.call('ZCARD', active_key)
if active_count >= max_concurrency then
  return {0, 2}
end

if daily_limit > 0 then
  local metered_count = redis.call('ZCARD', metered_key)
  if used_today + metered_count >= daily_limit then
    return {0, 1}
  end
end

redis.call('ZADD', active_key, expires_at, lease_id)
redis.call('PEXPIRE', active_key, ttl_ms + 1000)
if daily_limit > 0 then
  redis.call('ZADD', metered_key, expires_at, lease_id)
  redis.call('PEXPIRE', metered_key, ttl_ms + 1000)
end
return {1, 0}
`)

// AIRiskSlotStore tracks distributed per-student AI leases in Redis.
type AIRiskSlotStore struct {
	client *goredis.Client
	prefix string
}

// NewAIRiskSlotStore creates a Redis-backed AI lease store.
func NewAIRiskSlotStore(client *goredis.Client) (*AIRiskSlotStore, error) {
	if client == nil {
		return nil, errors.New("AI risk Redis client is nil")
	}
	return &AIRiskSlotStore{client: client, prefix: "msp:ai-risk:"}, nil
}

// Acquire reserves one slot while enforcing concurrency and in-flight quota capacity.
func (s *AIRiskSlotStore) Acquire(
	ctx context.Context,
	studentID string,
	leaseID string,
	maxConcurrency int,
	dailyLimit int,
	usedToday int,
	ttl time.Duration,
) (airiskapp.SlotDecision, error) {
	if ctx == nil {
		return airiskapp.SlotDecision{}, errors.New("AI risk slot context is nil")
	}
	studentID = strings.TrimSpace(studentID)
	leaseID = strings.TrimSpace(leaseID)
	if studentID == "" || leaseID == "" {
		return airiskapp.SlotDecision{}, errors.New("AI risk slot identifiers are empty")
	}
	if maxConcurrency < 1 || dailyLimit < 0 || usedToday < 0 || ttl <= 0 {
		return airiskapp.SlotDecision{}, errors.New("AI risk slot limits are invalid")
	}
	ttlMilliseconds := ttl.Milliseconds()
	if ttlMilliseconds < 1 {
		ttlMilliseconds = 1
	}
	result, err := acquireAIRiskSlotScript.Run(
		ctx,
		s.client,
		[]string{s.activeKey(studentID), s.meteredKey(studentID)},
		maxConcurrency,
		dailyLimit,
		usedToday,
		ttlMilliseconds,
		leaseID,
	).Slice()
	if err != nil {
		return airiskapp.SlotDecision{}, fmt.Errorf("acquire AI risk slot: %w", err)
	}
	if len(result) != 2 {
		return airiskapp.SlotDecision{}, fmt.Errorf("AI risk slot script returned %d values", len(result))
	}
	allowed, err := redisInt64(result[0])
	if err != nil {
		return airiskapp.SlotDecision{}, err
	}
	reason, err := redisInt64(result[1])
	if err != nil {
		return airiskapp.SlotDecision{}, err
	}
	decision := airiskapp.SlotDecision{Allowed: allowed == 1}
	if reason == 1 {
		decision.Reason = "quota"
	}
	if reason == 2 {
		decision.Reason = "concurrency"
	}
	return decision, nil
}

// Release removes a lease idempotently from active and metered sets.
func (s *AIRiskSlotStore) Release(ctx context.Context, studentID, leaseID string) error {
	if ctx == nil {
		return errors.New("AI risk release context is nil")
	}
	studentID = strings.TrimSpace(studentID)
	leaseID = strings.TrimSpace(leaseID)
	if studentID == "" || leaseID == "" {
		return errors.New("AI risk release identifiers are empty")
	}
	if _, err := s.client.Pipelined(ctx, func(pipe goredis.Pipeliner) error {
		pipe.ZRem(ctx, s.activeKey(studentID), leaseID)
		pipe.ZRem(ctx, s.meteredKey(studentID), leaseID)
		return nil
	}); err != nil {
		return fmt.Errorf("release AI risk slot: %w", err)
	}
	return nil
}

func (s *AIRiskSlotStore) activeKey(studentID string) string {
	return s.prefix + "active:" + studentID
}

func (s *AIRiskSlotStore) meteredKey(studentID string) string {
	return s.prefix + "metered:" + studentID
}

func redisInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case string:
		var parsed int64
		if _, err := fmt.Sscan(typed, &parsed); err != nil {
			return 0, fmt.Errorf("parse AI risk Redis integer %q: %w", typed, err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("AI risk Redis value has type %T", value)
	}
}
