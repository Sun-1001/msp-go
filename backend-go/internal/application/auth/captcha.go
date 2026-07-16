package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"mathstudy/backend-go/internal/platform/securerand"
)

const (
	captchaChallengePrefix = "msp:login_captcha:challenge:"
	captchaProofPrefix     = "msp:login_captcha:proof:"
	captchaIssuePrefix     = "msp:login_captcha:issue:"
	captchaWidth           = 320
	captchaHeight          = 160
	captchaPieceSize       = 48
	captchaTokenBytes      = 24

	captchaVerifyMissing   = int64(0)
	captchaVerifyInvalid   = int64(1)
	captchaVerifySucceeded = int64(2)
	captchaVerifyMalformed = int64(3)
)

var (
	// ErrCaptchaRateLimited indicates that one client requested too many challenges.
	ErrCaptchaRateLimited = errors.New("captcha challenge rate limited")
	// ErrCaptchaUnavailable indicates that challenge state cannot be stored safely.
	ErrCaptchaUnavailable = errors.New("captcha state unavailable")

	issueAndStoreCaptchaScript = goredis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 or redis.call("PTTL", KEYS[1]) < 0 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
if count > tonumber(ARGV[2]) then
  return 0
end
redis.call("SET", KEYS[2], ARGV[3], "PX", ARGV[4])
return 1
`)

	verifyAndStoreCaptchaProofScript = goredis.NewScript(`
local payload = redis.call("GET", KEYS[1])
if not payload then
  return 0
end
local ttl = redis.call("PTTL", KEYS[1])
redis.call("DEL", KEYS[1])
if ttl <= 0 then
  return 1
end
local decoded, state = pcall(cjson.decode, payload)
if not decoded or type(state) ~= "table" or type(state["client_key"]) ~= "string" or type(state["expected_x"]) ~= "number" or type(state["expires_at"]) ~= "string" then
  return 3
end
local position = tonumber(ARGV[2])
local tolerance = tonumber(ARGV[3])
local expires_at = state["expires_at_unix_ms"]
local now = tonumber(ARGV[4])
if expires_at ~= nil and (type(expires_at) ~= "number" or not now or expires_at <= now) then
  return 1
end
if state["client_key"] ~= ARGV[1] or not position or not tolerance or math.abs(state["expected_x"] - position) > tolerance then
  return 1
end
redis.call("SET", KEYS[2], ARGV[5], "PX", ARGV[6])
return 2
`)
)

// SliderCaptchaConfig controls the lifetime and abuse limits of login captchas.
type SliderCaptchaConfig struct {
	ChallengeTTL time.Duration
	ProofTTL     time.Duration
	IssueWindow  time.Duration
	Tolerance    int
	IssueLimit   int
	MaxLocalSize int
	Strict       bool
}

// SliderCaptchaChallenge contains only the public data needed to render a puzzle.
type SliderCaptchaChallenge struct {
	ID              string `json:"captcha_id"`
	BackgroundImage string `json:"background_image"`
	PieceImage      string `json:"piece_image"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	PieceWidth      int    `json:"piece_width"`
	PieceHeight     int    `json:"piece_height"`
	PieceY          int    `json:"piece_y"`
	ExpiresIn       int    `json:"expires_in"`
}

// SliderCaptchaManager creates visual challenges and one-time login proofs.
type SliderCaptchaManager struct {
	client *goredis.Client
	logger *slog.Logger
	config SliderCaptchaConfig
	now    func() time.Time

	mu         sync.Mutex
	challenges map[string]captchaChallengeState
	proofs     map[string]captchaProofState
	issues     map[string][]time.Time
}

type captchaChallengeState struct {
	ClientKey          string    `json:"client_key"`
	ExpectedX          int       `json:"expected_x"`
	ExpiresAt          time.Time `json:"expires_at"`
	ExpiresAtUnixMilli int64     `json:"expires_at_unix_ms,omitempty"`
}

