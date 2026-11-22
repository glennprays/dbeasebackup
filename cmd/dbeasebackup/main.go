package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/glennprays/dbeasebackup/config"
	"github.com/glennprays/dbeasebackup/internal/backup"
	"github.com/glennprays/dbeasebackup/internal/health"
	backupprovider "github.com/glennprays/dbeasebackup/pkg/backup"
	"github.com/glennprays/dbeasebackup/pkg/notifier"
	"github.com/glennprays/dbeasebackup/pkg/scheduler"
	"github.com/glennprays/dbeasebackup/pkg/storage"
	"github.com/glennprays/log"
)

// defaultCleanupCron is the fallback cleanup schedule when CLEANUP_CRON_SCHEDULE
// is unset and targets did not come from the legacy env fallback.
const defaultCleanupCron = "0 3 * * *"

func main() {
	// Load configuration using viper
	cfg, err := config.ProvideConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Resolve targets: TARGETS_FILE, else legacy env fallback.
	targets, fromLegacy, err := config.LoadTargetsOrDefault(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to resolve backup targets: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger, err := initializeLogger(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	traceID := "main-init"
	logger.Info(traceID, "Starting DBEaseBackup", nil,
		log.String("env", cfg.ENV),
		log.String("storage_type", cfg.STORAGE_TYPE),
		log.Int("targets", len(targets)),
		log.Int("max_parallel_backups", cfg.MAX_PARALLEL_BACKUPS),
	)

	// Initialize storage (once; credentials are global)
	storageProvider, err := initializeStorage(context.Background(), cfg, logger)
	if err != nil {
		logger.Error(traceID, "Failed to initialize storage", nil, log.Error(err))
		os.Exit(1)
	}
	logger.Info(traceID, "Storage initialized", nil)

	// Initialize webhook notifier (disabled when WEBHOOK_URL is empty)
	var webhookNotifier notifier.Notifier
	if wn := notifier.NewWebhookNotifier(notifier.WebhookConfig{
		URL:     cfg.WEBHOOK_URL,
		Secret:  cfg.WEBHOOK_SECRET,
		Timeout: cfg.GetWebhookTimeout(),
	}, logger); wn != nil {
		webhookNotifier = wn
		logger.Info(traceID, "Webhook notifier initialized", nil,
			log.String("url", cfg.WEBHOOK_URL),
		)
	}

	// Per-target services and runners, backed by a shared concurrency pool.
	sem := make(chan struct{}, max(1, cfg.MAX_PARALLEL_BACKUPS))
	services := make([]*backup.Service, 0, len(targets))
	runners := make([]*backup.Runner, 0, len(targets))

	for i := range targets {
		target := &targets[i]

		provider, err := backupprovider.NewProviderFor(target, logger)
		if err != nil {
			logger.Error(traceID, "Failed to create backup provider", nil,
				log.Error(err),
				log.String("target", target.Name),
			)
			os.Exit(1)
		}

		if err := provider.ValidateDependencies(); err != nil {
			logger.Error(traceID, "Backup provider dependency validation failed", nil,
				log.Error(err),
				log.String("target", target.Name),
				log.String("mode", target.Mode),
			)
			os.Exit(1)
		}

		svc, err := backup.NewService(storageProvider, provider, target, logger, webhookNotifier)
		if err != nil {
			logger.Error(traceID, "Failed to initialize backup service", nil,
				log.Error(err),
				log.String("target", target.Name),
			)
			os.Exit(1)
		}

		runners = append(runners, backup.NewRunner(svc, target, sem, logger, webhookNotifier))
		services = append(services, svc)

		logger.Info(traceID, "Target configured", nil,
			log.String("target", target.Name),
			log.String("mode", target.Mode),
			log.String("schedule", target.Schedule),
			log.String("backup_dir", target.BackupDir),
		)
	}

	// Initialize health check server
	healthServer := health.NewServer(services, logger)
	if err := healthServer.Start(cfg.HTTP_PORT); err != nil {
		logger.Error(traceID, "Failed to start health server", nil, log.Error(err))
		os.Exit(1)
	}
	logger.Info(traceID, "Health check server started", nil,
		log.Int("port", cfg.HTTP_PORT),
	)

	// Initialize scheduler
	cronScheduler, err := scheduler.NewCronScheduler(cfg, logger)
	if err != nil {
		logger.Error(traceID, "Failed to initialize scheduler", nil, log.Error(err))
		os.Exit(1)
	}

	// Per-target backup + cleanup jobs.
	for i := range runners {
		target := &targets[i]
		if err := cronScheduler.AddJob(target.Schedule, runners[i]); err != nil {
			logger.Error(traceID, "Failed to add backup job", nil,
				log.Error(err),
				log.String("target", target.Name),
			)
			os.Exit(1)
		}

		cleanupJob := backup.NewCleanupJob(services[i])
		cleanupSchedule := resolveCleanupSchedule(cfg, fromLegacy)
		if err := cronScheduler.AddJob(cleanupSchedule, cleanupJob); err != nil {
			logger.Error(traceID, "Failed to add cleanup job", nil,
				log.Error(err),
				log.String("target", target.Name),
			)
			os.Exit(1)
		}

		logger.Info(traceID, "Jobs scheduled", nil,
			log.String("target", target.Name),
			log.String("backup_schedule", target.Schedule),
			log.String("cleanup_schedule", cleanupSchedule),
		)
	}

	logger.Info(traceID, "Scheduler configured", nil,
		log.String("timezone", cfg.SCHEDULER_TIMEZONE),
	)

	// Start the scheduler
	if err := cronScheduler.Start(); err != nil {
		logger.Error(traceID, "Failed to start scheduler", nil, log.Error(err))
		os.Exit(1)
	}

	logger.Info(traceID, "DBEaseBackup started successfully", nil)

	// Wait for shutdown signal
	waitForShutdown(logger, cronScheduler, healthServer)
}

// resolveCleanupSchedule resolves the cleanup cron: CLEANUP_CRON_SCHEDULE if
// set; else the legacy CRON_SCHEDULE when targets came from legacy env; else
// the daily 03:00 default.
func resolveCleanupSchedule(cfg *config.Config, fromLegacy bool) string {
	if cfg.CLEANUP_CRON_SCHEDULE != "" {
		return cfg.CLEANUP_CRON_SCHEDULE
	}
	if fromLegacy && cfg.CRON_SCHEDULE != "" {
		return cfg.CRON_SCHEDULE
	}
	return defaultCleanupCron
}

// initializeStorage creates the appropriate storage provider based on configuration
func initializeStorage(ctx context.Context, cfg *config.Config, logger *log.Logger) (storage.Storage, error) {
	switch cfg.STORAGE_TYPE {
	case "google-drive":
		return storage.NewGoogleDriveStorage(
			ctx,
			cfg.GOOGLE_DRIVE_KEY_FILE,
			logger,
		)
	case "s3":
		s3Config := storage.S3Config{
			Bucket:          cfg.S3_BUCKET,
			Region:          cfg.S3_REGION,
			AccessKeyID:     cfg.S3_ACCESS_KEY_ID,
			SecretAccessKey: cfg.S3_SECRET_ACCESS_KEY,
			Endpoint:        cfg.S3_ENDPOINT,
			Prefix:          cfg.S3_PREFIX,
		}
		return storage.NewS3Storage(ctx, s3Config, logger)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.STORAGE_TYPE)
	}
}

// initializeLogger creates and configures the logger
func initializeLogger(cfg *config.Config) (*log.Logger, error) {
	logLevel := parseLogLevel(cfg.LOG_LEVEL)

	logConfig := log.Config{
		Service: "dbeasebackup",
		Env:     cfg.ENV,
		Level:   logLevel,
		Output:  log.OutputStdout,
	}

	return log.New(logConfig)
}

// parseLogLevel converts a string log level to log.Level
func parseLogLevel(level string) log.Level {
	switch level {
	case "debug":
		return log.DebugLevel
	case "info":
		return log.InfoLevel
	case "warn":
		return log.WarnLevel
	case "error":
		return log.ErrorLevel
	default:
		return log.InfoLevel
	}
}

// waitForShutdown blocks until a shutdown signal is received
func waitForShutdown(logger *log.Logger, sched *scheduler.CronScheduler, healthServer *health.Server) {
	traceID := "shutdown"

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	logger.Info(traceID, "Shutdown signal received", nil,
		log.String("signal", sig.String()),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := healthServer.Stop(ctx); err != nil {
		logger.Error(traceID, "Error stopping health server", nil, log.Error(err))
	}

	if err := sched.Stop(ctx); err != nil {
		logger.Error(traceID, "Error stopping scheduler", nil, log.Error(err))
	}

	logger.Info(traceID, "DBEaseBackup stopped", nil)
}
