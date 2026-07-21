package xidian

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"mathstudy/backend-go/internal/platform/ptrutil"
	"mathstudy/backend-go/internal/platform/redact"
)

// Repository persists verified Xidian account bindings.
type Repository interface {
	GetAccount(context.Context, string) (Account, bool, error)
	UpsertAccount(context.Context, AccountUpsert) (Account, error)
	DeleteAccount(context.Context, string) error
}

// PortalClient verifies Xidian credentials through the IDS login flow.
type PortalClient interface {
	StartBinding(context.Context) (Challenge, error)
	CompleteBinding(context.Context, ChallengeState, LoginInput) error
}

// ChallengeStore stores short-lived login challenges.
type ChallengeStore interface {
	Set(context.Context, string, ChallengeState, time.Duration) error
	Get(context.Context, string) (ChallengeState, bool, error)
	Delete(context.Context, string) error
}

// Config contains application-level Xidian binding settings.
type Config struct {
	ChallengeTTL  time.Duration
	CaptchaWidth  int
	CaptchaHeight int
	PieceWidth    int
	PieceHeight   int
}

// Service implements Xidian account binding use cases.
type Service struct {
	repo       Repository
	client     PortalClient
	challenges ChallengeStore
	config     Config
	now        func() time.Time
	newID      func() (string, error)
}

// NewService creates a Xidian account binding service.
func NewService(repo Repository, client PortalClient, challenges ChallengeStore, config Config) (*Service, error) {
	if repo == nil {
		return nil, errors.New("xidian repository is nil")
	}
	if client == nil {
		return nil, errors.New("xidian portal client is nil")
	}
	if challenges == nil {
		return nil, errors.New("xidian challenge store is nil")
	}
	if config.ChallengeTTL <= 0 {
		return nil, errors.New("xidian challenge ttl must be greater than 0")
	}
	if config.CaptchaWidth <= 0 || config.CaptchaHeight <= 0 || config.PieceWidth <= 0 || config.PieceHeight <= 0 {
		return nil, errors.New("xidian captcha dimensions must be greater than 0")
	}
	return &Service{
		repo:       repo,
		client:     client,
		challenges: challenges,
		config:     config,
		now:        func() time.Time { return time.Now().UTC() },
		newID:      newUUID,
	}, nil
}

// GetBindingStatus returns the user's binding state.
func (s *Service) GetBindingStatus(ctx context.Context, userID string) (BindingStatus, error) {
	account, found, err := s.repo.GetAccount(ctx, userID)
	if err != nil {
		return BindingStatus{}, err
	}
	if !found {
		return BindingStatus{IsBound: false}, nil
	}
	return BindingStatus{
		IsBound:        true,
		Username:       &account.Username,
		LastVerifiedAt: account.LastVerifiedAt,
	}, nil
}

// StartBinding opens a captcha challenge.
func (s *Service) StartBinding(ctx context.Context) (BindStartResponse, error) {
	challenge, err := s.client.StartBinding(ctx)
	if err != nil {
		return BindStartResponse{}, normalizeServiceError(err, "BINDING_START_FAILED", "获取验证码失败")
	}
	challengeID, err := s.newID()
	if err != nil {
		return BindStartResponse{}, err
	}
	if err := s.challenges.Set(ctx, challengeID, challenge.State, s.config.ChallengeTTL); err != nil {
		return BindStartResponse{}, err
	}
	return BindStartResponse{
		ChallengeID:  challengeID,
		CaptchaBig:   challenge.CaptchaBig,
		CaptchaPiece: challenge.CaptchaPiece,
		PuzzleWidth:  s.config.CaptchaWidth,
		PuzzleHeight: s.config.CaptchaHeight,
		PieceWidth:   s.config.PieceWidth,
		PieceHeight:  s.config.PieceHeight,
		PieceY:       challenge.PieceY,
	}, nil
}