type captchaProofState struct {
	ClientKey string    `json:"client_key"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewSliderCaptchaManager creates a Redis-backed captcha manager with a bounded
// in-memory fallback for development and degraded non-production environments.
func NewSliderCaptchaManager(client *goredis.Client, logger *slog.Logger, cfg SliderCaptchaConfig) (*SliderCaptchaManager, error) {
	if cfg.ChallengeTTL <= 0 || cfg.ProofTTL <= 0 || cfg.IssueWindow <= 0 {
		return nil, errors.New("captcha durations must be greater than 0")
	}
	if cfg.Tolerance <= 0 || cfg.Tolerance >= captchaPieceSize {
		return nil, errors.New("captcha tolerance must be between 1 and the piece size")
	}
	if cfg.IssueLimit <= 0 {
		return nil, errors.New("captcha issue limit must be greater than 0")
	}
	if cfg.MaxLocalSize <= 0 {
		return nil, errors.New("captcha local state size must be greater than 0")
	}
	if cfg.Strict && client == nil {
		return nil, errors.New("strict captcha manager requires redis client")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &SliderCaptchaManager{
		client:     client,
		logger:     logger,
		config:     cfg,
		now:        func() time.Time { return time.Now().UTC() },
		challenges: make(map[string]captchaChallengeState),
		proofs:     make(map[string]captchaProofState),
		issues:     make(map[string][]time.Time),
	}, nil
}

// NewChallenge creates and stores one short-lived visual puzzle.
func (m *SliderCaptchaManager) NewChallenge(ctx context.Context, clientKey string) (SliderCaptchaChallenge, error) {
	clientKey = strings.TrimSpace(clientKey)
	if clientKey == "" {
		return SliderCaptchaChallenge{}, errors.New("captcha client key is empty")
	}

	id, err := securerand.Hex(captchaTokenBytes)
	if err != nil {
		return SliderCaptchaChallenge{}, fmt.Errorf("generate captcha id: %w", err)
	}
	expectedX, pieceY, err := newSliderCaptchaPosition()
	if err != nil {
		return SliderCaptchaChallenge{}, fmt.Errorf("generate captcha position: %w", err)
	}
	expiresAt := m.now().Add(m.config.ChallengeTTL)
	state := captchaChallengeState{
		ClientKey:          clientKey,
		ExpectedX:          expectedX,
		ExpiresAt:          expiresAt,
		ExpiresAtUnixMilli: expiresAt.UnixMilli(),
	}
	if err := m.issueAndStoreChallenge(ctx, id, clientKey, state); err != nil {
		return SliderCaptchaChallenge{}, err
	}
	background, piece, err := renderSliderCaptcha(expectedX, pieceY)
	if err != nil {
		return SliderCaptchaChallenge{}, fmt.Errorf("render captcha: %w", err)
	}
	return SliderCaptchaChallenge{
		ID:              id,
		BackgroundImage: background,
		PieceImage:      piece,
		Width:           captchaWidth,
		Height:          captchaHeight,
		PieceWidth:      captchaPieceSize,
		PieceHeight:     captchaPieceSize,
		PieceY:          pieceY,
		ExpiresIn:       int(m.config.ChallengeTTL / time.Second),
	}, nil
}

// Verify consumes a visual challenge and returns a short-lived login proof when
// the submitted position is within the configured tolerance.
func (m *SliderCaptchaManager) Verify(ctx context.Context, challengeID string, position int, clientKey string) (string, bool, error) {
	challengeID = strings.TrimSpace(challengeID)
	clientKey = strings.TrimSpace(clientKey)
	if challengeID == "" || clientKey == "" {
		return "", false, nil
	}
	if m.client != nil {
		proof, proofState, err := m.newProof(clientKey)
		if err != nil {
			return "", false, err
		}
		status, redisErr := m.verifyAndStoreProofRedis(ctx, challengeID, position, clientKey, proof, proofState)
		if redisErr == nil {
			switch status {
			case captchaVerifySucceeded:
				return proof, true, nil
			case captchaVerifyInvalid:
				return "", false, nil
			case captchaVerifyMissing:
				if m.config.Strict {
					return "", false, nil
				}
			case captchaVerifyMalformed:
				return "", false, errors.New("decode captcha challenge state in redis")
			default:
				return "", false, fmt.Errorf("unexpected captcha verification status: %d", status)
			}
		} else if m.config.Strict {
			return "", false, fmt.Errorf("verify captcha in redis: %w", redisErr)
		} else {
			m.logger.Warn("redis captcha verify failed, using local fallback", "error", redisErr)
		}
	}
	return m.verifyLocal(challengeID, position, clientKey)
}

// ConsumeProof validates and removes one login proof so it cannot be replayed.
func (m *SliderCaptchaManager) ConsumeProof(ctx context.Context, proof string, clientKey string) (bool, error) {
	proof = strings.TrimSpace(proof)
	clientKey = strings.TrimSpace(clientKey)
	if proof == "" || clientKey == "" {
		return false, nil
	}
	state, found, err := m.consumeProof(ctx, proof)
	if err != nil || !found {
		return false, err
	}
	return state.ClientKey == clientKey && state.ExpiresAt.After(m.now()), nil
}

// ProofTTL returns the configured proof lifetime for HTTP response metadata.
func (m *SliderCaptchaManager) ProofTTL() time.Duration {
	if m == nil {
		return 0
	}
	return m.config.ProofTTL
}

// IssueWindow returns the configured challenge rate-limit window.
func (m *SliderCaptchaManager) IssueWindow() time.Duration {
	if m == nil {
		return 0
	}
	return m.config.IssueWindow
}

func (m *SliderCaptchaManager) issueAndStoreChallenge(ctx context.Context, id, clientKey string, state captchaChallengeState) error {
	if m.client != nil {
		payload, err := json.Marshal(state)
		if err != nil {
			return fmt.Errorf("encode captcha challenge state: %w", err)
		}
		allowed, err := issueAndStoreCaptchaScript.Run(
			ctx,
			m.client,
			[]string{captchaIssuePrefix + clientKey, captchaChallengePrefix + id},
			redisTTLMilliseconds(m.config.IssueWindow),
			m.config.IssueLimit,
			payload,
			redisTTLMilliseconds(m.config.ChallengeTTL),
		).Int64()
		if err == nil {
			if allowed == 0 {
				return ErrCaptchaRateLimited
			}
			if allowed != 1 {
				return fmt.Errorf("unexpected captcha issuance status: %d", allowed)
			}
			return nil
		}
		if m.config.Strict {
			return fmt.Errorf("issue captcha in redis: %w", err)
		}
		m.logger.Warn("redis captcha issuance failed, using local fallback", "error", err)
	}
	if !m.allowIssueLocal(clientKey) {
		return ErrCaptchaRateLimited
	}
	return m.storeChallengeLocal(id, state)
}

func (m *SliderCaptchaManager) allowIssueLocal(clientKey string) bool {
	now := m.now()
	cutoff := now.Add(-m.config.IssueWindow)
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, issueTimes := range m.issues {
		recentTimes := issueTimes[:0]
		for _, issueTime := range issueTimes {
			if issueTime.After(cutoff) {
				recentTimes = append(recentTimes, issueTime)
			}
		}
		if len(recentTimes) == 0 {
			delete(m.issues, key)
		} else {
			m.issues[key] = recentTimes
		}
	}
	if _, exists := m.issues[clientKey]; !exists && len(m.issues) >= m.config.MaxLocalSize {
		return false
	}
	hits := m.issues[clientKey]
	recent := hits[:0]
	for _, hit := range hits {
		if hit.After(cutoff) {
			recent = append(recent, hit)
		}
	}
	if len(recent) >= m.config.IssueLimit {
		m.issues[clientKey] = recent
		return false
	}
	m.issues[clientKey] = append(recent, now)
	return true
}

func (m *SliderCaptchaManager) storeChallengeLocal(id string, state captchaChallengeState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocalLocked()
	if len(m.challenges)+len(m.proofs) >= m.config.MaxLocalSize {
		return ErrCaptchaUnavailable
	}
	m.challenges[id] = state
	return nil
}

func (m *SliderCaptchaManager) storeProofLocal(proof string, state captchaProofState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocalLocked()
	if len(m.challenges)+len(m.proofs) >= m.config.MaxLocalSize {
		return ErrCaptchaUnavailable
	}
	m.proofs[proof] = state
	return nil
}

func (m *SliderCaptchaManager) consumeChallengeLocal(id string) (captchaChallengeState, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, found := m.challenges[id]
	delete(m.challenges, id)
	return state, found
}

func (m *SliderCaptchaManager) verifyAndStoreProofRedis(
	ctx context.Context,
	challengeID string,
	position int,
	clientKey string,
	proof string,
	proofState captchaProofState,
) (int64, error) {
	payload, err := json.Marshal(proofState)
	if err != nil {
		return 0, fmt.Errorf("encode captcha proof state: %w", err)
	}
	return verifyAndStoreCaptchaProofScript.Run(
		ctx,
		m.client,
		[]string{captchaChallengePrefix + challengeID, captchaProofPrefix + proof},
		clientKey,
		position,
		m.config.Tolerance,
		m.now().UnixMilli(),
		payload,
		redisTTLMilliseconds(m.config.ProofTTL),
	).Int64()
}

func (m *SliderCaptchaManager) verifyLocal(challengeID string, position int, clientKey string) (string, bool, error) {
	state, found := m.consumeChallengeLocal(challengeID)
	if !found {
		return "", false, nil
	}
	if state.ClientKey != clientKey || !state.ExpiresAt.After(m.now()) || absInt(state.ExpectedX-position) > m.config.Tolerance {
		return "", false, nil
	}
	proof, proofState, err := m.newProof(clientKey)
	if err != nil {
		return "", false, err
	}
	if err := m.storeProofLocal(proof, proofState); err != nil {
		return "", false, err
	}
	return proof, true, nil
}

func (m *SliderCaptchaManager) newProof(clientKey string) (string, captchaProofState, error) {
	proof, err := securerand.Hex(captchaTokenBytes)
	if err != nil {
		return "", captchaProofState{}, fmt.Errorf("generate captcha proof: %w", err)
	}
	return proof, captchaProofState{
		ClientKey: clientKey,
		ExpiresAt: m.now().Add(m.config.ProofTTL),
	}, nil
}

func (m *SliderCaptchaManager) consumeProof(ctx context.Context, proof string) (captchaProofState, bool, error) {
	var state captchaProofState
	found, fallback, err := m.consumeRedis(ctx, captchaProofPrefix+proof, &state)
	if err != nil || found || !fallback {
		return state, found, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state, found = m.proofs[proof]
	delete(m.proofs, proof)
	return state, found, nil
}

func (m *SliderCaptchaManager) consumeRedis(ctx context.Context, key string, target any) (found bool, fallback bool, err error) {
	if m.client == nil {
		return false, true, nil
	}
	payload, redisErr := m.client.GetDel(ctx, key).Bytes()
	switch {
	case redisErr == nil:
		if err := json.Unmarshal(payload, target); err != nil {
			return false, false, fmt.Errorf("decode captcha state: %w", err)
		}
		return true, false, nil
	case errors.Is(redisErr, goredis.Nil):
		return false, !m.config.Strict, nil
	case m.config.Strict:
		return false, false, fmt.Errorf("consume captcha from redis: %w", redisErr)
	default:
		m.logger.Warn("redis captcha consume failed, using local fallback", "error", redisErr)
		return false, true, nil
	}
}

func (m *SliderCaptchaManager) pruneLocalLocked() {
	now := m.now()
	for id, state := range m.challenges {
		if !state.ExpiresAt.After(now) {
			delete(m.challenges, id)
		}
	}
	for proof, state := range m.proofs {
		if !state.ExpiresAt.After(now) {
			delete(m.proofs, proof)
		}
	}
}

func newSliderCaptchaPosition() (expectedX int, pieceY int, err error) {
	expectedX, err = secureInt(captchaWidth - captchaPieceSize - 80)
	if err != nil {
		return 0, 0, err
	}
	expectedX += 64
	pieceY, err = secureInt(captchaHeight - captchaPieceSize - 28)
	if err != nil {
		return 0, 0, err
	}
	pieceY += 14
	return expectedX, pieceY, nil
}

func renderSliderCaptcha(expectedX, pieceY int) (backgroundData string, pieceData string, err error) {
	source := image.NewRGBA(image.Rect(0, 0, captchaWidth, captchaHeight))
	if err := paintCaptchaScene(source); err != nil {
		return "", "", err
	}
	background := image.NewRGBA(source.Bounds())
	copy(background.Pix, source.Pix)
	piece := image.NewRGBA(image.Rect(0, 0, captchaPieceSize, captchaPieceSize))

	for y := 0; y < captchaPieceSize; y++ {
		for x := 0; x < captchaPieceSize; x++ {
			if !jigsawMask(x, y) {
				continue
			}
			sourceColor := source.RGBAAt(expectedX+x, pieceY+y)
			piece.SetRGBA(x, y, sourceColor)
			background.SetRGBA(expectedX+x, pieceY+y, darken(sourceColor))
		}
	}
	outlineJigsaw(background, expectedX, pieceY, color.RGBA{R: 255, G: 255, B: 255, A: 210})
	outlineJigsaw(piece, 0, 0, color.RGBA{R: 255, G: 255, B: 255, A: 235})

	backgroundData, err = encodePNGDataURI(background)
	if err != nil {
		return "", "", err
	}
	pieceData, err = encodePNGDataURI(piece)
	if err != nil {
		return "", "", err
	}
	return backgroundData, pieceData, nil
}

func redisTTLMilliseconds(ttl time.Duration) int64 {
	milliseconds := ttl.Milliseconds()
	if milliseconds < 1 {
		return 1
	}
	return milliseconds
}

func paintCaptchaScene(target *image.RGBA) error {
	start := color.RGBA{R: 24, G: 119, B: 176, A: 255}
	end := color.RGBA{R: 77, G: 178, B: 154, A: 255}
	shift, err := secureInt(70)
	if err != nil {
		return err
	}
	start.R = addColorShift(start.R, shift, 3)
	end.B = addColorShift(end.B, shift, 2)
	for y := 0; y < captchaHeight; y++ {
		for x := 0; x < captchaWidth; x++ {
			ratio := float64(x+y) / float64(captchaWidth+captchaHeight)
			target.SetRGBA(x, y, mixColor(start, end, ratio))
		}
	}

	palette := []color.RGBA{
		{R: 253, G: 224, B: 71, A: 185},
		{R: 248, G: 113, B: 113, A: 165},
		{R: 255, G: 255, B: 255, A: 105},
		{R: 15, G: 23, B: 42, A: 80},
	}
	for i := 0; i < 18; i++ {
		x, err := secureInt(captchaWidth)
		if err != nil {
			return err
		}
		y, err := secureInt(captchaHeight)
		if err != nil {
			return err
		}
		size, err := secureInt(24)
		if err != nil {
			return err
		}
		paintCircle(target, x, y, size+7, palette[i%len(palette)])
	}
	for i := 0; i < 7; i++ {
		x, err := secureInt(captchaWidth - 45)
		if err != nil {
			return err
		}
		y, err := secureInt(captchaHeight - 18)
		if err != nil {
			return err
		}
		paintRect(target, x, y, 28+i*3, 5+i%3, palette[(i+2)%len(palette)])
	}
	return nil
}

func paintCircle(target *image.RGBA, centerX, centerY, radius int, value color.RGBA) {
	for y := centerY - radius; y <= centerY+radius; y++ {
		for x := centerX - radius; x <= centerX+radius; x++ {
			if x < 0 || y < 0 || x >= captchaWidth || y >= captchaHeight {
				continue
			}
			if square(x-centerX)+square(y-centerY) <= square(radius) {
				target.SetRGBA(x, y, blend(target.RGBAAt(x, y), value))
			}
		}
	}
}

func paintRect(target *image.RGBA, left, top, width, height int, value color.RGBA) {
	for y := top; y < top+height && y < captchaHeight; y++ {
		for x := left; x < left+width && x < captchaWidth; x++ {
			target.SetRGBA(x, y, blend(target.RGBAAt(x, y), value))
		}
	}
}

func jigsawMask(x, y int) bool {
	const margin = 7
	inside := x >= margin && x < captchaPieceSize-margin && y >= margin && y < captchaPieceSize-margin
	topTab := square(x-captchaPieceSize/2)+square(y-margin) <= square(7)
	rightTab := square(x-(captchaPieceSize-margin))+square(y-captchaPieceSize/2) <= square(7)
	leftIndent := square(x-margin)+square(y-(captchaPieceSize*2/3)) <= square(6)
	bottomIndent := square(x-(captchaPieceSize/3))+square(y-(captchaPieceSize-margin)) <= square(6)
	return (inside || topTab || rightTab) && !leftIndent && !bottomIndent
}

func outlineJigsaw(target *image.RGBA, offsetX, offsetY int, outline color.RGBA) {
	for y := 0; y < captchaPieceSize; y++ {
		for x := 0; x < captchaPieceSize; x++ {
			if !jigsawMask(x, y) {
				continue
			}
			if jigsawMask(x-1, y) && jigsawMask(x+1, y) && jigsawMask(x, y-1) && jigsawMask(x, y+1) {
				continue
			}
			target.SetRGBA(offsetX+x, offsetY+y, outline)
		}
	}
}

func encodePNGDataURI(value image.Image) (string, error) {
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, value); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buffer.Bytes()), nil
}

func secureInt(max int) (int, error) {
	if max <= 0 {
		return 0, errors.New("secure integer maximum must be greater than 0")
	}
	// #nosec G115 -- max is positive after the validation above.
	maxUint := uint64(max)
	limit := uint64(math.MaxUint32) + 1
	if maxUint > limit {
		return 0, errors.New("secure integer maximum exceeds the 32-bit sampling range")
	}
	limit -= limit % maxUint
	for {
		data, err := securerand.Bytes(4)
		if err != nil {
			return 0, err
		}
		value := uint64(binary.BigEndian.Uint32(data))
		if value < limit {
			// #nosec G115 -- modulo maxUint is strictly less than the positive int max.
			return int(value % maxUint), nil
		}
	}
}

func addColorShift(channel uint8, shift, divisor int) uint8 {
	if shift <= 0 || divisor <= 0 {
		return channel
	}
	delta := shift / divisor
	if delta > int(math.MaxUint8-channel) {
		return math.MaxUint8
	}
	// #nosec G115 -- delta is non-negative and bounded by the remaining channel range.
	return channel + uint8(delta)
}

func mixColor(left, right color.RGBA, ratio float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(left.R)*(1-ratio) + float64(right.R)*ratio),
		G: uint8(float64(left.G)*(1-ratio) + float64(right.G)*ratio),
		B: uint8(float64(left.B)*(1-ratio) + float64(right.B)*ratio),
		A: 255,
	}
}

func blend(background, foreground color.RGBA) color.RGBA {
	alpha := float64(foreground.A) / 255
	return color.RGBA{
		R: uint8(float64(foreground.R)*alpha + float64(background.R)*(1-alpha)),
		G: uint8(float64(foreground.G)*alpha + float64(background.G)*(1-alpha)),
		B: uint8(float64(foreground.B)*alpha + float64(background.B)*(1-alpha)),
		A: 255,
	}
}

func darken(value color.RGBA) color.RGBA {
	return blend(value, color.RGBA{R: 3, G: 7, B: 18, A: 145})
}

func square(value int) int {
	return value * value
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
