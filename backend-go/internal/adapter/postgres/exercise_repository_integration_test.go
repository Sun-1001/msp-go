package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestExerciseRepositoryForUpdateLockIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MSP_GO_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("set MSP_GO_TEST_DATABASE_URL to run PostgreSQL exercise lock integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()

	suffix := time.Now().UnixNano()
	teacherID := fmt.Sprintf("lock-teacher-%d", suffix)
	exerciseID := fmt.Sprintf("lock-exercise-%d", suffix)
	now := time.Now().UTC()
	_, err = pool.Exec(ctx, `
		INSERT INTO public.users (
			id, username, email, hashed_password, role, is_active, status, created_at, updated_at
		) VALUES ($1, $2, $3, 'integration-test', 'TEACHER', true, 'ACTIVE', $4, $4)`,
		teacherID,
		fmt.Sprintf("lock_teacher_%d", suffix),
		fmt.Sprintf("lock_teacher_%d@example.com", suffix),
		now,
	)
	if err != nil {
		t.Fatalf("insert teacher fixture: %v", err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.contents WHERE id = $1`, exerciseID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.users WHERE id = $1`, teacherID)
	}()
	_, err = pool.Exec(ctx, `
		INSERT INTO public.contents (
			id, type, owner_teacher_id, status, title, body, difficulty,
			concept_ids, tags, meta, created_at, updated_at
		) VALUES (
			$1, 'PROBLEM', $2, 'PUBLISHED', 'lock test', '1+0', 0.1,
			'[]'::json, '[]'::json,
			'{"type":"short_answer","answer":"1","answer_type":"numeric"}'::json,
			$3, $3
		)`,
		exerciseID,
		teacherID,
		now,
	)
	if err != nil {
		t.Fatalf("insert exercise fixture: %v", err)
	}

	lockTx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatalf("begin lock transaction: %v", err)
	}
	defer lockTx.Rollback(context.Background())
	repo, err := NewExerciseRepository(lockTx)
	if err != nil {
		t.Fatalf("NewExerciseRepository() error = %v", err)
	}
	if _, ok, err := repo.GetExerciseForUpdate(ctx, exerciseID); err != nil || !ok {
		t.Fatalf("GetExerciseForUpdate() ok=%t error=%v", ok, err)
	}

	updateTx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatalf("begin concurrent update transaction: %v", err)
	}
	updateCtx, updateCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	_, updateErr := updateTx.Exec(updateCtx, `
		UPDATE public.contents
		SET updated_at = updated_at + interval '1 microsecond'
		WHERE id = $1`,
		exerciseID,
	)
	updateCancel()
	_ = updateTx.Rollback(context.Background())
	if updateErr == nil || (!errors.Is(updateErr, context.DeadlineExceeded) && !errors.Is(updateErr, context.Canceled)) {
		t.Fatalf("concurrent update error = %v, want lock wait cancellation", updateErr)
	}

	if err := lockTx.Commit(ctx); err != nil {
		t.Fatalf("commit lock transaction: %v", err)
	}
	_, err = pool.Exec(ctx, `
		UPDATE public.contents
		SET updated_at = updated_at + interval '1 microsecond'
		WHERE id = $1`,
		exerciseID,
	)
	if err != nil {
		t.Fatalf("update after lock release: %v", err)
	}
}