// CompleteBinding verifies credentials and stores only the verified account identity.
func (s *Service) CompleteBinding(ctx context.Context, userID string, input CompleteBindingInput) (BindCompleteResponse, error) {
	if input.SliderPosition < 0 || input.SliderPosition > 1 {
		return BindCompleteResponse{}, ServiceError{Code: "VALIDATION_ERROR", Message: "滑块位置必须在 0 到 1 之间", Status: 422}
	}
	state, found, err := s.challenges.Get(ctx, input.ChallengeID)
	if err != nil {
		return BindCompleteResponse{}, err
	}
	if !found {
		return BindCompleteResponse{}, ServiceError{Code: "CHALLENGE_EXPIRED", Message: "验证码已过期，请重新获取", Status: 400}
	}

	account, accountFound, err := s.repo.GetAccount(ctx, userID)
	if err != nil {
		return BindCompleteResponse{}, err
	}
	username := strings.TrimSpace(ptrutil.ValueOrZero(input.Username))
	if username == "" {
		if !accountFound {
			return BindCompleteResponse{}, ServiceError{Code: "ACCOUNT_REQUIRED", Message: "缺少账号信息", Status: 400}
		}
		username = account.Username
	}
	password := ptrutil.ValueOrZero(input.Password)
	if password == "" {
		return BindCompleteResponse{}, ServiceError{Code: "PASSWORD_REQUIRED", Message: "请输入密码完成验证", Status: 400}
	}
	if err := s.client.CompleteBinding(ctx, state, LoginInput{
		Username:       username,
		Password:       password,
		SliderPosition: input.SliderPosition,
	}); err != nil {
		return BindCompleteResponse{}, normalizeServiceError(err, "LOGIN_FAILED", "登录失败，请稍后重试")
	}

	accountID, err := s.newID()
	if err != nil {
		return BindCompleteResponse{}, err
	}
	now := s.now()
	account, err = s.repo.UpsertAccount(ctx, AccountUpsert{
		ID:             accountID,
		UserID:         userID,
		Username:       username,
		LastVerifiedAt: now,
		Now:            now,
	})
	if err != nil {
		return BindCompleteResponse{}, err
	}
	_ = s.challenges.Delete(ctx, input.ChallengeID)
	return BindCompleteResponse{
		IsBound:        true,
		Username:       account.Username,
		LastVerifiedAt: account.LastVerifiedAt,
	}, nil
}

// Unbind deletes a user's Xidian account binding.
func (s *Service) Unbind(ctx context.Context, userID string) error {
	return s.repo.DeleteAccount(ctx, userID)
}

func normalizeServiceError(err error, fallbackCode string, fallbackMessage string) error {
	var serviceErr ServiceError
	if errors.As(err, &serviceErr) {
		return sanitizeServiceError(serviceErr)
	}
	return ServiceError{Code: fallbackCode, Message: fallbackMessage, Status: 400, Err: err}
}

func sanitizeServiceError(serviceErr ServiceError) ServiceError {
	serviceErr.Code = redact.String(serviceErr.Code)
	serviceErr.Message = redact.String(serviceErr.Message)
	if serviceErr.Err != nil {
		serviceErr.Err = errors.New(redact.String(serviceErr.Err.Error()))
	}
	return serviceErr
}

// MemoryChallengeStore stores challenges in process memory.
type MemoryChallengeStore struct {
	mu      sync.Mutex
	items   map[string]memoryChallenge
	now     func() time.Time
	maxSize int
}

type memoryChallenge struct {
	state     ChallengeState
	expiresAt time.Time
}

// NewMemoryChallengeStore creates a bounded in-process challenge store.
func NewMemoryChallengeStore(maxSizes ...int) *MemoryChallengeStore {
	maxSize := 500
	if len(maxSizes) > 0 && maxSizes[0] > 0 {
		maxSize = maxSizes[0]
	}
	return &MemoryChallengeStore{
		items:   map[string]memoryChallenge{},
		now:     func() time.Time { return time.Now().UTC() },
		maxSize: maxSize,
	}
}

// Set stores one challenge.
func (s *MemoryChallengeStore) Set(_ context.Context, id string, state ChallengeState, ttl time.Duration) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("xidian challenge ID is empty")
	}
	if ttl <= 0 {
		return errors.New("xidian challenge TTL must be greater than 0")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if _, exists := s.items[id]; !exists && len(s.items) >= s.maxSize {
		s.pruneExpiredLocked(now)
		if len(s.items) >= s.maxSize {
			s.evictEarliestLocked()
		}
	}
	s.items[id] = memoryChallenge{state: state, expiresAt: now.Add(ttl)}
	return nil
}

// Get returns one challenge if it exists and is not expired.
func (s *MemoryChallengeStore) Get(_ context.Context, id string) (ChallengeState, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return ChallengeState{}, false, nil
	}
	if !item.expiresAt.After(s.now()) {
		delete(s.items, id)
		return ChallengeState{}, false, nil
	}
	return item.state, true, nil
}

// Delete removes one challenge.
func (s *MemoryChallengeStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
	return nil
}

func (s *MemoryChallengeStore) pruneExpiredLocked(now time.Time) {
	for id, item := range s.items {
		if !item.expiresAt.After(now) {
			delete(s.items, id)
		}
	}
}

func (s *MemoryChallengeStore) evictEarliestLocked() {
	var earliestID string
	var earliestExpiry time.Time
	for id, item := range s.items {
		if earliestID == "" || item.expiresAt.Before(earliestExpiry) {
			earliestID = id
			earliestExpiry = item.expiresAt
		}
	}
	if earliestID != "" {
		delete(s.items, earliestID)
	}
}

func (e ServiceError) Error() string {
	sanitized := sanitizeServiceError(e)
	if sanitized.Err != nil {
		return fmt.Sprintf("%s: %s: %v", sanitized.Code, sanitized.Message, sanitized.Err)
	}
	return sanitized.Code + ": " + sanitized.Message
}
