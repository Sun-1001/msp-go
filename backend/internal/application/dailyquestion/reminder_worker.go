package dailyquestion

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

const (
	dailyReminderHour           = 8
	dailyReminderReconcileDelay = 5 * time.Minute
)

// ScheduledReminderDispatcher queues all due daily-question WeChat reminders.
type ScheduledReminderDispatcher interface {
	DispatchScheduledReminders(context.Context, time.Time) error
}

// ReminderWorker runs the WeChat-only reminder reconciliation at 08:00 in the
// platform's Shanghai calendar. Starting after 08:00 also reconciles today.
type ReminderWorker struct {
	dispatcher ScheduledReminderDispatcher
	logger     *slog.Logger
	now        func() time.Time
}

// NewReminderWorker creates the daily 08:00 reminder scheduler.
func NewReminderWorker(dispatcher ScheduledReminderDispatcher, logger *slog.Logger) (*ReminderWorker, error) {
	if dispatcher == nil {
		return nil, errors.New("daily question reminder dispatcher is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ReminderWorker{
		dispatcher: dispatcher,
		logger:     logger,
		now:        time.Now,
	}, nil
}

// Run waits for each 08:00 Shanghai boundary, then reconciles until the day
// ends. This picks up assignments that finish preparation after 08:00 while
// the durable per-recipient queue key prevents duplicate deliveries.
func (w *ReminderWorker) Run(ctx context.Context) error {
	now := w.now()
	nextRun := nextShanghaiReminderRun(now)
	if reminderTimeReached(now) {
		w.dispatch(ctx)
		nextRun = nextShanghaiReminderReconcile(now)
	}
	for {
		if err := waitForWorkerTimer(ctx, nextRun.Sub(w.now())); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		}
		if ctx.Err() != nil {
			return nil
		}
		now = w.now()
		w.dispatch(ctx)
		nextRun = nextShanghaiReminderReconcile(now)
	}
}

func (w *ReminderWorker) dispatch(ctx context.Context) bool {
	now := w.now()
	if err := w.dispatcher.DispatchScheduledReminders(ctx, now); err != nil && ctx.Err() == nil {
		w.logger.Error(
			"daily question reminder dispatch failed",
			"assignment_date", dateString(shanghaiDay(now)),
			"error_code", "repository_error",
		)
		return false
	}
	return ctx.Err() == nil
}

func nextShanghaiReminderRun(value time.Time) time.Time {
	local := value.In(shanghaiLocation)
	runAt := time.Date(local.Year(), local.Month(), local.Day(), dailyReminderHour, 0, 0, 0, shanghaiLocation)
	if local.Before(runAt) {
		return runAt
	}
	return runAt.AddDate(0, 0, 1)
}

func nextShanghaiReminderReconcile(value time.Time) time.Time {
	retryAt := value.Add(dailyReminderReconcileDelay)
	if shanghaiDay(retryAt).Equal(shanghaiDay(value)) {
		return retryAt
	}
	return nextShanghaiReminderRun(value)
}
