package main

import (
	"context"
	"log/slog"
	"os"

	"mathstudy/backend-go/internal/platform/config"
	"mathstudy/backend-go/internal/platform/migration"
	platformpostgres "mathstudy/backend-go/internal/platform/postgres"
	"mathstudy/backend-go/migrations"
)

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	pool, err := platformpostgres.NewPool(ctx, cfg)
	if err != nil {
		logger.Error("configure postgres pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	store, err := migration.NewPostgresStore(pool)
	if err != nil {
		logger.Error("configure migration store", "error", err)
		os.Exit(1)
	}
	loaded, err := migrations.Load()
	if err != nil {
		logger.Error("load migrations", "error", err)
		os.Exit(1)
	}
	runner, err := migration.NewRunner(store, loaded)
	if err != nil {
		logger.Error("configure migration runner", "error", err)
		os.Exit(1)
	}

	applied, err := runner.Up(ctx)
	if err != nil {
		logger.Error("apply migrations", "error", err)
		os.Exit(1)
	}
	logger.Info("migrations complete", "applied_count", len(applied))
	for _, item := range applied {
		logger.Info("migration applied", "version", item.Version, "name", item.Name)
	}
}
