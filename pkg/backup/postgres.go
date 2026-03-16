package backup

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/glennprays/dbeasebackup/pkg/traceid"
	"github.com/glennprays/log"
)

// PostgresConfig holds PostgreSQL-specific configuration
type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

// PostgresProvider implements the Provider interface for PostgreSQL
type PostgresProvider struct {
	cfg     PostgresConfig
	logger  *log.Logger
	timeout time.Duration
}

// NewPostgresProvider creates a new PostgreSQL backup provider
func NewPostgresProvider(cfg PostgresConfig, logger *log.Logger, timeout time.Duration) *PostgresProvider {
	if timeout == 0 {
		timeout = 30 * time.Minute // default
	}
	return &PostgresProvider{
		cfg:     cfg,
		logger:  logger,
		timeout: timeout,
	}
}

// Name returns the provider name
func (p *PostgresProvider) Name() string {
	return "postgres-backup"
}

// Dump creates a pg_dump backup file
func (p *PostgresProvider) Dump(ctx context.Context, backupDir string) (string, error) {
	traceID := traceid.FromContext(ctx)
	startTime := time.Now()

	p.logger.Info(traceID, "Starting database dump", nil,
		log.String("component", "postgres-provider"),
		log.String("database", p.cfg.Database),
		log.String("timeout", p.timeout.String()),
	)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	backupTime := time.Now()
	backupFile := fmt.Sprintf("backup_%s.tar", backupTime.Format("2006-01-02_15-04-05"))
	backupFilePath := fmt.Sprintf("%s/%s", backupDir, backupFile)

	// Ensure backup directory exists
	if err := os.MkdirAll(backupDir, os.ModePerm); err != nil {
		p.logger.Error(traceID, "Failed to create backup directory", nil,
			log.Error(err),
			log.String("directory", backupDir),
			log.String("component", "postgres-provider"),
		)
		return "", fmt.Errorf("unable to create backup directory: %w", err)
	}

	// Create pg_dump command
	cmd := exec.CommandContext(ctx,
		"pg_dump",
		"-h", p.cfg.Host,
		"-p", p.cfg.Port,
		"-U", p.cfg.User,
		"-d", p.cfg.Database,
		"-F", "t",
		"-f", backupFilePath,
	)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", p.cfg.Password))

	// Capture stderr to get actual error messages from pg_dump
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// Execute the dump
	if err := cmd.Run(); err != nil {
		// Check for timeout first
		if ctx.Err() == context.DeadlineExceeded {
			p.logger.Error(traceID, "Database dump timed out", nil,
				log.String("timeout", p.timeout.String()),
				log.String("component", "postgres-provider"),
			)
			return "", fmt.Errorf("database dump timed out after %s", p.timeout)
		}

		pgDumpError := strings.TrimSpace(stderr.String())
		if pgDumpError == "" {
			pgDumpError = "(no stderr output)"
		}

		p.logger.Error(traceID, "Failed to create database dump", nil,
			log.Error(err),
			log.String("file", backupFilePath),
			log.String("component", "postgres-provider"),
			log.String("pg_dump_error", pgDumpError),
		)
		return "", fmt.Errorf("unable to backup database: %w (pg_dump: %s)", err, pgDumpError)
	}

	p.logger.Info(traceID, "Database dump completed", nil,
		log.String("path", backupFilePath),
		log.String("component", "postgres-provider"),
		log.String("duration", time.Since(startTime).String()),
	)

	return backupFilePath, nil
}

// Cleanup removes the backup file
func (p *PostgresProvider) Cleanup(ctx context.Context, filePath string) error {
	traceID := traceid.FromContext(ctx)
	startTime := time.Now()

	p.logger.Info(traceID, "Starting cleanup", nil,
		log.String("path", filePath),
		log.String("component", "postgres-provider"),
	)

	if err := os.Remove(filePath); err != nil {
		p.logger.Error(traceID, "Failed to cleanup backup file", nil,
			log.Error(err),
			log.String("path", filePath),
			log.String("component", "postgres-provider"),
		)
		return fmt.Errorf("unable to cleanup backup file: %w", err)
	}

	p.logger.Info(traceID, "Cleanup completed", nil,
		log.String("path", filePath),
		log.String("component", "postgres-provider"),
		log.String("duration", time.Since(startTime).String()),
	)

	return nil
}

// ValidateDependencies checks if pg_dump is available in PATH
func (p *PostgresProvider) ValidateDependencies() error {
	_, err := exec.LookPath("pg_dump")
	if err != nil {
		return fmt.Errorf("pg_dump not found in PATH: please install PostgreSQL client tools (postgresql-client on Debian/Ubuntu, postgresql on macOS)")
	}
	return nil
}
