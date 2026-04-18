package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier is implemented by pgx pools, connections, and transactions.
type Querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Repository stores the common database handle used by concrete repositories.
type Repository struct {
	db Querier
}

// NewRepository creates a base repository for domain-specific PostgreSQL adapters.
func NewRepository(db Querier) (Repository, error) {
	if db == nil {
		return Repository{}, errors.New("postgres querier is nil")
	}
	return Repository{db: db}, nil
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
