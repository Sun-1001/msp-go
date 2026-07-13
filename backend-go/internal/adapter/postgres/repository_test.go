package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeQuerier struct {
	row pgx.Row
}

func (f fakeQuerier) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f fakeQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (f fakeQuerier) QueryRow(context.Context, string, ...any) pgx.Row {
	return f.row
}

type fakeRepositoryTx struct {
	pgx.Tx
	commitErr   error
	rollbackErr error
	commits     int
	rollbacks   int
}

func (f *fakeRepositoryTx) Commit(context.Context) error {
	f.commits++
	return f.commitErr
}

func (f *fakeRepositoryTx) Rollback(context.Context) error {
	f.rollbacks++
	return f.rollbackErr
}

type fakeRepositoryBeginner struct {
	fakeQuerier
	tx       pgx.Tx
	beginErr error
	begins   int
}

func (f *fakeRepositoryBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	f.begins++
	return f.tx, f.beginErr
}

type boolRow struct {
	value bool
	err   error
}

func (r boolRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	target := dest[0].(*bool)
	*target = r.value
	return nil
}

func TestNewRepositoryRejectsNilQuerier(t *testing.T) {
	if _, err := NewRepository(nil); err == nil {
		t.Fatal("NewRepository(nil) error = nil, want error")
	}
}

func TestRepositoryExistsScansBoolean(t *testing.T) {
	repo, err := NewRepository(fakeQuerier{row: boolRow{value: true}})
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}

	exists, err := repo.Exists(context.Background(), "select exists(select 1)")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Fatal("Exists() = false, want true")
	}
}

func TestRepositoryExistsReturnsScanError(t *testing.T) {
	wantErr := errors.New("scan failed")
	repo, err := NewRepository(fakeQuerier{row: boolRow{err: wantErr}})
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}

	_, err = repo.Exists(context.Background(), "select exists(select 1)")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Exists() error = %v, want %v", err, wantErr)
	}
}

func TestWithRepositoryTxUsesCurrentRepositoryWithoutBeginner(t *testing.T) {
	repo, err := NewRepository(fakeQuerier{})
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}

	called := false
	err = withRepositoryTx(context.Background(), "test", repo, func(base Repository) Repository {
		return base
	}, func(current Repository) error {
		called = true
		if current.DB() != repo.DB() {
			t.Fatal("current repository DB differs from source DB")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withRepositoryTx() error = %v", err)
	}
	if !called {
		t.Fatal("withRepositoryTx() did not call function")
	}
}

func TestWithRepositoryTxCommitsOnSuccess(t *testing.T) {
	tx := &fakeRepositoryTx{}
	beginner := &fakeRepositoryBeginner{tx: tx}
	repo, err := NewRepository(beginner)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}

	err = withRepositoryTx(context.Background(), "test", repo, func(base Repository) Repository {
		return base
	}, func(current Repository) error {
		if current.DB() != tx {
			t.Fatal("transaction repository does not use begun transaction")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withRepositoryTx() error = %v", err)
	}
	if beginner.begins != 1 || tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("begins/commits/rollbacks = %d/%d/%d, want 1/1/0", beginner.begins, tx.commits, tx.rollbacks)
	}
}

func TestWithRepositoryTxRollsBackFunctionError(t *testing.T) {
	wantErr := errors.New("operation failed")
	tx := &fakeRepositoryTx{}
	repo, err := NewRepository(&fakeRepositoryBeginner{tx: tx})
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}

	err = withRepositoryTx(context.Background(), "test", repo, func(base Repository) Repository {
		return base
	}, func(Repository) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("withRepositoryTx() error = %v, want %v", err, wantErr)
	}
	if tx.commits != 0 || tx.rollbacks != 1 {
		t.Fatalf("commits/rollbacks = %d/%d, want 0/1", tx.commits, tx.rollbacks)
	}
}

func TestWithRepositoryTxReturnsBeginError(t *testing.T) {
	wantErr := errors.New("begin failed")
	repo, err := NewRepository(&fakeRepositoryBeginner{beginErr: wantErr})
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}

	err = withRepositoryTx(context.Background(), "test", repo, func(base Repository) Repository {
		return base
	}, func(Repository) error {
		return nil
	})
	if !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "begin test transaction") {
		t.Fatalf("withRepositoryTx() error = %v, want wrapped begin error", err)
	}
}

func TestWithRepositoryTxJoinsCommitAndRollbackErrors(t *testing.T) {
	tx := &fakeRepositoryTx{
		commitErr:   errors.New("commit failed"),
		rollbackErr: errors.New("rollback failed"),
	}
	repo, err := NewRepository(&fakeRepositoryBeginner{tx: tx})
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}

	err = withRepositoryTx(context.Background(), "test", repo, func(base Repository) Repository {
		return base
	}, func(Repository) error {
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "commit test transaction") || !strings.Contains(err.Error(), "rollback test transaction") {
		t.Fatalf("withRepositoryTx() error = %v, want joined commit and rollback errors", err)
	}
	if tx.commits != 1 || tx.rollbacks != 1 {
		t.Fatalf("commits/rollbacks = %d/%d, want 1/1", tx.commits, tx.rollbacks)
	}
}

func TestWithRepositoryTxRejectsNilFunction(t *testing.T) {
	repo, err := NewRepository(fakeQuerier{})
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}

	err = withRepositoryTx[Repository](context.Background(), "test", repo, func(base Repository) Repository {
		return base
	}, nil)
	if err == nil || err.Error() != "test transaction function is nil" {
		t.Fatalf("withRepositoryTx(nil) error = %v", err)
	}
}
