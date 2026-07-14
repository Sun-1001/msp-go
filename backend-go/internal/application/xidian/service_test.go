package xidian

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestBindingStatusReturnsUnboundAndBoundState(t *testing.T) {
	repo := &fakeRepo{}
	service := newTestService(repo, &fakePortal{}, &fakeCipher{})

	status, err := service.GetBindingStatus(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetBindingStatus(unbound) error = %v", err)
	}
	if status.IsBound {
		t.Fatalf("status = %#v", status)
	}

	isPG := true
	verified := time.Date(2026, 5, 6, 9, 0, 0, 0, time.UTC)
	synced := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	repo.account = Account{UserID: "user-1", Username: "student", IsPostgraduate: &isPG, LastVerifiedAt: &verified}
	repo.accountFound = true
	repo.latestSyncAt = &synced
	status, err = service.GetBindingStatus(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetBindingStatus(bound) error = %v", err)
	}
	if !status.IsBound || *status.Username != "student" || !*status.IsPostgraduate || !status.LastSyncAt.Equal(synced) {
		t.Fatalf("status = %#v", status)
	}
}

func TestStartAndCompleteBindingStoresAccount(t *testing.T) {
	portal := &fakePortal{
		challenge: Challenge{CaptchaBig: "big", CaptchaPiece: "piece", PieceY: 12, State: ChallengeState{PasswordSalt: "salt"}},
		login:     LoginResult{Cookies: []Cookie{{"name": "sid", "value": "1"}}},
	}
	cipher := &fakeCipher{}
	repo := &fakeRepo{}
	service := newTestService(repo, portal, cipher)
	service.newID = func() (string, error) { return "challenge-1", nil }
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	start, err := service.StartBinding(context.Background())
	if err != nil {
		t.Fatalf("StartBinding() error = %v", err)
	}
	if start.ChallengeID != "challenge-1" || start.PuzzleWidth != 280 || start.PieceY != 12 {
		t.Fatalf("start = %#v", start)
	}
	username := "student"
	password := "password"
	complete, err := service.CompleteBinding(context.Background(), "user-1", CompleteBindingInput{
		ChallengeID:    "challenge-1",
		SliderPosition: 0.5,
		Username:       &username,
		Password:       &password,
	})
	if err != nil {
		t.Fatalf("CompleteBinding() error = %v", err)
	}
	if !complete.IsBound || complete.Username != "student" {
		t.Fatalf("complete = %#v", complete)
	}
	if repo.upsert.Username != "student" || repo.upsert.EncryptedPassword != "enc:password" || len(repo.upsert.SessionCookies) != 1 {
		t.Fatalf("upsert = %#v", repo.upsert)
	}
	if portal.loginInput.Username != "student" || portal.loginInput.Password != "password" || portal.loginInput.SliderPosition != 0.5 {
		t.Fatalf("login input = %#v", portal.loginInput)
	}
}

func TestCompleteBindingReusesStoredPassword(t *testing.T) {
	repo := &fakeRepo{accountFound: true, account: Account{Username: "student", EncryptedPassword: "enc:stored"}}
	portal := &fakePortal{login: LoginResult{}}
	service := newTestService(repo, portal, &fakeCipher{})
	service.newID = func() (string, error) { return "challenge-1", nil }
	_, err := service.StartBinding(context.Background())
	if err != nil {
		t.Fatalf("StartBinding() error = %v", err)
	}
	_, err = service.CompleteBinding(context.Background(), "user-1", CompleteBindingInput{ChallengeID: "challenge-1", SliderPosition: 0.2})
	if err != nil {
		t.Fatalf("CompleteBinding() error = %v", err)
	}
	if portal.loginInput.Username != "student" || portal.loginInput.Password != "stored" {
		t.Fatalf("login input = %#v", portal.loginInput)
	}
}

func TestSyncSavesSnapshotAndCookies(t *testing.T) {
	repo := &fakeRepo{accountFound: true, account: Account{Username: "student", SessionCookies: []Cookie{{"name": "sid", "value": "old"}}}}
	portal := &fakePortal{syncResult: SyncResult{Payload: map[string]any{"semester_code": "2025-2026-2", "scores": []any{}}, Cookies: []Cookie{{"name": "sid", "value": "new"}}}}
	service := newTestService(repo, portal, &fakeCipher{})
	service.newID = func() (string, error) { return "snapshot-1", nil }
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	response, err := service.SyncScores(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("SyncScores() error = %v", err)
	}
	if response.IsCached || !response.FetchedAt.Equal(now) {
		t.Fatalf("response = %#v", response)
	}
	if repo.snapshot.ID != "snapshot-1" || repo.snapshot.DataType != "score" || *repo.snapshot.SemesterCode != "2025-2026-2" {
		t.Fatalf("snapshot = %#v", repo.snapshot)
	}
	if repo.updatedCookies[0]["value"] != "new" {
		t.Fatalf("cookies = %#v", repo.updatedCookies)
	}
}

