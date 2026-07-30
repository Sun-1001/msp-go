package migration

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// Store persists migration state and applies migration SQL atomically.
type Store interface {
	Ensure(context.Context) error
	AppliedMigrations(context.Context) (map[int64]string, error)
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
	applied, err := r.store.AppliedMigrations(ctx)
	if err != nil {
		return nil, fmt.Errorf("read applied migrations: %w", err)
	}
	if err := r.validateAppliedMigrations(applied); err != nil {
		return nil, err
	}
	pending := make([]Migration, 0)
	for _, migration := range r.migrations {
		if _, ok := applied[migration.Version]; !ok {
			pending = append(pending, migration)
		}
	}
	return pending, nil
}

func (r Runner) validateAppliedMigrations(applied map[int64]string) error {
	expected := make(map[int64]string, len(r.migrations))
	for _, migration := range r.migrations {
		expected[migration.Version] = migration.Name
		if name, ok := applied[migration.Version]; ok && name != migration.Name {
			return fmt.Errorf(
				"migration %d name mismatch: database has %q, application expects %q",
				migration.Version,
				name,
				migration.Name,
			)
		}
	}

	unknown := make([]int64, 0)
	for version := range applied {
		if _, ok := expected[version]; !ok {
			unknown = append(unknown, version)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Slice(unknown, func(i, j int) bool { return unknown[i] < unknown[j] })
	version := unknown[0]
	return fmt.Errorf(
		"database migration %d_%s is not present in this application",
		version,
		applied[version],
	)
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
