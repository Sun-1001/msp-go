package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"mathstudy/backend/internal/platform/config"
	"mathstudy/backend/internal/platform/migration"
	platformpostgres "mathstudy/backend/internal/platform/postgres"
	"mathstudy/backend/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	result, mode, err := run(context.Background(), os.Args[1:], os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		logger.Error("reconcile migration ledger", "error", err)
		os.Exit(1)
	}
	logger.Info(
		"migration ledger reconciliation complete",
		"mode", mode,
		"legacy_rows", result.LegacyRows,
		"applied", result.Applied,
		"deleted_rows", result.DeletedRows,
	)
	if result.Applied {
		logger.Info("run the ordinary migration command twice to apply all pending canonical migrations and verify idempotency")
	}
}

func run(ctx context.Context, args []string, output io.Writer) (migration.ReconcileResult, string, error) {
	flags := flag.NewFlagSet("migrate-reconcile", flag.ContinueOnError)
	flags.SetOutput(output)
	check := flags.Bool("check", false, "validate the exact legacy ledger and schema without mutation")
	apply := flags.Bool("apply", false, "reconcile only the validated legacy migration metadata")
	confirm := flags.String("confirm", "", "exact confirmation token required with --apply")
	flags.Usage = func() {
		fmt.Fprintf(output, "Usage: go run ./cmd/migrate-reconcile [--check | --apply --confirm %s]\n", migration.LegacyMistakeBookConfirmation)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return migration.ReconcileResult{}, "", err
	}
	if flags.NArg() != 0 {
		return migration.ReconcileResult{}, "", fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	if *check && *apply {
		return migration.ReconcileResult{}, "", errors.New("--check and --apply cannot be used together")
	}
	if !*apply && *confirm != "" {
		return migration.ReconcileResult{}, "", errors.New("--confirm requires --apply")
	}

	cfg, err := config.Load()
	if err != nil {
		return migration.ReconcileResult{}, "", fmt.Errorf("load config: %w", err)
	}
	pool, err := platformpostgres.NewPool(ctx, cfg)
	if err != nil {
		return migration.ReconcileResult{}, "", fmt.Errorf("configure postgres pool: %w", err)
	}
	defer pool.Close()

	loaded, err := migrations.Load()
	if err != nil {
		return migration.ReconcileResult{}, "", fmt.Errorf("load migrations: %w", err)
	}
	result, err := migration.ReconcileLegacyMistakeBookLedger(ctx, pool, loaded, migration.ReconcileOptions{
		Apply:        *apply,
		Confirmation: *confirm,
		Environment:  cfg.Environment,
	})
	mode := "check"
	if *apply {
		mode = "apply"
	}
	if err != nil {
		return migration.ReconcileResult{}, mode, err
	}
	return result, mode, nil
}
