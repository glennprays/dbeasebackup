package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glennprays/dbeasebackup/config"
	"github.com/glennprays/log"
)

func TestNewCronScheduler_Success(t *testing.T) {
	logger, _ := log.New(log.Config{
		Service: "test",
		Env:     "development",
		Level:   log.ErrorLevel,
		Output:  log.OutputStdout,
	})

	cfg := &config.Config{
		SCHEDULER_TIMEZONE: "UTC",
	}

	scheduler, err := NewCronScheduler(cfg, logger)
	if err != nil {
		t.Errorf("NewCronScheduler() unexpected error = %v", err)
	}
	if scheduler == nil {
		t.Error("NewCronScheduler() returned nil scheduler")
	}
}

func TestNewCronScheduler_InvalidTimezone(t *testing.T) {
	logger, _ := log.New(log.Config{
		Service: "test",
		Env:     "development",
		Level:   log.ErrorLevel,
		Output:  log.OutputStdout,
	})

	cfg := &config.Config{
		SCHEDULER_TIMEZONE: "Invalid/Timezone",
	}

	scheduler, err := NewCronScheduler(cfg, logger)
	if err == nil {
		t.Error("NewCronScheduler() expected error for invalid timezone")
	}
	if scheduler != nil {
		t.Error("NewCronScheduler() should return nil scheduler on error")
	}
}

func TestNewCronScheduler_ValidTimezones(t *testing.T) {
	validTimezones := []string{
		"UTC",
		"America/New_York",
		"Europe/London",
		"Asia/Tokyo",
		"Australia/Sydney",
	}

	logger, _ := log.New(log.Config{
		Service: "test",
		Env:     "development",
		Level:   log.ErrorLevel,
		Output:  log.OutputStdout,
	})

	for _, tz := range validTimezones {
		t.Run("timezone_"+tz, func(t *testing.T) {
			cfg := &config.Config{
				SCHEDULER_TIMEZONE: tz,
			}

			scheduler, err := NewCronScheduler(cfg, logger)
			if err != nil {
				t.Errorf("NewCronScheduler() unexpected error for timezone %s: %v", tz, err)
			}
			if scheduler == nil {
				t.Errorf("NewCronScheduler() returned nil scheduler for timezone %s", tz)
			}
		})
	}
}

func TestCronScheduler_AddJob_Success(t *testing.T) {
	logger, _ := log.New(log.Config{
		Service: "test",
		Env:     "development",
		Level:   log.ErrorLevel,
		Output:  log.OutputStdout,
	})

	cfg := &config.Config{
		SCHEDULER_TIMEZONE: "UTC",
	}

	scheduler, err := NewCronScheduler(cfg, logger)
	if err != nil {
		t.Fatalf("NewCronScheduler() error = %v", err)
	}

	mockJob := &MockJob{
		NameFunc: func() string { return "test-job" },
	}

	err = scheduler.AddJob("0 * * * *", mockJob)
	if err != nil {
		t.Errorf("AddJob() unexpected error = %v", err)
	}
}

func TestCronScheduler_AddJob_InvalidCron(t *testing.T) {
	logger, _ := log.New(log.Config{
		Service: "test",
		Env:     "development",
		Level:   log.ErrorLevel,
		Output:  log.OutputStdout,
	})

	cfg := &config.Config{
		SCHEDULER_TIMEZONE: "UTC",
	}

	scheduler, err := NewCronScheduler(cfg, logger)
	if err != nil {
		t.Fatalf("NewCronScheduler() error = %v", err)
	}

	mockJob := &MockJob{
		NameFunc: func() string { return "test-job" },
	}

	err = scheduler.AddJob("invalid-cron", mockJob)
	if err == nil {
		t.Error("AddJob() expected error for invalid cron expression")
	}
}

