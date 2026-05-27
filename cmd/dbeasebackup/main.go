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
	"github.com/glennprays/dbeasebackup/pkg/database"
	"github.com/glennprays/dbeasebackup/pkg/scheduler"
	"github.com/glennprays/dbeasebackup/pkg/storage"
	"github.com/glennprays/log"
)

func main() {
	// Load configuration using viper
	cfg, err := config.ProvideConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
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
		log.String("backup_provider", cfg.BACKUP_PROVIDER),
		log.String("storage_type", cfg.STORAGE_TYPE),
	)

	// Initialize database
	db := database.NewPostgresDatabase(cfg, logger)
	if err := db.Connect(context.Background()); err != nil {
		logger.Error(traceID, "Failed to connect to database", nil, log.Error(err))
		os.Exit(1)
	}
	defer db.Close()

	logger.Info(traceID, "Database connected", nil)

	// Validate database connection
	if err := db.Ping(context.Background()); err != nil {
		logger.Error(traceID, "Database connection validation failed", nil,
			log.Error(err),
			log.String("host", cfg.PG_HOST),
			log.String("port", cfg.PG_PORT),
			log.String("database", cfg.PG_DATABASE),
			log.String("user", cfg.PG_USER),
		)
		os.Exit(1)
	}

	logger.Info(traceID, "Database connection validated", nil,
		log.String("host", cfg.PG_HOST),
		log.String("port", cfg.PG_PORT),
		log.String("database", cfg.PG_DATABASE),
	)

	// Initialize backup provider
	backupProvider, err := initializeBackupProvider(cfg, logger)
	if err != nil {
		logger.Error(traceID, "Failed to initialize backup provider", nil, log.Error(err))
		os.Exit(1)
	}

	logger.Info(traceID, "Backup provider initialized", nil,
		log.String("provider", backupProvider.Name()),
	)

	// Validate backup provider dependencies
	if err := backupProvider.ValidateDependencies(); err != nil {
		logger.Error(traceID, "Backup provider dependency validation failed", nil, log.Error(err))
		os.Exit(1)
	}

	logger.Info(traceID, "Backup provider dependencies validated", nil,
		log.String("provider", backupProvider.Name()),
	)

	// Initialize storage
	storageProvider, err := initializeStorage(context.Background(), cfg, logger)
	if err != nil {
		logger.Error(traceID, "Failed to initialize storage", nil, log.Error(err))
		os.Exit(1)
	}

	logger.Info(traceID, "Storage initialized", nil)

	// Initialize backup service
	backupService, err := backup.NewService(db, storageProvider, backupProvider, cfg, logger)
	if err != nil {
		logger.Error(traceID, "Failed to initialize backup service", nil, log.Error(err))
		os.Exit(1)
	}

	logger.Info(traceID, "Backup service initialized", nil)

	// Initialize health check server
	healthServer := health.NewServer(backupService, logger)
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

	// Add backup job to scheduler
	if err := cronScheduler.AddJob(cfg.CRON_SCHEDULE, backupService); err != nil {
		logger.Error(traceID, "Failed to add backup job", nil, log.Error(err))
		os.Exit(1)
	}

	// Add cleanup job
	cleanupJob := backup.NewCleanupJob(backupService)
	cleanupSchedule := cfg.CLEANUP_CRON_SCHEDULE
	if cleanupSchedule == "" {
		cleanupSchedule = cfg.CRON_SCHEDULE
	}
	if err := cronScheduler.AddJob(cleanupSchedule, cleanupJob); err != nil {
		logger.Error(traceID, "Failed to add cleanup job", nil, log.Error(err))
		os.Exit(1)
	}

	logger.Info(traceID, "Scheduler configured", nil,
		log.String("backup_schedule", cfg.CRON_SCHEDULE),
		log.String("cleanup_schedule", cleanupSchedule),
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

// initializeBackupProvider creates the appropriate backup provider based on configuration
func initializeBackupProvider(cfg *config.Config, logger *log.Logger) (backupprovider.Provider, error) {
	switch cfg.BACKUP_PROVIDER {
	case "postgres":
		pgConfig := backupprovider.PostgresConfig{
			Host:     cfg.PG_HOST,
			Port:     cfg.PG_PORT,
			User:     cfg.PG_USER,
			Password: cfg.PG_PASSWORD,
			Database: cfg.PG_DATABASE,
		}
		return backupprovider.NewPostgresProvider(pgConfig, logger, cfg.GetBackupTimeout()), nil
	default:
		return nil, fmt.Errorf("unsupported backup provider: %s", cfg.BACKUP_PROVIDER)
	}
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
