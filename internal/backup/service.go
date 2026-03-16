package backup

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/glennprays/dbeasebackup/config"
	"github.com/glennprays/dbeasebackup/pkg/database"
	"github.com/glennprays/dbeasebackup/pkg/storage"
	"github.com/glennprays/log"
)

// Service implements the backup orchestration
type Service struct {
	db      database.Database
	storage storage.Storage
	cfg     *config.Config
	logger  *log.Logger
}

// NewService creates a new backup service
func NewService(db database.Database, storage storage.Storage, cfg *config.Config, logger *log.Logger) (*Service, error) {
	traceID := "backup-service-init"

	s := &Service{
		db:      db,
		storage: storage,
		cfg:     cfg,
		logger:  logger,
	}

	// Ensure the backup table exists
	if err := s.EnsureBackupTable(context.Background()); err != nil {
		logger.Error(traceID, "Failed to ensure backup table exists", nil, log.Error(err))
		return nil, fmt.Errorf("unable to ensure backup table exists: %w", err)
	}

	logger.Info(traceID, "Backup service initialized", nil)
	return s, nil
}

// Name returns the name of the job (implements scheduler.Job interface)
func (s *Service) Name() string {
	return "postgres-backup"
}

// Execute runs the backup job (implements scheduler.Job interface)
func (s *Service) Execute(ctx context.Context) error {
	return s.Backup(ctx)
}

// Backup performs the complete backup workflow
func (s *Service) Backup(ctx context.Context) error {
	traceID := fmt.Sprintf("backup-%d", time.Now().Unix())

	s.logger.Info(traceID, "Starting backup", nil)

	// Create the backup dump
	backupFile, backupTime, err := s.createDump(ctx, traceID)
	if err != nil {
		return err
	}

	// Record the backup in the database
	if err := s.recordBackup(ctx, traceID, backupFile, backupTime); err != nil {
		return err
	}

	// Upload the backup to storage
	if err := s.uploadBackup(ctx, traceID, backupFile); err != nil {
		return err
	}

	// Delete the local backup file
	if err := s.deleteLocalBackup(traceID, backupFile); err != nil {
		return err
	}

	// Run garbage collection
	s.runGC(traceID)

	s.logger.Info(traceID, "Backup completed successfully", nil)
	return nil
}

// createDump creates a pg_dump backup file
func (s *Service) createDump(ctx context.Context, traceID string) (string, time.Time, error) {
	backupTime := time.Now()
	backupFile := fmt.Sprintf("backup_%s.tar", backupTime.Format("2006-01-02_15-04-05"))
	backupFilePath := fmt.Sprintf("%s/%s", s.cfg.Storage.BackupDir, backupFile)

	// Ensure backup directory exists
	if err := os.MkdirAll(s.cfg.Storage.BackupDir, os.ModePerm); err != nil {
		s.logger.Error(traceID, "Failed to create backup directory", nil,
			log.Error(err),
			log.String("directory", s.cfg.Storage.BackupDir),
		)
		return "", time.Time{}, fmt.Errorf("unable to create backup directory: %w", err)
	}

	// Create pg_dump command
	cmd := exec.CommandContext(ctx,
		"pg_dump",
		"-h", s.cfg.Database.Host,
		"-p", s.cfg.Database.Port,
		"-U", s.cfg.Database.User,
		"-d", s.cfg.Database.Database,
		"-F", "t",
		"-f", backupFilePath,
	)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", s.cfg.Database.Password))

	// Execute the dump
	if err := cmd.Run(); err != nil {
		s.logger.Error(traceID, "Failed to create database dump", nil,
			log.Error(err),
			log.String("file", backupFilePath),
		)
		return "", time.Time{}, fmt.Errorf("unable to backup database: %w", err)
	}

	s.logger.Info(traceID, "Database backup created", nil,
		log.String("path", backupFilePath),
	)

	return backupFilePath, backupTime, nil
}

// uploadBackup uploads the backup file to storage
func (s *Service) uploadBackup(ctx context.Context, traceID, backupFilePath string) error {
	file, err := os.Open(backupFilePath)
	if err != nil {
		s.logger.Error(traceID, "Failed to open backup file for upload", nil,
			log.Error(err),
			log.String("path", backupFilePath),
		)
		return fmt.Errorf("unable to open backup file: %w", err)
	}
	defer file.Close()

	// Get file info for the filename
	fileInfo, err := file.Stat()
	if err != nil {
		s.logger.Error(traceID, "Failed to get file info", nil,
			log.Error(err),
			log.String("path", backupFilePath),
		)
		return fmt.Errorf("unable to get file info: %w", err)
	}

	// Upload to storage
	uploadOpts := storage.UploadOptions{
		FolderID: s.cfg.GoogleDrive.FolderID,
	}

	if err := s.storage.Upload(ctx, file, fileInfo.Name(), uploadOpts); err != nil {
		return fmt.Errorf("unable to upload backup file: %w", err)
	}

	s.logger.Info(traceID, "Backup uploaded to storage", nil,
		log.String("filename", fileInfo.Name()),
	)

	return nil
}

