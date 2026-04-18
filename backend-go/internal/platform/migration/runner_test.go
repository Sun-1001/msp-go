package migration

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type memoryStore struct {
	ensured bool
	applied map[int64]struct{}
	names   []string
	err     error
}

func (s *memoryStore) Ensure(context.Context) error {
	s.ensured = true
	return s.err
}

func (s *memoryStore) AppliedVersions(context.Context) (map[int64]struct{}, error) {
	if s.err != nil {
		return nil, s.err
	}
	result := make(map[int64]struct{}, len(s.applied))
	for version := range s.applied {
		result[version] = struct{}{}
	}
	return result, nil
}

func (s *memoryStore) Apply(_ context.Context, migration Migration) error {
	if s.err != nil {
		return s.err
	}
	s.applied[migration.Version] = struct{}{}
	s.names = append(s.names, migration.Name)
	return nil
}

func TestRunnerPendingAndUp(t *testing.T) {
	store := &memoryStore{applied: map[int64]struct{}{1: {}}}
	runner, err := NewRunner(store, []Migration{
		{Version: 2, Name: "second", SQL: "SELECT 2;"},
		{Version: 1, Name: "first", SQL: "SELECT 1;"},
	})
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	pending, err := runner.Pending(context.Background())
	if err != nil {
		t.Fatalf("Pending() error = %v", err)
	}
	if len(pending) != 1 || pending[0].Version != 2 {
		t.Fatalf("Pending() = %#v, want version 2", pending)
	}

	applied, err := runner.Up(context.Background())
	if err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	if !store.ensured {
		t.Fatal("store was not ensured")
	}
	if len(applied) != 1 || applied[0].Name != "second" {
		t.Fatalf("Up() = %#v, want second migration", applied)
	}
	if !reflect.DeepEqual(store.names, []string{"second"}) {
		t.Fatalf("applied names = %#v", store.names)
	}
}

func TestNewRunnerRejectsNilStore(t *testing.T) {
	if _, err := NewRunner(nil, nil); err == nil {
		t.Fatal("NewRunner(nil, nil) error = nil, want error")
	}
}

func TestRunnerReturnsStoreError(t *testing.T) {
	wantErr := errors.New("store failed")
	runner, err := NewRunner(&memoryStore{applied: map[int64]struct{}{}, err: wantErr}, []Migration{
		{Version: 1, Name: "first", SQL: "SELECT 1;"},
	})
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	if _, err := runner.Pending(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("Pending() error = %v, want %v", err, wantErr)
	}
}
