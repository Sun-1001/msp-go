package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeTx struct {
	commitErr   error
	rollbackErr error
	commits     int
	rollbacks   int
}

func (f *fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (f *fakeTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return nil
}

func (f *fakeTx) Commit(context.Context) error {
	f.commits++
	return f.commitErr
}

func (f *fakeTx) Rollback(context.Context) error {
	f.rollbacks++
	return f.rollbackErr
}

func TestWithTxCommitsOnSuccess(t *testing.T) {
	tx := &fakeTx{}
	transactor, err := newTransactor(func(context.Context, pgx.TxOptions) (Tx, error) {
		return tx, nil
	}, pgx.TxOptions{})
	if err != nil {
		t.Fatalf("newTransactor() error = %v", err)
	}

	if err := transactor.WithTx(context.Background(), func(context.Context, Tx) error {
		return nil
	}); err != nil {
		t.Fatalf("WithTx() error = %v", err)
	}

	if tx.commits != 1 {
		t.Fatalf("commits = %d, want 1", tx.commits)
	}
	if tx.rollbacks != 0 {
		t.Fatalf("rollbacks = %d, want 0", tx.rollbacks)
	}
}

func TestWithTxRollsBackOnFunctionError(t *testing.T) {
	wantErr := errors.New("use case failed")
	tx := &fakeTx{}
	transactor, err := newTransactor(func(context.Context, pgx.TxOptions) (Tx, error) {
		return tx, nil
	}, pgx.TxOptions{})
	if err != nil {
		t.Fatalf("newTransactor() error = %v", err)
	}

	err = transactor.WithTx(context.Background(), func(context.Context, Tx) error {
		return wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("WithTx() error = %v, want %v", err, wantErr)
	}
	if tx.commits != 0 {
		t.Fatalf("commits = %d, want 0", tx.commits)
	}
	if tx.rollbacks != 1 {
		t.Fatalf("rollbacks = %d, want 1", tx.rollbacks)
	}
}

func TestWithTxJoinsRollbackError(t *testing.T) {
	tx := &fakeTx{rollbackErr: errors.New("rollback failed")}
	transactor, err := newTransactor(func(context.Context, pgx.TxOptions) (Tx, error) {
		return tx, nil
	}, pgx.TxOptions{})
	if err != nil {
		t.Fatalf("newTransactor() error = %v", err)
	}

	err = transactor.WithTx(context.Background(), func(context.Context, Tx) error {
		return errors.New("use case failed")
	})

	if err == nil {
		t.Fatal("WithTx() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "use case failed") || !strings.Contains(err.Error(), "rollback failed") {
		t.Fatalf("WithTx() error = %v, want joined use case and rollback errors", err)
	}
}

func TestWithTxRollsBackOnCommitError(t *testing.T) {
	tx := &fakeTx{commitErr: errors.New("commit failed")}
	transactor, err := newTransactor(func(context.Context, pgx.TxOptions) (Tx, error) {
		return tx, nil
	}, pgx.TxOptions{})
	if err != nil {
		t.Fatalf("newTransactor() error = %v", err)
	}

	err = transactor.WithTx(context.Background(), func(context.Context, Tx) error {
		return nil
	})

	if err == nil || !strings.Contains(err.Error(), "commit transaction") {
		t.Fatalf("WithTx() error = %v, want commit error", err)
	}
	if tx.rollbacks != 1 {
		t.Fatalf("rollbacks = %d, want 1", tx.rollbacks)
	}
}

func TestWithTxRejectsNilFunction(t *testing.T) {
	transactor, err := newTransactor(func(context.Context, pgx.TxOptions) (Tx, error) {
		return &fakeTx{}, nil
	}, pgx.TxOptions{})
	if err != nil {
		t.Fatalf("newTransactor() error = %v", err)
	}

	if err := transactor.WithTx(context.Background(), nil); err == nil {
		t.Fatal("WithTx(nil) error = nil, want error")
	}
}
