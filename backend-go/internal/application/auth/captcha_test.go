package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image/png"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestSliderCaptchaChallengeVerifyAndConsumeProof(t *testing.T) {
	manager := newTestCaptchaManager(t, nil, false)
	challenge, err := manager.NewChallenge(context.Background(), "ip:127.0.0.1")
	if err != nil {
		t.Fatalf("NewChallenge() error = %v", err)
	}
	assertCaptchaImage(t, challenge.BackgroundImage, captchaWidth, captchaHeight)
	assertCaptchaImage(t, challenge.PieceImage, captchaPieceSize, captchaPieceSize)
	if challenge.PieceY < 0 || challenge.ExpiresIn != 120 {
		t.Fatalf("challenge metadata = %#v", challenge)
	}

	manager.mu.Lock()
	expected := manager.challenges[challenge.ID].ExpectedX
	manager.mu.Unlock()
	proof, ok, err := manager.Verify(context.Background(), challenge.ID, expected+manager.config.Tolerance, "ip:127.0.0.1")
	if err != nil || !ok || proof == "" {
		t.Fatalf("Verify() = %q, %t, %v", proof, ok, err)
	}
	valid, err := manager.ConsumeProof(context.Background(), proof, "ip:127.0.0.1")
	if err != nil || !valid {
		t.Fatalf("ConsumeProof() = %t, %v", valid, err)
	}
	valid, err = manager.ConsumeProof(context.Background(), proof, "ip:127.0.0.1")
	if err != nil || valid {
		t.Fatalf("replayed ConsumeProof() = %t, %v", valid, err)
	}
}

func TestSliderCaptchaRejectsWrongPositionClientAndExpiredState(t *testing.T) {
	manager := newTestCaptchaManager(t, nil, false)
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }

	wrongPosition, err := manager.NewChallenge(context.Background(), "client-a")
	if err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	expected := manager.challenges[wrongPosition.ID].ExpectedX
	manager.mu.Unlock()
	if proof, ok, err := manager.Verify(context.Background(), wrongPosition.ID, expected+manager.config.Tolerance+1, "client-a"); err != nil || ok || proof != "" {
		t.Fatalf("wrong position Verify() = %q, %t, %v", proof, ok, err)
	}
	if _, ok, _ := manager.Verify(context.Background(), wrongPosition.ID, expected, "client-a"); ok {
		t.Fatal("consumed challenge was accepted on retry")
	}

	wrongClient, err := manager.NewChallenge(context.Background(), "client-a")
	if err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	expected = manager.challenges[wrongClient.ID].ExpectedX
	manager.mu.Unlock()
	if _, ok, _ := manager.Verify(context.Background(), wrongClient.ID, expected, "client-b"); ok {
		t.Fatal("challenge was accepted for a different client")
	}

	expired, err := manager.NewChallenge(context.Background(), "client-a")
	if err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	expected = manager.challenges[expired.ID].ExpectedX
	manager.mu.Unlock()
	now = now.Add(manager.config.ChallengeTTL + time.Second)
	if _, ok, _ := manager.Verify(context.Background(), expired.ID, expected, "client-a"); ok {
		t.Fatal("expired challenge was accepted")
	}
}

func TestSliderCaptchaRateLimitsChallengeIssuance(t *testing.T) {
	manager := newTestCaptchaManager(t, nil, false)
	manager.config.IssueLimit = 2
	for i := 0; i < 2; i++ {
		if _, err := manager.NewChallenge(context.Background(), "client-a"); err != nil {
			t.Fatalf("NewChallenge(%d) error = %v", i, err)
		}
	}
	if _, err := manager.NewChallenge(context.Background(), "client-a"); !errorsIs(err, ErrCaptchaRateLimited) {
		t.Fatalf("rate-limited NewChallenge() error = %v", err)
	}
	if _, err := manager.NewChallenge(context.Background(), "client-b"); err != nil {
		t.Fatalf("independent client NewChallenge() error = %v", err)
	}
}

