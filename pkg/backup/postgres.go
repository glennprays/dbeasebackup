package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/glennprays/dbeasebackup/pkg/traceid"
	"github.com/glennprays/log"
)

// BackupFilenameTimestamp is the UTC layout used in backup filenames.
// It is shared by the dump provider, basebackup provider, and the
// retention parser.
const BackupFilenameTimestamp = "20060102T150405Z"

// BackupFilename builds the canonical storage filename:
// <target>_<mode>_<UTC timestamp>.<ext>
func BackupFilename(target, mode, ext string, t time.Time) string {
	return fmt.Sprintf("%s_%s_%s.%s", target, mode, t.UTC().Format(BackupFilenameTimestamp), ext)
}

// PostgresConfig holds PostgreSQL dump-specific configuration
type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	// TargetName identifies the owning target in filenames and logs.
	TargetName string
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

// Dump creates a pg_dump backup file in custom format (-F c)
func (p *PostgresProvider) Dump(ctx context.Context, backupDir string) (string, error) {
	traceID := traceid.FromContext(ctx)
	startTime := time.Now()

	p.logger.Info(traceID, "Starting database dump", nil,
		log.String("component", "postgres-provider"),
		log.String("target", p.cfg.TargetName),
		log.String("database", p.cfg.Database),
		log.String("timeout", p.timeout.String()),
	)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	backupTime := time.Now()
	backupFile := BackupFilename(p.cfg.TargetName, "dump", "dump", backupTime)
	backupFilePath := fmt.Sprintf("%s/%s", backupDir, backupFile)

	// Ensure backup directory exists
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		p.logger.Error(traceID, "Failed to create backup directory", nil,
			log.Error(err),
			log.String("directory", backupDir),
			log.String("component", "postgres-provider"),
		)
		return "", fmt.Errorf("unable to create backup directory: %w", err)
	}

	cmd := exec.CommandContext(ctx, "pg_dump",
		"-h", p.cfg.Host,
		"-p", p.cfg.Port,
		"-U", p.cfg.User,
		"-d", p.cfg.Database,
		"-F", "c",
		"-f", backupFilePath,
	)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", p.cfg.Password))

	stderr := NewLimitedBuffer()
	cmd.Stderr = stderr

	p.logger.Debug(traceID, "Executing pg_dump command", nil,
		log.String("component", "postgres-provider"),
		log.String("command", cmd.String()),
	)

	err := cmd.Run()

	if ctx.Err() != nil {
		// Context was cancelled or timed out — clean up partial file
		os.Remove(backupFilePath)
		p.logger.Error(traceID, "Backup context cancelled", nil,
			log.String("component", "postgres-provider"),
			log.String("ctx_error", ctx.Err().Error()),
		)
		return "", fmt.Errorf("backup cancelled: %w", ctx.Err())
	}

	if err != nil {
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

// ValidateDependencies checks if pg_dump and pg_restore are available in PATH
func (p *PostgresProvider) ValidateDependencies() error {
	if _, err := exec.LookPath("pg_dump"); err != nil {
		return fmt.Errorf("pg_dump not found in PATH: please install PostgreSQL client tools (postgresql-client on Debian/Ubuntu, postgresql on macOS)")
	}
	if _, err := exec.LookPath("pg_restore"); err != nil {
		return fmt.Errorf("pg_restore not found in PATH: needed for backup verification; please install PostgreSQL client tools (postgresql-client on Debian/Ubuntu, postgresql on macOS)")
	}
	return nil
}
