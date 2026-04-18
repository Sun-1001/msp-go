package migration

import (
	"context"
	"errors"
	"fmt"
)

// Store persists migration state and applies migration SQL atomically.
type Store interface {
	Ensure(context.Context) error
	AppliedVersions(context.Context) (map[int64]struct{}, error)
	Apply(context.Context, Migration) error
}

// Runner compares embedded migrations with persisted state and applies pending migrations.
type Runner struct {
	store      Store
	migrations []Migration
}

// NewRunner creates a migration runner.
func NewRunner(store Store, migrations []Migration) (Runner, error) {
	if store == nil {
		return Runner{}, errors.New("migration store is nil")
	}
	ordered := append([]Migration(nil), migrations...)
	if err := validateMigrations(ordered); err != nil {
		return Runner{}, err
	}
	return Runner{store: store, migrations: ordered}, nil
}

// Pending returns migrations that have not been recorded as applied.
func (r Runner) Pending(ctx context.Context) ([]Migration, error) {
	if err := r.store.Ensure(ctx); err != nil {
		return nil, fmt.Errorf("ensure migration store: %w", err)
	}
	applied, err := r.store.AppliedVersions(ctx)
	if err != nil {
		return nil, fmt.Errorf("read applied migrations: %w", err)
	}
	pending := make([]Migration, 0)
	for _, migration := range r.migrations {
		if _, ok := applied[migration.Version]; !ok {
			pending = append(pending, migration)
		}
	}
	return pending, nil
}

// Up applies all pending migrations in version order and returns the applied list.
func (r Runner) Up(ctx context.Context) ([]Migration, error) {
	pending, err := r.Pending(ctx)
	if err != nil {
		return nil, err
	}
	for _, migration := range pending {
		if err := r.store.Apply(ctx, migration); err != nil {
			return nil, fmt.Errorf("apply migration %d_%s: %w", migration.Version, migration.Name, err)
		}
	}
	return pending, nil
}
