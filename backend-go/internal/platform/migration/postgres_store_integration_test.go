package migration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresStoreIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MSP_GO_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("set MSP_GO_TEST_DATABASE_URL to run PostgreSQL migration integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()

	tableName := fmt.Sprintf("go_schema_migrations_test_%d", time.Now().UnixNano())
	store, err := NewPostgresStoreWithTable(pool, tableName)
	if err != nil {
		t.Fatalf("NewPostgresStoreWithTable() error = %v", err)
	}
	defer pool.Exec(context.Background(), "DROP TABLE IF EXISTS "+tableName)

	runner, err := NewRunner(store, []Migration{
		{Version: 1, Name: "integration", SQL: "SELECT 1;"},
	})
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	applied, err := runner.Up(ctx)
	if err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	if len(applied) != 1 {
		t.Fatalf("applied migrations = %d, want 1", len(applied))
	}

	applied, err = runner.Up(ctx)
	if err != nil {
		t.Fatalf("second Up() error = %v", err)
	}
	if len(applied) != 0 {
		t.Fatalf("second Up() applied migrations = %d, want 0", len(applied))
	}
}
