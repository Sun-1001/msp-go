package migration

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultTableName = "go_schema_migrations"

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// PostgresStore stores migration state in PostgreSQL.
type PostgresStore struct {
	pool      *pgxpool.Pool
	tableName string
}

// NewPostgresStore creates a PostgreSQL-backed migration store.
func NewPostgresStore(pool *pgxpool.Pool) (PostgresStore, error) {
	return NewPostgresStoreWithTable(pool, defaultTableName)
}

// NewPostgresStoreWithTable creates a migration store with an explicit table name.
func NewPostgresStoreWithTable(pool *pgxpool.Pool, tableName string) (PostgresStore, error) {
	if pool == nil {
		return PostgresStore{}, errors.New("postgres pool is nil")
	}
	if !identifierPattern.MatchString(tableName) {
		return PostgresStore{}, fmt.Errorf("invalid migration table name %q", tableName)
	}
	return PostgresStore{pool: pool, tableName: tableName}, nil
}

// Ensure creates the migration history table when needed.
func (s PostgresStore) Ensure(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
	version BIGINT PRIMARY KEY,
	name TEXT NOT NULL,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`, s.qualifiedTableName()))
	return err
}

// AppliedMigrations returns the applied migration names keyed by version.
func (s PostgresStore) AppliedMigrations(ctx context.Context) (map[int64]string, error) {
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`SELECT version, name FROM %s`, s.qualifiedTableName()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int64]string)
	for rows.Next() {
		var version int64
		var name string
		if err := rows.Scan(&version, &name); err != nil {
			return nil, err
		}
		applied[version] = name
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return applied, nil
}

// Apply executes a migration and records it in the history table in one transaction.
func (s PostgresStore) Apply(ctx context.Context, migration Migration) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, migration.SQL, pgx.QueryExecModeSimpleProtocol); err != nil {
		return err
	}
	if _, err := tx.Exec(
		ctx,
		fmt.Sprintf(`INSERT INTO %s (version, name) VALUES ($1, $2)`, s.qualifiedTableName()),
		migration.Version,
		migration.Name,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s PostgresStore) qualifiedTableName() string {
	return "public." + s.tableName
}