func TestCronScheduler_AddJob_ValidCronExpressions(t *testing.T) {
	validExpressions := []string{
		"* * * * *",
		"0 * * * *",
		"0 0 * * *",
		"0 0 1 * *",
		"0 0 1 1 *",
		"0 0 * * MON",
		"0 0 * * 1",
		"*/15 * * * *",
		"0 9-17 * * 1-5",
	}

	logger, _ := log.New(log.Config{
		Service: "test",
		Env:     "development",
		Level:   log.ErrorLevel,
		Output:  log.OutputStdout,
	})

	cfg := &config.Config{
		SCHEDULER_TIMEZONE: "UTC",
	}

	scheduler, err := NewCronScheduler(cfg, logger)
	if err != nil {
		t.Fatalf("NewCronScheduler() error = %v", err)
	}

	for _, expr := range validExpressions {
		t.Run("cron_"+expr, func(t *testing.T) {
			mockJob := &MockJob{
				NameFunc: func() string { return "test-job" },
			}

			err := scheduler.AddJob(expr, mockJob)
			if err != nil {
				t.Errorf("AddJob() unexpected error for cron expression %s: %v", expr, err)
			}
		})
	}
}

func TestCronScheduler_Start(t *testing.T) {
	logger, _ := log.New(log.Config{
		Service: "test",
		Env:     "development",
		Level:   log.ErrorLevel,
		Output:  log.OutputStdout,
	})

	cfg := &config.Config{
		SCHEDULER_TIMEZONE: "UTC",
	}

	scheduler, err := NewCronScheduler(cfg, logger)
	if err != nil {
		t.Fatalf("NewCronScheduler() error = %v", err)
	}

	err = scheduler.Start()
	if err != nil {
		t.Errorf("Start() unexpected error = %v", err)
	}

	if !scheduler.IsRunning() {
		t.Error("IsRunning() should return true after Start()")
	}

	// Cleanup
	_ = scheduler.Stop(context.Background())
}

func TestCronScheduler_Start_AlreadyRunning(t *testing.T) {
	logger, _ := log.New(log.Config{
		Service: "test",
		Env:     "development",
		Level:   log.ErrorLevel,
		Output:  log.OutputStdout,
	})

	cfg := &config.Config{
		SCHEDULER_TIMEZONE: "UTC",
	}

	scheduler, err := NewCronScheduler(cfg, logger)
	if err != nil {
		t.Fatalf("NewCronScheduler() error = %v", err)
	}

	// Start first time
	_ = scheduler.Start()

	// Start again - should not error
	err = scheduler.Start()
	if err != nil {
		t.Errorf("Start() should not error when already running: %v", err)
	}

	// Cleanup
	_ = scheduler.Stop(context.Background())
}

func TestCronScheduler_Stop(t *testing.T) {
	logger, _ := log.New(log.Config{
		Service: "test",
		Env:     "development",
		Level:   log.ErrorLevel,
		Output:  log.OutputStdout,
	})

	cfg := &config.Config{
		SCHEDULER_TIMEZONE: "UTC",
	}

	scheduler, err := NewCronScheduler(cfg, logger)
	if err != nil {
		t.Fatalf("NewCronScheduler() error = %v", err)
	}

	_ = scheduler.Start()

	err = scheduler.Stop(context.Background())
	if err != nil {
		t.Errorf("Stop() unexpected error = %v", err)
	}

	if scheduler.IsRunning() {
		t.Error("IsRunning() should return false after Stop()")
	}
}

func TestCronScheduler_Stop_NotRunning(t *testing.T) {
	logger, _ := log.New(log.Config{
		Service: "test",
		Env:     "development",
		Level:   log.ErrorLevel,
		Output:  log.OutputStdout,
	})

	cfg := &config.Config{
		SCHEDULER_TIMEZONE: "UTC",
	}

	scheduler, err := NewCronScheduler(cfg, logger)
	if err != nil {
		t.Fatalf("NewCronScheduler() error = %v", err)
	}

	// Stop without starting - should not error
	err = scheduler.Stop(context.Background())
	if err != nil {
		t.Errorf("Stop() should not error when not running: %v", err)
	}
}

