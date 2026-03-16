package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/glennprays/dbeasebackup/config"
	"github.com/glennprays/dbeasebackup/internal/backup"
	"github.com/glennprays/dbeasebackup/pkg/database"
	"github.com/glennprays/dbeasebackup/pkg/scheduler"
	"github.com/glennprays/dbeasebackup/pkg/storage"
	"github.com/glennprays/log"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if not in production
	if os.Getenv("GO_ENV") != "production" {
		if err := godotenv.Load(); err != nil {
			fmt.Println("Warning: .env file not found")
		}
	}

	// Load configuration
	cfg, err := config.Load()
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
		log.String("env", cfg.GoEnv),
	)

	// Initialize database
	db := database.NewPostgresDatabase(&cfg.Database, logger)
	if err := db.Connect(context.Background()); err != nil {
		logger.Error(traceID, "Failed to connect to database", nil, log.Error(err))
		os.Exit(1)
	}
	defer db.Close()

	logger.Info(traceID, "Database connected", nil)

	// Initialize storage
	googleDriveStorage, err := storage.NewGoogleDriveStorage(
		context.Background(),
		cfg.GoogleDrive.KeyFilePath,
		logger,
	)
	if err != nil {
		logger.Error(traceID, "Failed to initialize Google Drive storage", nil, log.Error(err))
		os.Exit(1)
	}

	logger.Info(traceID, "Storage initialized", nil)

	// Initialize backup service
	backupService, err := backup.NewService(db, googleDriveStorage, cfg, logger)
	if err != nil {
		logger.Error(traceID, "Failed to initialize backup service", nil, log.Error(err))
		os.Exit(1)
	}

	logger.Info(traceID, "Backup service initialized", nil)

	// Initialize scheduler
	cronScheduler, err := scheduler.NewCronScheduler(cfg.Scheduler.Timezone, logger)
	if err != nil {
		logger.Error(traceID, "Failed to initialize scheduler", nil, log.Error(err))
		os.Exit(1)
	}

	// Add backup job to scheduler
	if err := cronScheduler.AddJob(cfg.Scheduler.Schedule, backupService); err != nil {
		logger.Error(traceID, "Failed to add backup job", nil, log.Error(err))
		os.Exit(1)
	}

	logger.Info(traceID, "Scheduler configured", nil,
		log.String("schedule", cfg.Scheduler.Schedule),
		log.String("timezone", cfg.Scheduler.Timezone),
	)

	// Start the scheduler
	if err := cronScheduler.Start(); err != nil {
		logger.Error(traceID, "Failed to start scheduler", nil, log.Error(err))
		os.Exit(1)
	}

	logger.Info(traceID, "DBEaseBackup started successfully", nil)
	fmt.Println("Database Auto Backup Service Started...")

	// Wait for shutdown signal
	waitForShutdown(logger, cronScheduler)
}

// initializeLogger creates and configures the logger
func initializeLogger(cfg *config.Config) (*log.Logger, error) {
	logLevel := parseLogLevel(cfg.Logging.Level)

	logConfig := log.Config{
		Service: "dbeasebackup",
		Env:     cfg.GoEnv,
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
func waitForShutdown(logger *log.Logger, sched *scheduler.CronScheduler) {
	traceID := "shutdown"

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	logger.Info(traceID, "Shutdown signal received", nil,
		log.String("signal", sig.String()),
	)
	fmt.Printf("\nReceived %v, shutting down...\n", sig)

	// Stop the scheduler
	ctx := context.Background()
	if err := sched.Stop(ctx); err != nil {
		logger.Error(traceID, "Error stopping scheduler", nil, log.Error(err))
	}

	logger.Info(traceID, "DBEaseBackup stopped", nil)
	fmt.Println("Database Auto Backup Service Stopped.")
}