// recordBackup records the backup metadata in the database
func (s *Service) recordBackup(ctx context.Context, traceID, backupFile string, backupTime time.Time) error {
	// Extract just the filename from the path
	filename := backupFile
	if idx := len(s.cfg.Storage.BackupDir) + 1; idx < len(backupFile) {
		filename = backupFile[idx:]
	}

	_, err := s.db.Exec(ctx,
		"INSERT INTO database_backups (backup_file, backup_time) VALUES ($1, $2)",
		filename, backupTime,
	)
	if err != nil {
		s.logger.Error(traceID, "Failed to record backup in database", nil,
			log.Error(err),
			log.String("file", filename),
		)
		return fmt.Errorf("unable to record backup: %w", err)
	}

	s.logger.Info(traceID, "Backup recorded in database", nil,
		log.String("file", filename),
	)

	return nil
}

// deleteLocalBackup removes the local backup file
func (s *Service) deleteLocalBackup(traceID, backupFilePath string) error {
	if err := os.Remove(backupFilePath); err != nil {
		s.logger.Error(traceID, "Failed to delete local backup file", nil,
			log.Error(err),
			log.String("path", backupFilePath),
		)
		return fmt.Errorf("unable to delete backup file: %w", err)
	}

	s.logger.Info(traceID, "Local backup file deleted", nil,
		log.String("path", backupFilePath),
	)

	return nil
}

// runGC runs garbage collection to manage memory
func (s *Service) runGC(traceID string) {
	s.logger.Debug(traceID, "Running garbage collection", nil)
	runtime.GC()
	s.logger.Debug(traceID, "Garbage collection completed", nil)
}

// EnsureBackupTable creates the backup tracking table if it doesn't exist
func (s *Service) EnsureBackupTable(ctx context.Context) error {
	traceID := "ensure-backup-table"

	createTableQuery := `
	CREATE TABLE IF NOT EXISTS database_backups (
		id SERIAL PRIMARY KEY,
		backup_file TEXT NOT NULL,
		backup_time TIMESTAMP NOT NULL
	);`

	_, err := s.db.Exec(ctx, createTableQuery)
	if err != nil {
		s.logger.Error(traceID, "Failed to create backup table", nil, log.Error(err))
		return fmt.Errorf("unable to create backup table: %w", err)
	}

	s.logger.Info(traceID, "Backup table ensured", nil)
	return nil
}

// ListBackups retrieves backup records from the database
func (s *Service) ListBackups(ctx context.Context, limit int) ([]BackupRecord, error) {
	traceID := "list-backups"

	query := "SELECT id, backup_file, backup_time FROM database_backups ORDER BY backup_time DESC"
	if limit > 0 {
		query = fmt.Sprintf("%s LIMIT %d", query, limit)
	}

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		s.logger.Error(traceID, "Failed to list backups", nil, log.Error(err))
		return nil, fmt.Errorf("unable to list backups: %w", err)
	}
	defer rows.Close()

	var backups []BackupRecord
	for rows.Next() {
		var record BackupRecord
		if err := rows.Scan(&record.ID, &record.File, &record.Time); err != nil {
			s.logger.Error(traceID, "Failed to scan backup record", nil, log.Error(err))
			return nil, fmt.Errorf("unable to scan backup record: %w", err)
		}
		backups = append(backups, record)
	}

	s.logger.Info(traceID, "Backups retrieved", nil, log.Int("count", len(backups)))
	return backups, nil
}

// BackupRecord represents a backup database record
type BackupRecord struct {
	ID   int
	File string
	Time time.Time
}

// Health checks if the backup service is healthy
func (s *Service) Health(ctx context.Context) error {
	if err := s.db.Ping(ctx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}
	if err := s.storage.Health(ctx); err != nil {
		return fmt.Errorf("storage health check failed: %w", err)
	}
	return nil
}

// io.Reader adapter for file upload
type readerAdapter struct {
	io.Reader
}

func (r *readerAdapter) Close() error {
	return nil
}
