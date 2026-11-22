package backup

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/glennprays/dbeasebackup/config"
	"github.com/glennprays/dbeasebackup/pkg/notifier"
	"github.com/glennprays/dbeasebackup/pkg/traceid"
	"github.com/glennprays/log"
)

const (
	// retryBackoffBase is the first backoff; it grows by 4x per attempt.
	retryBackoffBase = 30 * time.Second
	// retryBackoffMax caps the backoff between attempts.
	retryBackoffMax = 2 * time.Minute
)

// Runner wraps one target's Service with the execution policy that the
// scheduler triggers: global concurrency pool, per-target overlap
// protection, retries with backoff, and exactly one notification per run.
type Runner struct {
	service  *Service
	target   *config.Target
	sem      chan struct{}
	mu       *sync.Mutex // per-target overlap lock
	notifier notifier.Notifier
	logger   *log.Logger

	// backoffBase is the first retry backoff; tests shrink it.
	backoffBase time.Duration
}

// NewRunner creates a runner for one target. sem is the shared global pool
// (capacity MAX_PARALLEL_BACKUPS); mu must be unique per target.
func NewRunner(service *Service, target *config.Target, sem chan struct{}, logger *log.Logger, n notifier.Notifier) *Runner {
	return &Runner{
		service:     service,
		target:      target,
		sem:         sem,
		mu:          &sync.Mutex{},
		notifier:    n,
		logger:      logger,
		backoffBase: retryBackoffBase,
	}
}

// Name returns the job name (implements scheduler.Job interface).
func (r *Runner) Name() string {
	return "backup:" + r.target.Name
}

// Execute runs the target's backup with pool, overlap, retry, and
// notify-once semantics.
func (r *Runner) Execute(ctx context.Context) error {
	traceID := traceid.FromContext(ctx)
	startTime := time.Now()

	// 1. Per-target overlap protection: skip, don't queue.
	if !r.mu.TryLock() {
		r.logger.Warn(traceID, "Previous run still in progress, skipping", nil,
			log.String("target", r.target.Name),
			log.String("component", "runner"),
		)
		return nil
	}
	defer r.mu.Unlock()

	// 2. Global pool: block until a slot frees (runs late rather than
	// skipping); abort if the context is cancelled while waiting.
	select {
	case r.sem <- struct{}{}:
		defer func() { <-r.sem }()
	case <-ctx.Done():
		return ctx.Err()
	}

	// 3. Attempt loop: 1 + retries attempts with 4x-growing backoff.
	var finalErr error
	attempts := 1 + r.target.RetryCount()
loop:
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			backoff := r.backoffBase * time.Duration(1<<(2*(attempt-1))) // base, 4x, 16x...
			if backoff > retryBackoffMax {
				backoff = retryBackoffMax
			}
			r.logger.Info(traceID, "Retrying backup after backoff", nil,
				log.String("target", r.target.Name),
				log.Int("attempt", attempt+1),
				log.String("backoff", backoff.String()),
				log.String("component", "runner"),
			)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				finalErr = fmt.Errorf("backup cancelled during backoff: %w", ctx.Err())
				break loop
			}
		}

		if err := r.service.Backup(ctx); err != nil {
			finalErr = err
			r.logger.Error(traceID, "Backup attempt failed", nil,
				log.Error(err),
				log.String("target", r.target.Name),
				log.Int("attempt", attempt+1),
				log.Int("max_attempts", attempts),
				log.String("component", "runner"),
			)
			continue
		}
		finalErr = nil
		break
	}

	// 4. Notify exactly once per final outcome.
	duration := time.Since(startTime).String()
	event := notifier.Event{
		EventType: notifier.EventBackup,
		Target:    r.target.Name,
		Duration:  duration,
		Timestamp: time.Now(),
		TraceID:   traceID,
	}
	if finalErr != nil {
		event.Status = notifier.StatusFailure
		event.Error = finalErr.Error()
	} else {
		event.Status = notifier.StatusSuccess
	}
	if r.notifier != nil {
		r.notifier.Notify(ctx, event)
	}

	return finalErr
}

// Ensure the runner implements scheduler.Job.
var _ interface {
	Name() string
	Execute(ctx context.Context) error
} = (*Runner)(nil)
