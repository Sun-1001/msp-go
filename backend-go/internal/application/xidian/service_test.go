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
	service := newTestService(repo, &fakePortal{})

	status, err := service.GetBindingStatus(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetBindingStatus(unbound) error = %v", err)
	}
	if status.IsBound {
		t.Fatalf("status = %#v", status)
	}

	verified := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)
	repo.account = Account{UserID: "user-1", Username: "student", LastVerifiedAt: &verified}
	repo.accountFound = true
	status, err = service.GetBindingStatus(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetBindingStatus(bound) error = %v", err)
	}
	if !status.IsBound || status.Username == nil || *status.Username != "student" || !status.LastVerifiedAt.Equal(verified) {
		t.Fatalf("status = %#v", status)
	}
}

func TestStartAndCompleteBindingStoresOnlyVerifiedIdentity(t *testing.T) {
	portal := &fakePortal{
		challenge: Challenge{CaptchaBig: "big", CaptchaPiece: "piece", PieceY: 12, State: ChallengeState{PasswordSalt: "salt"}},
	}
	repo := &fakeRepo{}
	service := newTestService(repo, portal)
	service.newID = func() (string, error) { return "challenge-1", nil }
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	start, err := service.StartBinding(context.Background())
	if err != nil {
		t.Fatalf("StartBinding() error = %v", err)
	}
	if start.ChallengeID != "challenge-1" || start.PuzzleWidth != 280 || start.PieceY != 12 {
		t.Fatalf("start = %#v", start)
	}
	username := " student "
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
	if repo.upsert.UserID != "user-1" || repo.upsert.Username != "student" || !repo.upsert.LastVerifiedAt.Equal(now) {
		t.Fatalf("upsert = %#v", repo.upsert)
	}
	if portal.loginInput.Username != "student" || portal.loginInput.Password != "password" || portal.loginInput.SliderPosition != 0.5 {
		t.Fatalf("login input = %#v", portal.loginInput)
	}
	if _, found, err := service.challenges.Get(context.Background(), "challenge-1"); err != nil || found {
		t.Fatalf("challenge found after success = %t, error = %v", found, err)
	}
}

