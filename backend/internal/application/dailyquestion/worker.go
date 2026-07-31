package dailyquestion

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	airiskapp "mathstudy/backend/internal/application/airisk"
)

const (
	defaultWorkerBatchSize      = 100
	defaultWorkerConcurrency    = 8
	defaultWorkerStudentTimeout = time.Minute
	maxWorkerPageAttempts       = 3
	dailyWorkerRetryDelay       = 5 * time.Minute
)

// ActiveStudentLister provides stable keyset pages for one midnight run.
type ActiveStudentLister interface {
	ListActiveStudentIDs(context.Context, string, int) ([]string, error)
}

// DailyPreparer assigns a question without recording a student page visit.
type DailyPreparer interface {
	PrepareTodayInBackground(context.Context, string) (TodayResponse, error)
}

// WorkerConfig bounds each database page and the number of simultaneous preparations.
type WorkerConfig struct {
	BatchSize      int
	Concurrency    int
	BatchInterval  time.Duration
	StudentTimeout time.Duration
}

// Worker prepares one fixed question for every active student at Shanghai midnight.
type Worker struct {
	repository ActiveStudentLister
	preparer   DailyPreparer
	logger     *slog.Logger
	config     WorkerConfig
	now        func() time.Time
}

// NewWorker creates a daily preparation worker. Run must be started by the application lifecycle.
func NewWorker(repository ActiveStudentLister, preparer DailyPreparer, logger *slog.Logger, cfg WorkerConfig) (*Worker, error) {
	if repository == nil {
		return nil, errors.New("daily question worker repository is nil")
	}
	if preparer == nil {
		return nil, errors.New("daily question worker preparer is nil")
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultWorkerBatchSize
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = defaultWorkerConcurrency
	}
	if cfg.Concurrency > cfg.BatchSize {
		cfg.Concurrency = cfg.BatchSize
	}
	if cfg.BatchInterval < 0 {
		return nil, errors.New("daily question worker batch interval must not be negative")
	}
	if cfg.StudentTimeout <= 0 {
		cfg.StudentTimeout = defaultWorkerStudentTimeout
	}
	if cfg.StudentTimeout >= defaultStalePreparation {
		return nil, fmt.Errorf("daily question worker student timeout must be shorter than %s", defaultStalePreparation)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{
		repository: repository,
		preparer:   preparer,
		logger:     logger,
		config:     cfg,
		now:        time.Now,
	}, nil
}

// Run catches up the current Shanghai day after startup, then prepares every
// following day at midnight. Reprocessing is safe because preparation reserves
// one unique student-day assignment before generating any content.
func (w *Worker) Run(ctx context.Context) error {
	nextRun := shanghaiDay(w.now())
	for {
		now := w.now()
		if err := waitForWorkerTimer(ctx, nextRun.Sub(now)); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		}
		if ctx.Err() != nil {
			return nil
		}

		runDate := shanghaiDay(w.now())
		w.logger.Info("daily question batch started", "assignment_date", dateString(runDate))
		retryNeeded, err := w.prepareDate(ctx, runDate)
		if ctx.Err() != nil {
			return nil
		}
		now = w.now()
		if !shanghaiDay(now).Equal(runDate) {
			w.logger.Info("daily question batch stopped at day boundary", "assignment_date", dateString(runDate))
			nextRun = shanghaiDay(now)
			continue
		}
		if err != nil {
			w.logger.Error(
				"daily question batch failed",
				"assignment_date", dateString(runDate),
				"error_code", "repository_error",
			)
			nextRun = nextShanghaiWorkerRetry(now)
			continue
		}
		if retryNeeded {
			w.logger.Info(
				"daily question batch completed with recoverable preparations",
				"assignment_date", dateString(runDate),
				"retry_delay", dailyWorkerRetryDelay,
			)
			nextRun = nextShanghaiWorkerRetry(now)
			continue
		}
		w.logger.Info("daily question batch completed", "assignment_date", dateString(runDate))
		nextRun = nextShanghaiMidnight(now)
	}
}

func (w *Worker) prepareDate(ctx context.Context, date time.Time) (bool, error) {
	dayCtx, cancel := context.WithDeadline(ctx, date.AddDate(0, 0, 1))
	defer cancel()

	afterID := ""
	retryNeeded := false
	for dayCtx.Err() == nil && shanghaiDay(w.now()).Equal(date) {
		studentIDs, err := w.listActiveStudentPage(dayCtx, afterID)
		if err != nil {
			if dayCtx.Err() != nil {
				return retryNeeded, nil
			}
			return retryNeeded, fmt.Errorf("list active students for daily question: %w", err)
		}
		if len(studentIDs) == 0 {
			return retryNeeded, nil
		}

		batch := w.prepareBatch(dayCtx, date, studentIDs)
		retryNeeded = retryNeeded || batch.retryNeeded
		for errorCode, count := range batch.failures {
			w.logger.Warn(
				"daily question batch student preparations failed",
				"assignment_date", dateString(date),
				"error_code", errorCode,
				"failure_count", count,
			)
		}
		if dayCtx.Err() != nil || !shanghaiDay(w.now()).Equal(date) {
			return retryNeeded, nil
		}
		afterID = studentIDs[len(studentIDs)-1]
		if len(studentIDs) < w.config.BatchSize {
			return retryNeeded, nil
		}
		if err := waitForWorkerTimer(dayCtx, w.config.BatchInterval); err != nil {
			return retryNeeded, nil
		}
	}
	return retryNeeded, nil
}

