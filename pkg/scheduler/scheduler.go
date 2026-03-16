package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/glennprays/dbeasebackup/config"
	"github.com/glennprays/log"
	"github.com/robfig/cron/v3"
)

// Job defines the interface for schedulable jobs
type Job interface {
	// Name returns the name of the job
	Name() string
	// Execute runs the job
	Execute(ctx context.Context) error
}

// Scheduler defines the interface for job scheduling
type Scheduler interface {
	// AddJob adds a job with the given cron expression
	AddJob(cronExpr string, job Job) error
	// Start starts the scheduler
	Start() error
	// Stop stops the scheduler
	Stop(ctx context.Context) error
	// IsRunning returns whether the scheduler is running
	IsRunning() bool
}

// CronScheduler implements the Scheduler interface using robfig/cron
type CronScheduler struct {
	cron    *cron.Cron
	cfg     *config.Config
	logger  *log.Logger
	running bool
}

// NewCronScheduler creates a new cron scheduler
func NewCronScheduler(cfg *config.Config, logger *log.Logger) (*CronScheduler, error) {
	traceID := "scheduler-init"

	loc, err := time.LoadLocation(cfg.SCHEDULER_TIMEZONE)
	if err != nil {
		logger.Error(traceID, "Failed to load timezone", nil,
			log.Error(err),
			log.String("timezone", cfg.SCHEDULER_TIMEZONE),
		)
		return nil, fmt.Errorf("invalid timezone %s: %w", cfg.SCHEDULER_TIMEZONE, err)
	}

	logger.Info(traceID, "Scheduler initialized", nil,
		log.String("timezone", cfg.SCHEDULER_TIMEZONE),
	)

	return &CronScheduler{
		cron:   cron.New(cron.WithLocation(loc)),
		cfg:    cfg,
		logger: logger,
	}, nil
}

// AddJob adds a job with the given cron expression
func (s *CronScheduler) AddJob(cronExpr string, job Job) error {
	traceID := "scheduler-add-job"

	entryID, err := s.cron.AddFunc(cronExpr, func() {
		jobTraceID := fmt.Sprintf("job-%s-%d", job.Name(), time.Now().Unix())

		s.logger.Info(jobTraceID, "Starting scheduled job", nil,
			log.String("job", job.Name()),
		)

		ctx := context.Background()
		if err := job.Execute(ctx); err != nil {
			s.logger.Error(jobTraceID, "Job execution failed", nil,
				log.Error(err),
				log.String("job", job.Name()),
			)
			return
		}

		s.logger.Info(jobTraceID, "Job completed successfully", nil,
			log.String("job", job.Name()),
		)
	})

	if err != nil {
		s.logger.Error(traceID, "Failed to add job to scheduler", nil,
			log.Error(err),
			log.String("job", job.Name()),
			log.String("cronExpr", cronExpr),
		)
		return fmt.Errorf("failed to add job %s: %w", job.Name(), err)
	}

	s.logger.Info(traceID, "Job added to scheduler", nil,
		log.String("job", job.Name()),
		log.String("cronExpr", cronExpr),
		log.Int("entryID", int(entryID)),
	)

	return nil
}

// Start starts the scheduler
func (s *CronScheduler) Start() error {
	traceID := "scheduler-start"

	if s.running {
		s.logger.Warn(traceID, "Scheduler is already running", nil)
		return nil
	}

	s.cron.Start()
	s.running = true

	s.logger.Info(traceID, "Scheduler started", nil)
	return nil
}

// Stop stops the scheduler
func (s *CronScheduler) Stop(ctx context.Context) error {
	traceID := "scheduler-stop"

	if !s.running {
		s.logger.Warn(traceID, "Scheduler is not running", nil)
		return nil
	}

	ctx = s.cron.Stop()
	<-ctx.Done()

	s.running = false

	s.logger.Info(traceID, "Scheduler stopped", nil)
	return nil
}

// IsRunning returns whether the scheduler is running
func (s *CronScheduler) IsRunning() bool {
	return s.running
}
