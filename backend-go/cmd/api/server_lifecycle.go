package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type lifecycleServer interface {
	ListenAndServe() error
	Shutdown(context.Context) error
}

func serveHTTP(server lifecycleServer, stopCh <-chan os.Signal, shutdownTimeout time.Duration, logger *slog.Logger) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case sig := <-stopCh:
		if sig != nil {
			logger.Info("shutdown requested", "signal", sig.String())
		} else {
			logger.Info("shutdown requested")
		}
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped", "error", err)
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		return err
	}
	logger.Info("server shutdown complete")
	return nil
}