func TestSliderCaptchaUsesRedisForSharedOneTimeState(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	first := newTestCaptchaManager(t, client, true)
	second := newTestCaptchaManager(t, client, true)

	challenge, err := first.NewChallenge(context.Background(), "client-a")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := client.Get(context.Background(), captchaChallengePrefix+challenge.ID).Bytes()
	if err != nil {
		t.Fatalf("read shared challenge: %v", err)
	}
	var state captchaChallengeState
	if err := json.Unmarshal(payload, &state); err != nil {
		t.Fatalf("decode shared challenge: %v", err)
	}
	proof, ok, err := second.Verify(context.Background(), challenge.ID, state.ExpectedX, "client-a")
	if err != nil || !ok {
		t.Fatalf("shared Verify() = %q, %t, %v", proof, ok, err)
	}
	valid, err := first.ConsumeProof(context.Background(), proof, "client-a")
	if err != nil || !valid {
		t.Fatalf("shared ConsumeProof() = %t, %v", valid, err)
	}
	valid, err = first.ConsumeProof(context.Background(), proof, "client-a")
	if err != nil || valid {
		t.Fatalf("replayed shared ConsumeProof() = %t, %v", valid, err)
	}
}

func TestSliderCaptchaRedisHotPathsUseOneCommandEach(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	commandCounter := &redisCommandCounter{}
	client.AddHook(commandCounter)
	manager := newTestCaptchaManager(t, client, true)
	ctx := context.Background()

	if err := issueAndStoreCaptchaScript.Load(ctx, client).Err(); err != nil {
		t.Fatalf("load issuance script: %v", err)
	}
	beforeIssue := commandCounter.commands.Load()
	challenge, err := manager.NewChallenge(ctx, "client-a")
	if err != nil {
		t.Fatal(err)
	}
	if commands := commandCounter.commands.Load() - beforeIssue; commands != 1 {
		t.Fatalf("redis issuance commands = %d, want 1", commands)
	}

	payload, err := server.Get(captchaChallengePrefix + challenge.ID)
	if err != nil {
		t.Fatalf("read challenge state: %v", err)
	}
	var state captchaChallengeState
	if err := json.Unmarshal([]byte(payload), &state); err != nil {
		t.Fatalf("decode challenge state: %v", err)
	}
	if err := verifyAndStoreCaptchaProofScript.Load(ctx, client).Err(); err != nil {
		t.Fatalf("load verification script: %v", err)
	}
	beforeVerify := commandCounter.commands.Load()
	proof, ok, err := manager.Verify(ctx, challenge.ID, state.ExpectedX, "client-a")
	if err != nil || !ok || proof == "" {
		t.Fatalf("Verify() = %q, %t, %v", proof, ok, err)
	}
	if commands := commandCounter.commands.Load() - beforeVerify; commands != 1 {
		t.Fatalf("redis verification commands = %d, want 1", commands)
	}
}

func TestSliderCaptchaRedisRateLimitDoesNotStoreRejectedChallenge(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	manager := newTestCaptchaManager(t, client, true)
	manager.config.IssueLimit = 1

	if _, err := manager.NewChallenge(context.Background(), "client-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.NewChallenge(context.Background(), "client-a"); !errors.Is(err, ErrCaptchaRateLimited) {
		t.Fatalf("rate-limited NewChallenge() error = %v", err)
	}
	challengeKeys := 0
	for _, key := range server.Keys() {
		if strings.HasPrefix(key, captchaChallengePrefix) {
			challengeKeys++
		}
	}
	if challengeKeys != 1 {
		t.Fatalf("stored challenge keys = %d, want 1", challengeKeys)
	}
	if ttl := server.TTL(captchaIssuePrefix + "client-a"); ttl <= 0 {
		t.Fatalf("captcha issue counter TTL = %s", ttl)
	}
}

