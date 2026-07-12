package postgres

import (
	"context"
	"math"
	"testing"
	"time"

	"mathstudy/backend-go/internal/platform/config"
)

func TestNewPoolAppliesConfiguredPoolOptions(t *testing.T) {
	cfg := config.Config{
		PostgresHost:        "localhost",
		PostgresPort:        5432,
		PostgresUser:        "postgres",
		PostgresPassword:    "postgres",
		PostgresDB:          "math_platform",
		DBPoolSize:          7,
		DBPoolMinConns:      2,
		DBPoolRecycle:       10 * time.Minute,
		DBConnectTimeout:    3 * time.Second,
		DBStatementTimeout:  1500 * time.Millisecond,
		DBIdleTxTimeout:     45 * time.Second,
		DBHealthCheckPeriod: 11 * time.Second,
	}

	pool, err := NewPool(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	got := pool.Config()
	if got.MaxConns != 7 {
		t.Fatalf("MaxConns = %d", got.MaxConns)
	}
	if got.MinConns != 2 {
		t.Fatalf("MinConns = %d", got.MinConns)
	}
	if got.ConnConfig.ConnectTimeout != 3*time.Second {
		t.Fatalf("ConnectTimeout = %s", got.ConnConfig.ConnectTimeout)
	}
	if got.ConnConfig.RuntimeParams["statement_timeout"] != "1500" {
		t.Fatalf("statement_timeout = %q", got.ConnConfig.RuntimeParams["statement_timeout"])
	}
	if got.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] != "45000" {
		t.Fatalf("idle_in_transaction_session_timeout = %q", got.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"])
	}
}

func TestNewPoolRejectsInvalidPoolLimits(t *testing.T) {
	tests := []struct {
		name     string
		maxConns int
		minConns int
	}{
		{name: "zero max", maxConns: 0},
		{name: "negative min", maxConns: 1, minConns: -1},
		{name: "min exceeds max", maxConns: 1, minConns: 2},
		{name: "max overflows int32", maxConns: int(math.MaxInt32) + 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := config.Config{DBPoolSize: test.maxConns, DBPoolMinConns: test.minConns}
			if _, err := NewPool(context.Background(), cfg); err == nil {
				t.Fatal("NewPool() error = nil, want invalid pool limits error")
			}
		})
	}
}

func TestNewTransactorRejectsNilPool(t *testing.T) {
	if _, err := NewTransactor(nil); err == nil {
		t.Fatal("NewTransactor(nil) error = nil, want error")
	}
}