func (w *Worker) listActiveStudentPage(ctx context.Context, afterID string) ([]string, error) {
	var lastErr error
	for attempt := 1; attempt <= maxWorkerPageAttempts; attempt++ {
		studentIDs, err := w.repository.ListActiveStudentIDs(ctx, afterID, w.config.BatchSize)
		if err == nil {
			return studentIDs, nil
		}
		lastErr = err
		if ctx.Err() != nil || attempt == maxWorkerPageAttempts {
			break
		}
		if err := waitForWorkerTimer(ctx, w.config.BatchInterval); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

type workerBatchResult struct {
	failures    map[string]int
	retryNeeded bool
}

type workerStudentResult struct {
	failureCode string
	retryNeeded bool
}

func (w *Worker) prepareBatch(ctx context.Context, date time.Time, studentIDs []string) workerBatchResult {
	slots := make(chan struct{}, w.config.Concurrency)
	results := make(chan workerStudentResult, len(studentIDs))
	var waitGroup sync.WaitGroup

dispatch:
	for _, studentID := range studentIDs {
		if ctx.Err() != nil || !shanghaiDay(w.now()).Equal(date) {
			break
		}
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			break dispatch
		}
		waitGroup.Add(1)
		go func(studentID string) {
			defer waitGroup.Done()
			defer func() { <-slots }()
			if ctx.Err() != nil || !shanghaiDay(w.now()).Equal(date) {
				return
			}
			studentCtx, cancel := context.WithTimeout(ctx, w.config.StudentTimeout)
			response, err := w.preparer.PrepareTodayInBackground(studentCtx, studentID)
			cancel()
			if ctx.Err() == nil {
				result := classifyWorkerPreparation(response, err)
				if result.failureCode != "" || result.retryNeeded {
					results <- result
				}
			}
		}(studentID)
	}
	waitGroup.Wait()
	close(results)
	batch := workerBatchResult{failures: make(map[string]int)}
	for result := range results {
		if result.failureCode != "" {
			batch.failures[result.failureCode]++
		}
		batch.retryNeeded = batch.retryNeeded || result.retryNeeded
	}
	return batch
}

func classifyWorkerPreparation(response TodayResponse, err error) workerStudentResult {
	failureCode := ""
	if response.FailureCode != nil {
		failureCode = *response.FailureCode
	}

	switch response.Status {
	case StatusPreparing:
		if failureCode == "" {
			failureCode = StatusPreparing
		}
		return workerStudentResult{
			failureCode: failureCode,
			retryNeeded: true,
		}
	case StatusUnavailable:
		if failureCode == "" {
			failureCode = StatusUnavailable
		}
		return workerStudentResult{
			failureCode: failureCode,
			retryNeeded: response.RetryCount < maxBackgroundPreparationRetries &&
				workerFailureIsRecoverable(failureCode, err),
		}
	}

	if err == nil {
		return workerStudentResult{}
	}
	return workerStudentResult{
		failureCode: workerPreparationErrorCode(err),
		retryNeeded: workerPreparationErrorIsRecoverable(err),
	}
}

func workerFailureIsRecoverable(failureCode string, err error) bool {
	switch failureCode {
	case "ai_generation_unavailable",
		"ai_generation_timeout",
		"solver_unavailable",
		"ai_generation_invalid_response",
		"ai_generation_invalid_content",
		"ai_generation_rate_limited",
		"history_lookup_failed",
		"recent_attempt_lookup_failed",
		"content_selection_failed",
		"repeat_fallback_failed",
		"assignment_finalize_failed",
		"question_repeated_during_assignment",
		"content_unavailable",
		FailureTeacherNotAssigned:
		return true
	case "ai_access_unavailable":
		return errors.Is(err, airiskapp.ErrUnavailable) || errors.Is(err, airiskapp.ErrConcurrencyExceeded)
	default:
		return false
	}
}

func workerPreparationErrorIsRecoverable(err error) bool {
	return !errors.Is(err, ErrNotFound) &&
		!errors.Is(err, ErrForbidden) &&
		!errors.Is(err, airiskapp.ErrAccessBlocked) &&
		!errors.Is(err, airiskapp.ErrContentBlocked) &&
		!errors.Is(err, airiskapp.ErrQuotaExceeded)
}

func workerPreparationErrorCode(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "student_timeout"
	case errors.Is(err, ErrRateLimited):
		return "rate_limited"
	default:
		return "preparation_error"
	}
}

func nextShanghaiMidnight(value time.Time) time.Time {
	return shanghaiDay(value).AddDate(0, 0, 1)
}

func nextShanghaiWorkerRetry(value time.Time) time.Time {
	retryAt := value.Add(dailyWorkerRetryDelay)
	if shanghaiDay(retryAt).Equal(shanghaiDay(value)) {
		return retryAt
	}
	return shanghaiDay(retryAt)
}

func waitForWorkerTimer(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
