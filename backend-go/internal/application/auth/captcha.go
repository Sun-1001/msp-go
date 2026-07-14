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

	"mathstudy/backend-go/internal/platform/ratelimit"
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
)

var (
	// ErrCaptchaRateLimited indicates that one client requested too many challenges.
	ErrCaptchaRateLimited = errors.New("captcha challenge rate limited")
	// ErrCaptchaUnavailable indicates that challenge state cannot be stored safely.
	ErrCaptchaUnavailable = errors.New("captcha state unavailable")
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
	ClientKey string    `json:"client_key"`
	ExpectedX int       `json:"expected_x"`
	ExpiresAt time.Time `json:"expires_at"`
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
	allowed, err := m.allowIssue(ctx, clientKey)
	if err != nil {
		return SliderCaptchaChallenge{}, err
	}
	if !allowed {
		return SliderCaptchaChallenge{}, ErrCaptchaRateLimited
	}

	id, err := securerand.Hex(captchaTokenBytes)
	if err != nil {
		return SliderCaptchaChallenge{}, fmt.Errorf("generate captcha id: %w", err)
	}
	background, piece, expectedX, pieceY, err := renderSliderCaptcha()
	if err != nil {
		return SliderCaptchaChallenge{}, fmt.Errorf("render captcha: %w", err)
	}
	state := captchaChallengeState{
		ClientKey: clientKey,
		ExpectedX: expectedX,
		ExpiresAt: m.now().Add(m.config.ChallengeTTL),
	}
	if err := m.storeChallenge(ctx, id, state); err != nil {
		return SliderCaptchaChallenge{}, err
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
	state, found, err := m.consumeChallenge(ctx, challengeID)
	if err != nil || !found {
		return "", false, err
	}
	if state.ClientKey != clientKey || !state.ExpiresAt.After(m.now()) || absInt(state.ExpectedX-position) > m.config.Tolerance {
		return "", false, nil
	}

	proof, err := securerand.Hex(captchaTokenBytes)
	if err != nil {
		return "", false, fmt.Errorf("generate captcha proof: %w", err)
	}
	proofState := captchaProofState{ClientKey: clientKey, ExpiresAt: m.now().Add(m.config.ProofTTL)}
	if err := m.storeProof(ctx, proof, proofState); err != nil {
		return "", false, err
	}
	return proof, true, nil
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

func (m *SliderCaptchaManager) allowIssue(ctx context.Context, clientKey string) (bool, error) {
	if m.client != nil {
		key := captchaIssuePrefix + clientKey
		count, err := ratelimit.IncrementWithExpiry(ctx, m.client, key, m.config.IssueWindow)
		if err == nil {
			return count <= int64(m.config.IssueLimit), nil
		}
		if m.config.Strict {
			return false, fmt.Errorf("rate limit captcha in redis: %w", err)
		}
		m.logger.Warn("redis captcha rate limit failed, using local fallback", "error", err)
	}
	return m.allowIssueLocal(clientKey), nil
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

func (m *SliderCaptchaManager) storeChallenge(ctx context.Context, id string, state captchaChallengeState) error {
	if stored, err := m.storeRedis(ctx, captchaChallengePrefix+id, state, m.config.ChallengeTTL); stored || err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocalLocked()
	if len(m.challenges)+len(m.proofs) >= m.config.MaxLocalSize {
		return ErrCaptchaUnavailable
	}
	m.challenges[id] = state
	return nil
}

func (m *SliderCaptchaManager) storeProof(ctx context.Context, proof string, state captchaProofState) error {
	if stored, err := m.storeRedis(ctx, captchaProofPrefix+proof, state, m.config.ProofTTL); stored || err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocalLocked()
	if len(m.challenges)+len(m.proofs) >= m.config.MaxLocalSize {
		return ErrCaptchaUnavailable
	}
	m.proofs[proof] = state
	return nil
}

func (m *SliderCaptchaManager) storeRedis(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	if m.client == nil {
		return false, nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return false, fmt.Errorf("encode captcha state: %w", err)
	}
	if err := m.client.Set(ctx, key, payload, ttl).Err(); err == nil {
		return true, nil
	} else if m.config.Strict {
		return false, fmt.Errorf("store captcha in redis: %w", err)
	} else {
		m.logger.Warn("redis captcha store failed, using local fallback", "error", err)
		return false, nil
	}
}

func (m *SliderCaptchaManager) consumeChallenge(ctx context.Context, id string) (captchaChallengeState, bool, error) {
	var state captchaChallengeState
	found, fallback, err := m.consumeRedis(ctx, captchaChallengePrefix+id, &state)
	if err != nil || found || !fallback {
		return state, found, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state, found = m.challenges[id]
	delete(m.challenges, id)
	return state, found, nil
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

func renderSliderCaptcha() (backgroundData string, pieceData string, expectedX int, pieceY int, err error) {
	expectedX, err = secureInt(captchaWidth - captchaPieceSize - 80)
	if err != nil {
		return "", "", 0, 0, err
	}
	expectedX += 64
	pieceY, err = secureInt(captchaHeight - captchaPieceSize - 28)
	if err != nil {
		return "", "", 0, 0, err
	}
	pieceY += 14

	source := image.NewRGBA(image.Rect(0, 0, captchaWidth, captchaHeight))
	if err := paintCaptchaScene(source); err != nil {
		return "", "", 0, 0, err
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
		return "", "", 0, 0, err
	}
	pieceData, err = encodePNGDataURI(piece)
	if err != nil {
		return "", "", 0, 0, err
	}
	return backgroundData, pieceData, expectedX, pieceY, nil
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
