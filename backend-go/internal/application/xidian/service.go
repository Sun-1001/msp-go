package xidian

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"mathstudy/backend-go/internal/platform/redact"
)

var (
	// ErrNotBound is returned when a user has no Xidian account binding.
	ErrNotBound = ServiceError{Code: "NOT_BOUND", Message: "请先绑定西电账号", Status: 400}
)

// Repository persists Xidian accounts and data snapshots.
type Repository interface {
	GetAccount(context.Context, string) (Account, bool, error)
	UpsertAccount(context.Context, AccountUpsert) (Account, error)
	DeleteAccountAndSnapshots(context.Context, string) error
	LatestSyncAt(context.Context, string) (*time.Time, error)
	SaveSnapshot(context.Context, SnapshotInput) error
	LatestSnapshot(context.Context, string, string) (Snapshot, bool, error)
	UpdateCookies(context.Context, string, []Cookie, time.Time) error
}

// PortalClient hides Xidian IDS/Ehall/Yjspt HTTP details behind the application boundary.
type PortalClient interface {
	StartBinding(context.Context) (Challenge, error)
	CompleteBinding(context.Context, ChallengeState, LoginInput) (LoginResult, error)
	Sync(context.Context, SyncRequest) (SyncResult, error)
}

// Cipher protects the stored Xidian password.
type Cipher interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}

// ChallengeStore stores short-lived login challenges.
type ChallengeStore interface {
	Set(context.Context, string, ChallengeState, time.Duration) error
	Get(context.Context, string) (ChallengeState, bool, error)
	Delete(context.Context, string) error
}

// Config contains application-level Xidian behavior settings.
type Config struct {
	ChallengeTTL            time.Duration
	SnapshotFallbackEnabled bool
	CaptchaWidth            int
	CaptchaHeight           int
	PieceWidth              int
	PieceHeight             int
}

// Service implements Xidian account binding and academic data sync use cases.
type Service struct {
	repo       Repository
	client     PortalClient
	cipher     Cipher
	challenges ChallengeStore
	config     Config
	now        func() time.Time
	newID      func() (string, error)
}

// NewService creates a Xidian application service.
func NewService(repo Repository, client PortalClient, cipher Cipher, challenges ChallengeStore, config Config) (*Service, error) {
	if repo == nil {
		return nil, errors.New("xidian repository is nil")
	}
	if client == nil {
		return nil, errors.New("xidian portal client is nil")
	}
	if cipher == nil {
		return nil, errors.New("xidian cipher is nil")
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
		cipher:     cipher,
		challenges: challenges,
		config:     config,
		now:        func() time.Time { return time.Now().UTC() },
		newID:      newUUID,
	}, nil
}