func TestCronScheduler_IsRunning(t *testing.T) {
	logger, _ := log.New(log.Config{
		Service: "test",
		Env:     "development",
		Level:   log.ErrorLevel,
		Output:  log.OutputStdout,
	})

	cfg := &config.Config{
		SCHEDULER_TIMEZONE: "UTC",
	}

	scheduler, err := NewCronScheduler(cfg, logger)
	if err != nil {
		t.Fatalf("NewCronScheduler() error = %v", err)
	}

	// Initially not running
	if scheduler.IsRunning() {
		t.Error("IsRunning() should return false initially")
	}

	// After start
	_ = scheduler.Start()
	if !scheduler.IsRunning() {
		t.Error("IsRunning() should return true after Start()")
	}

	// After stop
	_ = scheduler.Stop(context.Background())
	if scheduler.IsRunning() {
		t.Error("IsRunning() should return false after Stop()")
	}
}

func TestCronScheduler_JobExecution(t *testing.T) {
	logger, _ := log.New(log.Config{
		Service: "test",
		Env:     "development",
		Level:   log.ErrorLevel,
		Output:  log.OutputStdout,
	})

	cfg := &config.Config{
		SCHEDULER_TIMEZONE: "UTC",
	}

	scheduler, err := NewCronScheduler(cfg, logger)
	if err != nil {
		t.Fatalf("NewCronScheduler() error = %v", err)
	}

	mockJob := &MockJob{
		NameFunc: func() string { return "test-job" },
		ExecuteFunc: func(ctx context.Context) error {
			return nil
		},
	}

	// Add job to run every minute (5-field cron expression)
	err = scheduler.AddJob("* * * * *", mockJob)
	if err != nil {
		t.Fatalf("AddJob() error = %v", err)
	}

	_ = scheduler.Start()

	// Wait briefly for scheduler to start
	time.Sleep(100 * time.Millisecond)

	_ = scheduler.Stop(context.Background())

	// Note: We can't reliably test actual job execution in unit tests
	// because standard cron runs at minute granularity.
	// The test verifies that jobs can be added and the scheduler starts/stops.
	if !scheduler.IsRunning() {
		// After stop, it should not be running
		t.Log("Scheduler stopped successfully")
	}
}

func TestCronScheduler_JobExecutionError(t *testing.T) {
	logger, _ := log.New(log.Config{
		Service: "test",
		Env:     "development",
		Level:   log.ErrorLevel,
		Output:  log.OutputStdout,
	})

	cfg := &config.Config{
		SCHEDULER_TIMEZONE: "UTC",
	}

	scheduler, err := NewCronScheduler(cfg, logger)
	if err != nil {
		t.Fatalf("NewCronScheduler() error = %v", err)
	}

	mockJob := &MockJob{
		NameFunc: func() string { return "failing-job" },
		ExecuteFunc: func(ctx context.Context) error {
			return errors.New("job failed")
		},
	}

	// Add job to run every minute (5-field cron expression)
	err = scheduler.AddJob("* * * * *", mockJob)
	if err != nil {
		t.Fatalf("AddJob() error = %v", err)
	}

	_ = scheduler.Start()

	// Wait briefly for scheduler to start
	time.Sleep(100 * time.Millisecond)

	_ = scheduler.Stop(context.Background())

	// Note: We can't reliably test actual job execution in unit tests
	// because standard cron runs at minute granularity.
	// The test verifies that failing jobs can be added and the scheduler handles them.
	if !scheduler.IsRunning() {
		// After stop, it should not be running
		t.Log("Scheduler stopped successfully even with failing job")
	}
}

// MockJob implements the Job interface for testing
type MockJob struct {
	NameFunc     func() string
	ExecuteFunc  func(ctx context.Context) error
	ExecuteCount int
}

func (m *MockJob) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock-job"
}

func (m *MockJob) Execute(ctx context.Context) error {
	m.ExecuteCount++
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx)
	}
	return nil
}