func TestSyncFallsBackToSnapshot(t *testing.T) {
	fetchedAt := time.Date(2026, 5, 5, 9, 0, 0, 0, time.UTC)
	repo := &fakeRepo{
		accountFound:  true,
		account:       Account{Username: "student"},
		snapshotFound: true,
		snapshotValue: Snapshot{Payload: map[string]any{"scores": []any{"cached"}}, FetchedAt: fetchedAt},
	}
	portal := &fakePortal{syncErr: ServiceError{Code: "CAPTCHA_REQUIRED", Message: "会话已过期，请重新验证", Status: 409}}
	service := newTestService(repo, portal, &fakeCipher{})

	response, err := service.SyncScores(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("SyncScores() error = %v", err)
	}
	if !response.IsCached || response.Data["is_cached"] != true || response.Data["cached_at"] == nil {
		t.Fatalf("response = %#v", response)
	}
}

func TestStartBindingReturnsIDGenerationError(t *testing.T) {
	idErr := errors.New("random failed")
	service := newTestService(&fakeRepo{}, &fakePortal{}, &fakeCipher{})
	service.newID = func() (string, error) { return "", idErr }

	if _, err := service.StartBinding(context.Background()); !errors.Is(err, idErr) {
		t.Fatalf("StartBinding() error = %v, want %v", err, idErr)
	}
}

