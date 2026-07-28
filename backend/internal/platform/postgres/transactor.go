package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Tx is the transaction surface used by application services and repositories.
type Tx interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Commit(context.Context) error
	Rollback(context.Context) error
}

type beginFunc func(context.Context, pgx.TxOptions) (Tx, error)

// Transactor provides a single transaction boundary pattern for use cases.
type Transactor struct {
	begin beginFunc
	opts  pgx.TxOptions
}

// NewTransactor creates a transaction helper backed by the shared pgx pool.
func NewTransactor(pool *pgxpool.Pool) (Transactor, error) {
	if pool == nil {
		return Transactor{}, errors.New("postgres pool is nil")
	}
	return newTransactor(func(ctx context.Context, opts pgx.TxOptions) (Tx, error) {
		return pool.BeginTx(ctx, opts)
	}, pgx.TxOptions{})
}

func newTransactor(begin beginFunc, opts pgx.TxOptions) (Transactor, error) {
	if begin == nil {
		return Transactor{}, errors.New("postgres transaction begin function is nil")
	}
	return Transactor{begin: begin, opts: opts}, nil
}

// WithTx runs fn in a transaction and commits only when fn returns nil.
func (t Transactor) WithTx(ctx context.Context, fn func(context.Context, Tx) error) error {
	return t.WithTxOptions(ctx, t.opts, fn)
}

// WithTxOptions runs fn in a transaction with explicit pgx transaction options.
func (t Transactor) WithTxOptions(ctx context.Context, opts pgx.TxOptions, fn func(context.Context, Tx) error) error {
	if fn == nil {
		return errors.New("postgres transaction function is nil")
	}
	tx, err := t.begin(ctx, opts)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(ctx, tx); err != nil {
		return joinRollback(ctx, tx, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return joinRollback(ctx, tx, fmt.Errorf("commit transaction: %w", err))
	}
	return nil
}

func joinRollback(ctx context.Context, tx Tx, cause error) error {
	if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
		return errors.Join(cause, fmt.Errorf("rollback transaction: %w", rollbackErr))
	}
	return cause
}
