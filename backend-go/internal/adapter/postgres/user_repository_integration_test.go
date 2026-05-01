package postgres

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	admininboxapp "mathstudy/backend-go/internal/application/admininbox"
	authapp "mathstudy/backend-go/internal/application/auth"
	"mathstudy/backend-go/internal/domain/user"
)

func TestUserRepositoryIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MSP_GO_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("set MSP_GO_TEST_DATABASE_URL to run PostgreSQL user repository integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}
	defer tx.Rollback(context.Background())

	repo, err := NewUserRepository(tx)
	if err != nil {
		t.Fatalf("NewUserRepository() error = %v", err)
	}

	suffix := time.Now().UnixNano()
	account, err := repo.Create(ctx, user.CreateUser{
		ID:             fmt.Sprintf("test-user-%d", suffix),
		Username:       fmt.Sprintf("test_user_%d", suffix),
		Email:          fmt.Sprintf("test_user_%d@example.com", suffix),
		HashedPassword: "$2b$12$9x6kJZ77Z6u3Kz7Rkcl0Wuzx6E2UL6zLGCbyjEtW0QHfWkq0hPcN2",
		Role:           user.RoleTeacher,
		IsActive:       true,
		Status:         user.StatusActive,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if account.Role != user.RoleTeacher || account.Status != user.StatusActive {
		t.Fatalf("account = %#v", account)
	}

	byUsername, ok, err := repo.GetByUsername(ctx, account.Username)
	if err != nil {
		t.Fatalf("GetByUsername() error = %v", err)
	}
	if !ok || byUsername.ID != account.ID {
		t.Fatalf("GetByUsername() = %#v/%t", byUsername, ok)
	}

	settings, err := repo.RegistrationSettings(ctx)
	if err != nil {
		t.Fatalf("RegistrationSettings() error = %v", err)
	}
	if !settings.AllowStudent || !settings.AllowTeacher {
		t.Fatalf("RegistrationSettings() = %#v", settings)
	}

	requestID, err := repo.CreatePasswordResetRequest(ctx, authapp.PasswordResetRequest{
		UserID:    account.ID,
		Username:  account.Username,
		Email:     account.Email,
		Reason:    "integration",
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("CreatePasswordResetRequest() error = %v", err)
	}
	if requestID == "" {
		t.Fatal("CreatePasswordResetRequest() returned empty id")
	}

	pending, err := repo.HasPendingPasswordReset(ctx, account.ID)
	if err != nil {
		t.Fatalf("HasPendingPasswordReset() error = %v", err)
	}
	if !pending {
		t.Fatal("HasPendingPasswordReset() = false, want true")
	}

	status, ok, err := repo.LatestPasswordResetRequestStatus(ctx, account.Username, account.Email)
	if err != nil {
		t.Fatalf("LatestPasswordResetRequestStatus() error = %v", err)
	}
	if !ok || !status.HasPending || status.Status == nil || *status.Status != "pending" {
		t.Fatalf("LatestPasswordResetRequestStatus() = %#v/%t", status, ok)
	}

	requests, total, pendingCount, err := repo.ListPasswordResetRequests(ctx, admininboxapp.ListFilter{Status: "pending", Page: 1, PageSize: 100})
	if err != nil {
		t.Fatalf("ListPasswordResetRequests() error = %v", err)
	}
	if total < 1 || pendingCount < 1 || !containsPasswordResetRequest(requests, requestID) {
		t.Fatalf("ListPasswordResetRequests() total=%d pending=%d items=%#v", total, pendingCount, requests)
	}
	count, err := repo.CountPendingPasswordResetRequests(ctx)
	if err != nil {
		t.Fatalf("CountPendingPasswordResetRequests() error = %v", err)
	}
	if count < 1 {
		t.Fatalf("CountPendingPasswordResetRequests() = %d, want at least 1", count)
	}

	newHash := "$2b$12$06Eun3P3CyQRyGr8cbfLJejoV/j4bF9nB7aLBBPEy1nlYnjxncE.G"
	reviewedAt := time.Now().UTC()
	reviewResult, err := repo.ReviewPasswordResetRequest(ctx, admininboxapp.ReviewUpdate{
		RequestID:    requestID,
		AdminID:      account.ID,
		Action:       "approve",
		PasswordHash: &newHash,
		ReviewedAt:   reviewedAt,
	})
	if err != nil {
		t.Fatalf("ReviewPasswordResetRequest(approve) error = %v", err)
	}
	if !reviewResult.Found || !reviewResult.UserFound || reviewResult.AlreadyProcessed || reviewResult.Username != account.Username {
		t.Fatalf("ReviewPasswordResetRequest(approve) = %#v", reviewResult)
	}
	updated, ok, err := repo.GetByID(ctx, account.ID)
	if err != nil {
		t.Fatalf("GetByID(after review) error = %v", err)
	}
	if !ok || updated.HashedPassword != newHash {
		t.Fatalf("updated user = %#v ok=%t", updated, ok)
	}

	reviewResult, err = repo.ReviewPasswordResetRequest(ctx, admininboxapp.ReviewUpdate{
		RequestID:  requestID,
		AdminID:    account.ID,
		Action:     "reject",
		ReviewedAt: reviewedAt,
	})
	if err != nil {
		t.Fatalf("ReviewPasswordResetRequest(processed) error = %v", err)
	}
	if !reviewResult.Found || !reviewResult.AlreadyProcessed {
		t.Fatalf("ReviewPasswordResetRequest(processed) = %#v", reviewResult)
	}
}

func containsPasswordResetRequest(items []admininboxapp.RequestItem, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}