func TestSliderCaptchaRedisRejectsAndConsumesWrongPosition(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	manager := newTestCaptchaManager(t, client, true)
	challenge, err := manager.NewChallenge(context.Background(), "client-a")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := server.Get(captchaChallengePrefix + challenge.ID)
	if err != nil {
		t.Fatal(err)
	}
	var state captchaChallengeState
	if err := json.Unmarshal([]byte(payload), &state); err != nil {
		t.Fatal(err)
	}

	proof, ok, err := manager.Verify(context.Background(), challenge.ID, state.ExpectedX+manager.config.Tolerance+1, "client-a")
	if err != nil || ok || proof != "" {
		t.Fatalf("wrong-position Verify() = %q, %t, %v", proof, ok, err)
	}
	if server.Exists(captchaChallengePrefix + challenge.ID) {
		t.Fatal("wrong-position challenge was not consumed")
	}
	for _, key := range server.Keys() {
		if strings.HasPrefix(key, captchaProofPrefix) {
			t.Fatalf("wrong-position verification stored proof %q", key)
		}
	}
	if _, ok, err := manager.Verify(context.Background(), challenge.ID, state.ExpectedX, "client-a"); err != nil || ok {
		t.Fatalf("retried Verify() accepted consumed challenge: ok=%t, err=%v", ok, err)
	}
}

func TestSliderCaptchaRedisRejectsExpiredApplicationState(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	manager := newTestCaptchaManager(t, client, true)
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	challenge, err := manager.NewChallenge(context.Background(), "client-a")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := server.Get(captchaChallengePrefix + challenge.ID)
	if err != nil {
		t.Fatal(err)
	}
	var state captchaChallengeState
	if err := json.Unmarshal([]byte(payload), &state); err != nil {
		t.Fatal(err)
	}
	manager.now = func() time.Time { return now.Add(manager.config.ChallengeTTL + time.Second) }

	if _, ok, err := manager.Verify(context.Background(), challenge.ID, state.ExpectedX, "client-a"); err != nil || ok {
		t.Fatalf("expired Verify() accepted challenge: ok=%t, err=%v", ok, err)
	}
}

func TestSliderCaptchaRedisAcceptsLegacyJSONChallenge(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	manager := newTestCaptchaManager(t, client, true)
	legacyState := captchaChallengeState{
		ClientKey: "client-a",
		ExpectedX: 123,
		ExpiresAt: time.Now().UTC().Add(manager.config.ChallengeTTL),
	}
	payload, err := json.Marshal(legacyState)
	if err != nil {
		t.Fatal(err)
	}
	server.Set(captchaChallengePrefix+"legacy", string(payload))
	server.SetTTL(captchaChallengePrefix+"legacy", manager.config.ChallengeTTL)

	proof, ok, err := manager.Verify(context.Background(), "legacy", legacyState.ExpectedX, "client-a")
	if err != nil || !ok || proof == "" {
		t.Fatalf("legacy Verify() = %q, %t, %v", proof, ok, err)
	}
}

func TestSliderCaptchaFallsBackLocallyWhenRedisIsUnavailable(t *testing.T) {
	client := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = client.Close() })
	client.AddHook(failingRedisHook{})
	manager := newTestCaptchaManager(t, client, false)
	manager.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	challenge, err := manager.NewChallenge(ctx, "client-a")
	if err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	expected := manager.challenges[challenge.ID].ExpectedX
	manager.mu.Unlock()
	proof, ok, err := manager.Verify(ctx, challenge.ID, expected, "client-a")
	if err != nil || !ok || proof == "" {
		t.Fatalf("fallback Verify() = %q, %t, %v", proof, ok, err)
	}
	valid, err := manager.ConsumeProof(ctx, proof, "client-a")
	if err != nil || !valid {
		t.Fatalf("fallback ConsumeProof() = %t, %v", valid, err)
	}
}

