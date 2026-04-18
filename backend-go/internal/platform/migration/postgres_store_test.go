package migration

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNewPostgresStoreWithTableValidatesInput(t *testing.T) {
	if _, err := NewPostgresStoreWithTable(nil, "go_schema_migrations"); err == nil {
		t.Fatal("NewPostgresStoreWithTable(nil) error = nil, want error")
	}
	if _, err := NewPostgresStoreWithTable(&pgxpool.Pool{}, "go_schema_migrations;DROP"); err == nil {
		t.Fatal("NewPostgresStoreWithTable() accepted unsafe table name")
	}
}