func TestCompleteBindingReturnsIDGenerationError(t *testing.T) {
	idErr := errors.New("random failed")
	service := newTestService(&fakeRepo{}, &fakePortal{}, &fakeCipher{})
	if err := service.challenges.Set(context.Background(), "challenge-1", ChallengeState{PasswordSalt: "salt"}, time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	service.newID = func() (string, error) { return "", idErr }
	username := "student"
	password := "password"

	_, err := service.CompleteBinding(context.Background(), "user-1", CompleteBindingInput{
		ChallengeID:    "challenge-1",
		SliderPosition: 0.5,
		Username:       &username,
		Password:       &password,
	})
	if !errors.Is(err, idErr) {
		t.Fatalf("CompleteBinding() error = %v, want %v", err, idErr)
	}
}

func TestSyncReturnsIDGenerationError(t *testing.T) {
	idErr := errors.New("random failed")
	repo := &fakeRepo{accountFound: true, account: Account{Username: "student"}}
	portal := &fakePortal{syncResult: SyncResult{
		Payload: map[string]any{"scores": []any{}},
		Cookies: []Cookie{{"name": "sid", "value": "new"}},
	}}
	service := newTestService(repo, portal, &fakeCipher{})
	service.newID = func() (string, error) { return "", idErr }

	_, err := service.SyncScores(context.Background(), "user-1")
	if !errors.Is(err, idErr) {
		t.Fatalf("SyncScores() error = %v, want %v", err, idErr)
	}
	if len(repo.updatedCookies) != 0 {
		t.Fatalf("updatedCookies = %#v, want none when snapshot ID generation fails", repo.updatedCookies)
	}
}

func TestPortalServiceErrorsAreRedacted(t *testing.T) {
	portal := &fakePortal{
		loginErr: ServiceError{
			Code:    "PASSWORD_WRONG",
			Message: "登录失败 Authorization: Bearer portal-token url=https://ids.example.com/authserver/login?token=query-token",
			Status:  http.StatusUnauthorized,
			Err:     errors.New("api_key=plain cookie=sid-secret"),
		},
	}
	service := newTestService(&fakeRepo{}, portal, &fakeCipher{})
	service.newID = func() (string, error) { return "challenge-1", nil }
	_, err := service.StartBinding(context.Background())
	if err != nil {
		t.Fatalf("StartBinding() error = %v", err)
	}
	username := "student"
	password := "password"
	_, err = service.CompleteBinding(context.Background(), "user-1", CompleteBindingInput{
		ChallengeID:    "challenge-1",
		SliderPosition: 0.5,
		Username:       &username,
		Password:       &password,
	})
	var serviceErr ServiceError
	if !errors.As(err, &serviceErr) {
		t.Fatalf("CompleteBinding() error = %v, want ServiceError", err)
	}
	assertNoXidianCredentialLeak(t, serviceErr.Message)
	assertNoXidianCredentialLeak(t, serviceErr.Error())
}

func TestServiceErrors(t *testing.T) {
	service := newTestService(&fakeRepo{}, &fakePortal{}, &fakeCipher{})
	var serviceErr ServiceError
	if _, err := service.SyncScores(context.Background(), "user-1"); !errors.As(err, &serviceErr) {
		t.Fatalf("SyncScores() error = %v, want ServiceError", err)
	}
	if _, err := service.GetSnapshot(context.Background(), "user-1", "score"); !errors.As(err, &serviceErr) {
		t.Fatalf("GetSnapshot() error = %v, want ServiceError", err)
	}
	if _, err := NewService(nil, &fakePortal{}, &fakeCipher{}, NewMemoryChallengeStore(), Config{ChallengeTTL: time.Minute, CaptchaWidth: 1, CaptchaHeight: 1, PieceWidth: 1, PieceHeight: 1}); err == nil {
		t.Fatal("NewService(nil repo) error = nil, want error")
	}
}

func assertNoXidianCredentialLeak(t *testing.T, value string) {
	t.Helper()
	for _, leaked := range []string{"portal-token", "token=query-token", "api_key=plain", "sid-secret", "Bearer portal-token"} {
		if strings.Contains(value, leaked) {
			t.Fatalf("value leaked %q in %q", leaked, value)
		}
	}
	if !strings.Contains(value, "[REDACTED]") {
		t.Fatalf("value = %q, want redaction marker", value)
	}
}

func newTestService(repo Repository, portal PortalClient, cipher Cipher) *Service {
	service, err := NewService(repo, portal, cipher, NewMemoryChallengeStore(), Config{
		ChallengeTTL:            time.Minute,
		SnapshotFallbackEnabled: true,
		CaptchaWidth:            280,
		CaptchaHeight:           155,
		PieceWidth:              44,
		PieceHeight:             155,
	})
	if err != nil {
		panic(err)
	}
	return service
}

func TestMemoryChallengeStoreBoundsExpiryAndInputs(t *testing.T) {
	now := time.Date(2026, 7, 14, 3, 0, 0, 0, time.UTC)
	store := NewMemoryChallengeStore(2)
	store.now = func() time.Time { return now }
	ctx := context.Background()

	if err := store.Set(ctx, "", ChallengeState{}, time.Minute); err == nil {
		t.Fatal("Set(empty ID) error = nil")
	}
	if err := store.Set(ctx, "invalid-ttl", ChallengeState{}, 0); err == nil {
		t.Fatal("Set(invalid TTL) error = nil")
	}
	if err := store.Set(ctx, "first", ChallengeState{PasswordSalt: "first"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if err := store.Set(ctx, "second", ChallengeState{PasswordSalt: "second"}, 2*time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(ctx, "third", ChallengeState{PasswordSalt: "third"}, 3*time.Minute); err != nil {
		t.Fatal(err)
	}
	if len(store.items) != 2 {
		t.Fatalf("challenge count = %d, want 2", len(store.items))
	}
	if _, found, err := store.Get(ctx, "first"); err != nil || found {
		t.Fatalf("evicted Get() found = %t, error = %v", found, err)
	}

	now = now.Add(2 * time.Minute)
	if _, found, err := store.Get(ctx, "second"); err != nil || found {
		t.Fatalf("expired Get() found = %t, error = %v", found, err)
	}
}

type fakeRepo struct {
	account        Account
	accountFound   bool
	upsert         AccountUpsert
	latestSyncAt   *time.Time
	snapshot       SnapshotInput
	snapshotValue  Snapshot
	snapshotFound  bool
	updatedCookies []Cookie
}

func (r *fakeRepo) GetAccount(context.Context, string) (Account, bool, error) {
	return r.account, r.accountFound, nil
}

func (r *fakeRepo) UpsertAccount(_ context.Context, input AccountUpsert) (Account, error) {
	r.upsert = input
	verified := input.LastVerifiedAt
	return Account{Username: input.Username, IsPostgraduate: input.IsPostgraduate, LastVerifiedAt: &verified}, nil
}

func (r *fakeRepo) DeleteAccountAndSnapshots(context.Context, string) error { return nil }

func (r *fakeRepo) LatestSyncAt(context.Context, string) (*time.Time, error) {
	return r.latestSyncAt, nil
}

func (r *fakeRepo) SaveSnapshot(_ context.Context, input SnapshotInput) error {
	r.snapshot = input
	return nil
}

func (r *fakeRepo) LatestSnapshot(context.Context, string, string) (Snapshot, bool, error) {
	return r.snapshotValue, r.snapshotFound, nil
}

func (r *fakeRepo) UpdateCookies(_ context.Context, _ string, cookies []Cookie, _ time.Time) error {
	r.updatedCookies = cookies
	return nil
}

type fakePortal struct {
	challenge  Challenge
	login      LoginResult
	loginErr   error
	loginInput LoginInput
	syncResult SyncResult
	syncErr    error
}

func (p *fakePortal) StartBinding(context.Context) (Challenge, error) {
	if p.challenge.State.PasswordSalt == "" {
		p.challenge.State.PasswordSalt = "salt"
	}
	return p.challenge, nil
}

func (p *fakePortal) CompleteBinding(_ context.Context, _ ChallengeState, input LoginInput) (LoginResult, error) {
	p.loginInput = input
	if p.loginErr != nil {
		return LoginResult{}, p.loginErr
	}
	return p.login, nil
}

func (p *fakePortal) Sync(context.Context, SyncRequest) (SyncResult, error) {
	if p.syncErr != nil {
		return SyncResult{}, p.syncErr
	}
	return p.syncResult, nil
}

type fakeCipher struct{}

func (c *fakeCipher) Encrypt(value string) (string, error) { return "enc:" + value, nil }

func (c *fakeCipher) Decrypt(value string) (string, error) {
	if len(value) >= 4 && value[:4] == "enc:" {
		return value[4:], nil
	}
	return "", nil
}
