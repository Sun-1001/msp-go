package migrations

import "testing"

func TestLoadIncludesBaselineMigration(t *testing.T) {
	migrations, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("Load() returned no migrations")
	}
	if migrations[0].Version != 1 || migrations[0].Name != "initial_schema" {
		t.Fatalf("first migration = %#v, want 0001_initial_schema", migrations[0])
	}
}
