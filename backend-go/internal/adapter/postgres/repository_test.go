package postgres

import (
	"context"
	"errors"
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
