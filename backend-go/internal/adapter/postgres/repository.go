package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier is implemented by pgx pools, connections, and transactions.
type Querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type pgxTxBeginner interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

// Repository stores the common database handle used by concrete repositories.
type Repository struct {
	db       Querier
	beginner pgxTxBeginner
}

// NewRepository creates a base repository for domain-specific PostgreSQL adapters.
func NewRepository(db Querier) (Repository, error) {
	if db == nil {
		return Repository{}, errors.New("postgres querier is nil")
	}
	repository := Repository{db: db}
	if beginner, ok := db.(pgxTxBeginner); ok {
		repository.beginner = beginner
	}
	return repository, nil
}

// DB returns the underlying pgx-compatible query handle.
func (r Repository) DB() Querier {
	return r.db
}

// Exists executes a SELECT EXISTS style query and scans the boolean result.
func (r Repository) Exists(ctx context.Context, sql string, args ...any) (bool, error) {
	var exists bool
	if err := r.db.QueryRow(ctx, sql, args...).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func withRepositoryTx[T any](
	ctx context.Context,
	scope string,
	base Repository,
	wrap func(Repository) T,
	fn func(T) error,
) error {
	if fn == nil {
		return errors.New(scope + " transaction function is nil")
	}
	if base.beginner == nil {
		return fn(wrap(base))
	}

	tx, err := base.beginner.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin %s transaction: %w", scope, err)
	}
	txBase, err := NewRepository(tx)
	if err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := fn(wrap(txBase)); err != nil {
		return rollbackRepositoryTx(ctx, tx, scope, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return rollbackRepositoryTx(ctx, tx, scope, fmt.Errorf("commit %s transaction: %w", scope, err))
	}
	return nil
}

func rollbackRepositoryTx(ctx context.Context, tx pgx.Tx, scope string, cause error) error {
	if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
		return errors.Join(cause, fmt.Errorf("rollback %s transaction: %w", scope, rollbackErr))
	}
	return cause
}
