package main

import (
	"context"
	"log/slog"
	"time"

	dailyquestionapp "mathstudy/backend/internal/application/dailyquestion"
)

func startDailyQuestionWorker(
	worker *dailyquestionapp.Worker,
	enabled bool,
	shutdownTimeout time.Duration,
	logger *slog.Logger,
) func() {
	if !enabled || worker == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := worker.Run(ctx); err != nil {
			logger.Error("daily question worker stopped", "error_code", "worker_error")
		}
	}()

	return func() {
		cancel()
		timer := time.NewTimer(shutdownTimeout)
		defer timer.Stop()
		select {
		case <-done:
			logger.Info("daily question worker stopped")
		case <-timer.C:
			logger.Warn("daily question worker shutdown timed out")
		}
	}
}

func startDailyQuestionReminderWorker(
	worker *dailyquestionapp.ReminderWorker,
	enabled bool,
	shutdownTimeout time.Duration,
	logger *slog.Logger,
) func() {
	if !enabled || worker == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := worker.Run(ctx); err != nil {
			logger.Error("daily question reminder worker stopped", "error_code", "worker_error")
		}
	}()

	return func() {
		cancel()
		timer := time.NewTimer(shutdownTimeout)
		defer timer.Stop()
		select {
		case <-done:
			logger.Info("daily question reminder worker stopped")
		case <-timer.C:
			logger.Warn("daily question reminder worker shutdown timed out")
		}
	}
}