func TestSliderCaptchaConfigurationAndLocalCapacity(t *testing.T) {
	base := SliderCaptchaConfig{
		ChallengeTTL: time.Minute,
		ProofTTL:     time.Minute,
		IssueWindow:  time.Minute,
		Tolerance:    5,
		IssueLimit:   10,
		MaxLocalSize: 1,
	}
	if _, err := NewSliderCaptchaManager(nil, nil, SliderCaptchaConfig{}); err == nil {
		t.Fatal("NewSliderCaptchaManager() accepted invalid config")
	}
	strict := base
	strict.Strict = true
	if _, err := NewSliderCaptchaManager(nil, nil, strict); err == nil {
		t.Fatal("NewSliderCaptchaManager() accepted strict mode without redis")
	}
	manager, err := NewSliderCaptchaManager(nil, nil, base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.NewChallenge(context.Background(), "client-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.NewChallenge(context.Background(), "client-a"); !errorsIs(err, ErrCaptchaUnavailable) {
		t.Fatalf("capacity NewChallenge() error = %v", err)
	}
}

func TestSliderCaptchaBoundsLocalIssueClients(t *testing.T) {
	manager := newTestCaptchaManager(t, nil, false)
	manager.config.MaxLocalSize = 1
	if _, err := manager.NewChallenge(context.Background(), "client-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.NewChallenge(context.Background(), "client-b"); !errorsIs(err, ErrCaptchaRateLimited) {
		t.Fatalf("bounded client NewChallenge() error = %v", err)
	}
	if len(manager.issues) != 1 {
		t.Fatalf("local issue client count = %d, want 1", len(manager.issues))
	}
}

func TestSecureIntAndColorShiftBounds(t *testing.T) {
	if _, err := secureInt(0); err == nil {
		t.Fatal("secureInt(0) error = nil")
	}
	if strconv.IntSize == 64 {
		tooLarge, err := strconv.Atoi("4294967297")
		if err != nil {
			t.Fatalf("parse oversized secureInt bound: %v", err)
		}
		if _, err := secureInt(tooLarge); err == nil {
			t.Fatal("secureInt() accepted a bound larger than its sampling range")
		}
	}
	if got := addColorShift(24, 69, 3); got != 47 {
		t.Fatalf("addColorShift() = %d, want 47", got)
	}
	if got := addColorShift(250, 69, 2); got != 255 {
		t.Fatalf("saturated addColorShift() = %d, want 255", got)
	}
	if got := addColorShift(24, -1, 3); got != 24 {
		t.Fatalf("negative addColorShift() = %d, want 24", got)
	}
	if got := addColorShift(24, 1, 0); got != 24 {
		t.Fatalf("zero-divisor addColorShift() = %d, want 24", got)
	}
}

func newTestCaptchaManager(t *testing.T, client *goredis.Client, strict bool) *SliderCaptchaManager {
	t.Helper()
	manager, err := NewSliderCaptchaManager(client, nil, SliderCaptchaConfig{
		ChallengeTTL: 2 * time.Minute,
		ProofTTL:     90 * time.Second,
		IssueWindow:  time.Minute,
		Tolerance:    5,
		IssueLimit:   10,
		MaxLocalSize: 100,
		Strict:       strict,
	})
	if err != nil {
		t.Fatalf("NewSliderCaptchaManager() error = %v", err)
	}
	return manager
}

func assertCaptchaImage(t *testing.T, dataURI string, width, height int) {
	t.Helper()
	encoded := strings.TrimPrefix(dataURI, "data:image/png;base64,")
	if encoded == dataURI {
		t.Fatalf("image is not a PNG data URI")
	}
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode image: %v", err)
	}
	image, err := png.Decode(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	if image.Bounds().Dx() != width || image.Bounds().Dy() != height {
		t.Fatalf("image dimensions = %dx%d", image.Bounds().Dx(), image.Bounds().Dy())
	}
}

func errorsIs(err, target error) bool {
	return errors.Is(err, target)
}

type redisCommandCounter struct {
	commands atomic.Int64
}

func (h *redisCommandCounter) DialHook(next goredis.DialHook) goredis.DialHook {
	return next
}

func (h *redisCommandCounter) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, command goredis.Cmder) error {
		h.commands.Add(1)
		return next(ctx, command)
	}
}

func (h *redisCommandCounter) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return func(ctx context.Context, commands []goredis.Cmder) error {
		h.commands.Add(1)
		return next(ctx, commands)
	}
}

type failingRedisHook struct{}

func (failingRedisHook) DialHook(next goredis.DialHook) goredis.DialHook {
	return next
}

func (failingRedisHook) ProcessHook(goredis.ProcessHook) goredis.ProcessHook {
	return func(context.Context, goredis.Cmder) error {
		return errors.New("redis unavailable")
	}
}

func (failingRedisHook) ProcessPipelineHook(goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return func(context.Context, []goredis.Cmder) error {
		return errors.New("redis unavailable")
	}
}