func TestCompleteBindingReusesVerifiedUsernameButRequiresPassword(t *testing.T) {
	repo := &fakeRepo{accountFound: true, account: Account{Username: "student"}}
	portal := &fakePortal{}
	service := newTestService(repo, portal)
	if err := service.challenges.Set(context.Background(), "challenge-1", ChallengeState{PasswordSalt: "salt"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	password := "fresh-password"
	_, err := service.CompleteBinding(context.Background(), "user-1", CompleteBindingInput{
		ChallengeID:    "challenge-1",
		SliderPosition: 0.2,
		Password:       &password,
	})
	if err != nil {
		t.Fatalf("CompleteBinding() error = %v", err)
	}
	if portal.loginInput.Username != "student" || portal.loginInput.Password != "fresh-password" {
		t.Fatalf("login input = %#v", portal.loginInput)
	}
}

func TestCompleteBindingValidationErrors(t *testing.T) {
	username := "student"
	password := "password"
	tests := []struct {
		name    string
		repo    *fakeRepo
		prepare bool
		input   CompleteBindingInput
		code    string
	}{
		{
			name:  "slider outside range",
			repo:  &fakeRepo{},
			input: CompleteBindingInput{ChallengeID: "challenge-1", SliderPosition: 1.1, Username: &username, Password: &password},
			code:  "VALIDATION_ERROR",
		},
		{
			name:  "expired challenge",
			repo:  &fakeRepo{},
			input: CompleteBindingInput{ChallengeID: "challenge-1", SliderPosition: 0.5, Username: &username, Password: &password},
			code:  "CHALLENGE_EXPIRED",
		},
		{
			name:    "missing account",
			repo:    &fakeRepo{},
			prepare: true,
			input:   CompleteBindingInput{ChallengeID: "challenge-1", SliderPosition: 0.5, Password: &password},
			code:    "ACCOUNT_REQUIRED",
		},
		{
			name:    "missing password",
			repo:    &fakeRepo{},
			prepare: true,
			input:   CompleteBindingInput{ChallengeID: "challenge-1", SliderPosition: 0.5, Username: &username},
			code:    "PASSWORD_REQUIRED",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := newTestService(test.repo, &fakePortal{})
			if test.prepare {
				if err := service.challenges.Set(context.Background(), "challenge-1", ChallengeState{}, time.Minute); err != nil {
					t.Fatal(err)
				}
			}
			_, err := service.CompleteBinding(context.Background(), "user-1", test.input)
			var serviceErr ServiceError
			if !errors.As(err, &serviceErr) || serviceErr.Code != test.code {
				t.Fatalf("CompleteBinding() error = %#v, want code %s", err, test.code)
			}
		})
	}
}

func TestStartAndCompleteBindingReturnIDGenerationErrors(t *testing.T) {
	idErr := errors.New("random failed")
	service := newTestService(&fakeRepo{}, &fakePortal{})
	service.newID = func() (string, error) { return "", idErr }
	if _, err := service.StartBinding(context.Background()); !errors.Is(err, idErr) {
		t.Fatalf("StartBinding() error = %v, want %v", err, idErr)
	}

	service = newTestService(&fakeRepo{}, &fakePortal{})
	if err := service.challenges.Set(context.Background(), "challenge-1", ChallengeState{}, time.Minute); err != nil {
		t.Fatal(err)
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

func TestPortalServiceErrorsAreRedacted(t *testing.T) {
	portal := &fakePortal{
		loginErr: ServiceError{
			Code:    "PASSWORD_WRONG",
			Message: "登录失败 Authorization: Bearer portal-token url=https://ids.example.com/authserver/login?token=query-token",
			Status:  http.StatusUnauthorized,
			Err:     errors.New("api_key=plain cookie=sid-secret"),
		},
	}
	service := newTestService(&fakeRepo{}, portal)
	if err := service.challenges.Set(context.Background(), "challenge-1", ChallengeState{}, time.Minute); err != nil {
		t.Fatal(err)
	}
	username := "student"
	password := "password"
	_, err := service.CompleteBinding(context.Background(), "user-1", CompleteBindingInput{
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

func TestServiceDependencyValidationAndUnbind(t *testing.T) {
	repo := &fakeRepo{}
	service := newTestService(repo, &fakePortal{})
	if err := service.Unbind(context.Background(), "user-1"); err != nil {
		t.Fatalf("Unbind() error = %v", err)
	}
	if repo.deletedUserID != "user-1" {
		t.Fatalf("deleted user ID = %q", repo.deletedUserID)
	}

	config := Config{ChallengeTTL: time.Minute, CaptchaWidth: 1, CaptchaHeight: 1, PieceWidth: 1, PieceHeight: 1}
	if _, err := NewService(nil, &fakePortal{}, NewMemoryChallengeStore(), config); err == nil {
		t.Fatal("NewService(nil repo) error = nil")
	}
	if _, err := NewService(&fakeRepo{}, nil, NewMemoryChallengeStore(), config); err == nil {
		t.Fatal("NewService(nil client) error = nil")
	}
	if _, err := NewService(&fakeRepo{}, &fakePortal{}, nil, config); err == nil {
		t.Fatal("NewService(nil challenge store) error = nil")
	}
	if _, err := NewService(&fakeRepo{}, &fakePortal{}, NewMemoryChallengeStore(), Config{}); err == nil {
		t.Fatal("NewService(invalid config) error = nil")
	}
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
	if err := store.Delete(ctx, "third"); err != nil {
		t.Fatalf("Delete() error = %v", err)
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

func newTestService(repo Repository, portal PortalClient) *Service {
	service, err := NewService(repo, portal, NewMemoryChallengeStore(), Config{
		ChallengeTTL:  time.Minute,
		CaptchaWidth:  280,
		CaptchaHeight: 155,
		PieceWidth:    44,
		PieceHeight:   155,
	})
	if err != nil {
		panic(err)
	}
	return service
}

type fakeRepo struct {
	account       Account
	accountFound  bool
	upsert        AccountUpsert
	deletedUserID string
}

func (r *fakeRepo) GetAccount(context.Context, string) (Account, bool, error) {
	return r.account, r.accountFound, nil
}

func (r *fakeRepo) UpsertAccount(_ context.Context, input AccountUpsert) (Account, error) {
	r.upsert = input
	verified := input.LastVerifiedAt
	return Account{Username: input.Username, LastVerifiedAt: &verified}, nil
}

func (r *fakeRepo) DeleteAccount(_ context.Context, userID string) error {
	r.deletedUserID = userID
	return nil
}

type fakePortal struct {
	challenge  Challenge
	loginErr   error
	loginInput LoginInput
}

func (p *fakePortal) StartBinding(context.Context) (Challenge, error) {
	if p.challenge.State.PasswordSalt == "" {
		p.challenge.State.PasswordSalt = "salt"
	}
	return p.challenge, nil
}

func (p *fakePortal) CompleteBinding(_ context.Context, _ ChallengeState, input LoginInput) error {
	p.loginInput = input
	return p.loginErr
}
