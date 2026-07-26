package main

import (
	"context"
	"log/slog"
	"time"

	wechatreminder "mathstudy/backend-go/internal/application/wechatreminder"
)

func startWechatReminderWorker(
	worker *wechatreminder.Worker,
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
			logger.Error("wechat reminder worker stopped", "error_code", "worker_error")
		}
	}()

	return func() {
		cancel()
		timer := time.NewTimer(shutdownTimeout)
		defer timer.Stop()
		select {
		case <-done:
			logger.Info("wechat reminder worker stopped")
		case <-timer.C:
			logger.Warn("wechat reminder worker shutdown timed out")
		}
	}
}