// GetBindingStatus returns the user's binding state and latest snapshot time.
func (s *Service) GetBindingStatus(ctx context.Context, userID string) (BindingStatus, error) {
	account, found, err := s.repo.GetAccount(ctx, userID)
	if err != nil {
		return BindingStatus{}, err
	}
	if !found {
		return BindingStatus{IsBound: false}, nil
	}
	lastSyncAt, err := s.repo.LatestSyncAt(ctx, userID)
	if err != nil {
		return BindingStatus{}, err
	}
	return BindingStatus{
		IsBound:        true,
		Username:       &account.Username,
		IsPostgraduate: account.IsPostgraduate,
		LastVerifiedAt: account.LastVerifiedAt,
		LastSyncAt:     lastSyncAt,
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

// CompleteBinding verifies the captcha, logs in, and stores the account binding.
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
	username := stringValue(input.Username)
	if username == "" {
		if !accountFound {
			return BindCompleteResponse{}, ServiceError{Code: "ACCOUNT_REQUIRED", Message: "缺少账号信息", Status: 400}
		}
		username = account.Username
	}
	password := stringValue(input.Password)
	if password == "" {
		if !accountFound || account.EncryptedPassword == "" {
			return BindCompleteResponse{}, ServiceError{Code: "PASSWORD_REQUIRED", Message: "请输入密码完成绑定", Status: 400}
		}
		password, err = s.cipher.Decrypt(account.EncryptedPassword)
		if err != nil || password == "" {
			return BindCompleteResponse{}, ServiceError{Code: "PASSWORD_REQUIRED", Message: "请输入密码完成绑定", Status: 400}
		}
	}
	login, err := s.client.CompleteBinding(ctx, state, LoginInput{
		Username:       username,
		Password:       password,
		SliderPosition: input.SliderPosition,
	})
	if err != nil {
		return BindCompleteResponse{}, normalizeServiceError(err, "LOGIN_FAILED", "登录失败，请稍后重试")
	}
	encryptedPassword, err := s.cipher.Encrypt(password)
	if err != nil {
		return BindCompleteResponse{}, err
	}
	accountID, err := s.newID()
	if err != nil {
		return BindCompleteResponse{}, err
	}
	now := s.now()
	account, err = s.repo.UpsertAccount(ctx, AccountUpsert{
		ID:                accountID,
		UserID:            userID,
		Username:          username,
		EncryptedPassword: encryptedPassword,
		IsPostgraduate:    login.IsPostgraduate,
		SessionCookies:    login.Cookies,
		LastVerifiedAt:    now,
		Now:               now,
	})
	if err != nil {
		return BindCompleteResponse{}, err
	}
	_ = s.challenges.Delete(ctx, input.ChallengeID)
	return BindCompleteResponse{
		IsBound:        true,
		Username:       account.Username,
		IsPostgraduate: account.IsPostgraduate,
		LastVerifiedAt: account.LastVerifiedAt,
	}, nil
}

// Unbind deletes a user's Xidian account and cached snapshots.
func (s *Service) Unbind(ctx context.Context, userID string) error {
	return s.repo.DeleteAccountAndSnapshots(ctx, userID)
}

// SyncClasstable refreshes the user's timetable.
func (s *Service) SyncClasstable(ctx context.Context, userID string) (SyncResponse, error) {
	return s.sync(ctx, userID, "classtable")
}

// SyncExams refreshes the user's exam schedule.
func (s *Service) SyncExams(ctx context.Context, userID string) (SyncResponse, error) {
	return s.sync(ctx, userID, "exam")
}

// SyncScores refreshes the user's scores.
func (s *Service) SyncScores(ctx context.Context, userID string) (SyncResponse, error) {
	return s.sync(ctx, userID, "score")
}

// GetSnapshot returns the latest cached data for dataType.
func (s *Service) GetSnapshot(ctx context.Context, userID string, dataType string) (SnapshotResponse, error) {
	snapshot, found, err := s.repo.LatestSnapshot(ctx, userID, dataType)
	if err != nil {
		return SnapshotResponse{}, err
	}
	if !found {
		return SnapshotResponse{}, ServiceError{Code: "NO_SNAPSHOT", Message: "暂无缓存数据", Status: 404}
	}
	cachedAt := snapshot.FetchedAt.Format(time.RFC3339Nano)
	return SnapshotResponse{Data: snapshot.Payload, IsCached: true, CachedAt: &cachedAt}, nil
}

func (s *Service) sync(ctx context.Context, userID string, dataType string) (SyncResponse, error) {
	account, found, err := s.repo.GetAccount(ctx, userID)
	if err != nil {
		return SyncResponse{}, err
	}
	if !found {
		return SyncResponse{}, ErrNotBound
	}
	result, err := s.client.Sync(ctx, SyncRequest{
		DataType:       dataType,
		Username:       account.Username,
		IsPostgraduate: account.IsPostgraduate,
		Cookies:        account.SessionCookies,
	})
	if err != nil {
		if s.config.SnapshotFallbackEnabled {
			snapshot, ok, snapshotErr := s.repo.LatestSnapshot(ctx, userID, dataType)
			if snapshotErr != nil {
				return SyncResponse{}, snapshotErr
			}
			if ok {
				payload := copyPayload(snapshot.Payload)
				payload["is_cached"] = true
				payload["cached_at"] = snapshot.FetchedAt.Format(time.RFC3339Nano)
				return SyncResponse{Data: payload, FetchedAt: s.now(), IsCached: true}, nil
			}
		}
		return SyncResponse{}, normalizeServiceError(err, "SYNC_FAILED", "同步失败，请稍后重试")
	}
	now := s.now()
	snapshotID, err := s.newID()
	if err != nil {
		return SyncResponse{}, err
	}
	if len(result.Cookies) > 0 {
		if err := s.repo.UpdateCookies(ctx, userID, result.Cookies, now); err != nil {
			return SyncResponse{}, err
		}
	}
	if err := s.repo.SaveSnapshot(ctx, SnapshotInput{
		ID:           snapshotID,
		UserID:       userID,
		DataType:     dataType,
		SemesterCode: stringPtrFromAny(result.Payload["semester_code"]),
		Payload:      result.Payload,
		FetchedAt:    now,
	}); err != nil {
		return SyncResponse{}, err
	}
	return SyncResponse{Data: result.Payload, FetchedAt: now, IsCached: false}, nil
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

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringPtrFromAny(value any) *string {
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return nil
		}
		return &typed
	default:
		return nil
	}
}

func copyPayload(payload map[string]any) map[string]any {
	copied := make(map[string]any, len(payload))
	for key, value := range payload {
		copied[key] = value
	}
	return copied
}

// MemoryChallengeStore stores challenges in process memory.
type MemoryChallengeStore struct {
	mu    sync.Mutex
	items map[string]memoryChallenge
	now   func() time.Time
}

type memoryChallenge struct {
	state     ChallengeState
	expiresAt time.Time
}

// NewMemoryChallengeStore creates an in-process challenge store.
func NewMemoryChallengeStore() *MemoryChallengeStore {
	return &MemoryChallengeStore{
		items: map[string]memoryChallenge{},
		now:   func() time.Time { return time.Now().UTC() },
	}
}

// Set stores one challenge.
func (s *MemoryChallengeStore) Set(_ context.Context, id string, state ChallengeState, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[id] = memoryChallenge{state: state, expiresAt: s.now().Add(ttl)}
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
	if item.expiresAt.Before(s.now()) {
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

func (e ServiceError) Error() string {
	sanitized := sanitizeServiceError(e)
	if sanitized.Err != nil {
		return fmt.Sprintf("%s: %s: %v", sanitized.Code, sanitized.Message, sanitized.Err)
	}
	return sanitized.Code + ": " + sanitized.Message
}
