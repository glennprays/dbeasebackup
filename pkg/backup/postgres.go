package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

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
	cfg    PostgresConfig
	logger *log.Logger
}

// NewPostgresProvider creates a new PostgreSQL backup provider
func NewPostgresProvider(cfg PostgresConfig, logger *log.Logger) *PostgresProvider {
	return &PostgresProvider{
		cfg:    cfg,
		logger: logger,
	}
}

// Name returns the provider name
func (p *PostgresProvider) Name() string {
	return "postgres-backup"
}

// Dump creates a pg_dump backup file
func (p *PostgresProvider) Dump(ctx context.Context, backupDir string) (string, error) {
	traceID := "postgres-dump"

	backupTime := time.Now()
	backupFile := fmt.Sprintf("backup_%s.tar", backupTime.Format("2006-01-02_15-04-05"))
	backupFilePath := fmt.Sprintf("%s/%s", backupDir, backupFile)

	// Ensure backup directory exists
	if err := os.MkdirAll(backupDir, os.ModePerm); err != nil {
		p.logger.Error(traceID, "Failed to create backup directory", nil,
			log.Error(err),
			log.String("directory", backupDir),
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

	// Execute the dump
	if err := cmd.Run(); err != nil {
		p.logger.Error(traceID, "Failed to create database dump", nil,
			log.Error(err),
			log.String("file", backupFilePath),
		)
		return "", fmt.Errorf("unable to backup database: %w", err)
	}

	p.logger.Info(traceID, "Database backup created", nil,
		log.String("path", backupFilePath),
	)

	return backupFilePath, nil
}

// Cleanup removes the backup file
func (p *PostgresProvider) Cleanup(filePath string) error {
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("unable to cleanup backup file: %w", err)
	}
	return nil
}
