package backup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/glennprays/dbeasebackup/config"
	backupprovider "github.com/glennprays/dbeasebackup/pkg/backup"
	"github.com/glennprays/dbeasebackup/pkg/database"
	"github.com/glennprays/dbeasebackup/pkg/storage"
	"github.com/glennprays/dbeasebackup/pkg/traceid"
	"github.com/glennprays/log"
)

// Service implements the backup orchestration
type Service struct {
	db       database.Database
	storage  storage.Storage
	provider backupprovider.Provider
	cfg      *config.Config
	logger   *log.Logger
}

// NewService creates a new backup service
func NewService(db database.Database, storage storage.Storage, provider backupprovider.Provider, cfg *config.Config, logger *log.Logger) (*Service, error) {
	traceID := "backup-service-init"

	s := &Service{
		db:       db,
		storage:  storage,
		provider: provider,
		cfg:      cfg,
		logger:   logger,
	}

	// Ensure the backup table exists
	if err := s.EnsureBackupTable(context.Background()); err != nil {
		logger.Error(traceID, "Failed to ensure backup table exists", nil, log.Error(err))
		return nil, fmt.Errorf("unable to ensure backup table exists: %w", err)
	}

	logger.Info(traceID, "Backup service initialized", nil,
		log.String("provider", provider.Name()),
	)
	return s, nil
}

// Name returns the name of the job (implements scheduler.Job interface)
func (s *Service) Name() string {
	return s.provider.Name()
}

// Execute runs the backup job (implements scheduler.Job interface)
func (s *Service) Execute(ctx context.Context) error {
	return s.Backup(ctx)
}

// Backup performs the complete backup workflow
func (s *Service) Backup(ctx context.Context) error {
	traceID := traceid.FromContext(ctx)
	startTime := time.Now()

	s.logger.Info(traceID, "Starting backup workflow", nil,
		log.String("component", "backup-service"),
		log.String("provider", s.provider.Name()),
	)

	// Create the backup dump using provider
	backupFile, err := s.provider.Dump(ctx, s.cfg.BACKUP_DIR)
	if err != nil {
		return err
	}

	// Ensure local file is cleaned up on any failure after dump
	cleanedUp := false
	defer func() {
		if !cleanedUp {
			if removeErr := s.provider.Cleanup(ctx, backupFile); removeErr != nil {
				s.logger.Error(traceID, "Failed to clean up local backup after failure", nil,
					log.Error(removeErr),
					log.String("path", backupFile),
					log.String("component", "backup-service"),
				)
			}
		}
	}()

	backupTime := time.Now()

	// Record the backup in the database
	if err := s.recordBackup(ctx, traceID, backupFile, backupTime); err != nil {
		return err
	}

	// Upload the backup to storage
	if err := s.uploadBackup(ctx, traceID, backupFile); err != nil {
		return err
	}

	// Verify the backup (optional)
	if s.cfg.BACKUP_VERIFY {
		if err := s.verifyBackup(ctx, traceID, backupFile); err != nil {
			s.logger.Warn(traceID, "Backup verification failed, but backup was uploaded successfully", nil,
				log.Error(err),
				log.String("component", "backup-service"),
			)
		}
	}

	// Delete the local backup file
	if err := s.deleteLocalBackup(ctx, traceID, backupFile); err != nil {
		return err
	}
	cleanedUp = true

	s.logger.Info(traceID, "Backup workflow completed successfully", nil,
		log.String("component", "backup-service"),
		log.String("duration", time.Since(startTime).String()),
	)
	return nil
}

// uploadBackup uploads the backup file to storage
func (s *Service) uploadBackup(ctx context.Context, traceID, backupFilePath string) error {
	file, err := os.Open(backupFilePath)
	if err != nil {
		s.logger.Error(traceID, "Failed to open backup file for upload", nil,
			log.Error(err),
			log.String("path", backupFilePath),
			log.String("component", "backup-service"),
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
			log.String("component", "backup-service"),
		)
		return fmt.Errorf("unable to get file info: %w", err)
	}

	// Upload to storage - use FolderID for Google Drive, or as prefix for S3
	uploadOpts := storage.UploadOptions{
		FolderID: s.cfg.GOOGLE_DRIVE_FOLDER_ID,
	}

	if err := s.storage.Upload(ctx, file, fileInfo.Name(), uploadOpts); err != nil {
		return fmt.Errorf("unable to upload backup file: %w", err)
	}

	// Reclaim memory from upload buffers (Google Drive SDK buffers entire file)
	runtime.GC()

	s.logger.Info(traceID, "Backup uploaded to storage", nil,
		log.String("filename", fileInfo.Name()),
		log.String("component", "backup-service"),
	)

	return nil
}

// verifyBackup downloads and validates the backup integrity using pg_restore --list
func (s *Service) verifyBackup(ctx context.Context, traceID, backupFilePath string) error {
	startTime := time.Now()

	// Extract filename from path
	filename := filepath.Base(backupFilePath)

	s.logger.Info(traceID, "Starting backup verification", nil,
		log.String("filename", filename),
		log.String("component", "backup-service"),
	)

	// Download the backup from storage
	reader, err := s.storage.Download(ctx, filename)
	if err != nil {
		return fmt.Errorf("unable to download backup for verification: %w", err)
	}
	defer reader.Close()

	// Run pg_restore --list to validate the backup structure
	// This parses the archive without actually restoring data
	cmd := exec.CommandContext(ctx, "pg_restore", "--list", "-")
	cmd.Stdin = reader

	output, err := cmd.CombinedOutput()
	if err != nil {
		s.logger.Error(traceID, "Backup verification failed", nil,
			log.Error(err),
			log.String("output", string(output)),
			log.String("component", "backup-service"),
		)
		return fmt.Errorf("backup verification failed: %w", err)
	}

	s.logger.Info(traceID, "Backup verification completed", nil,
		log.String("filename", filename),
		log.String("component", "backup-service"),
		log.String("duration", time.Since(startTime).String()),
	)

	return nil
}

// recordBackup records the backup metadata in the database
func (s *Service) recordBackup(ctx context.Context, traceID, backupFile string, backupTime time.Time) error {
	filename := filepath.Base(backupFile)

	_, err := s.db.Exec(ctx,
		"INSERT INTO database_backups (backup_file, backup_time) VALUES ($1, $2)",
		filename, backupTime,
	)
	if err != nil {
		s.logger.Error(traceID, "Failed to record backup in database", nil,
			log.Error(err),
			log.String("file", filename),
			log.String("component", "backup-service"),
		)
		return fmt.Errorf("unable to record backup: %w", err)
	}

	s.logger.Info(traceID, "Backup recorded in database", nil,
		log.String("file", filename),
		log.String("component", "backup-service"),
	)

	return nil
}

// deleteLocalBackup removes the local backup file using the provider's Cleanup
func (s *Service) deleteLocalBackup(ctx context.Context, traceID, backupFilePath string) error {
	if err := s.provider.Cleanup(ctx, backupFilePath); err != nil {
		s.logger.Error(traceID, "Failed to delete local backup file", nil,
			log.Error(err),
			log.String("path", backupFilePath),
			log.String("component", "backup-service"),
		)
		return fmt.Errorf("unable to delete backup file: %w", err)
	}

	s.logger.Info(traceID, "Local backup file deleted", nil,
		log.String("path", backupFilePath),
		log.String("component", "backup-service"),
	)

	return nil
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating backup rows: %w", err)
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

// CleanupJob is a scheduler job that cleans up old backups
type CleanupJob struct {
	service *Service
}

// NewCleanupJob creates a new cleanup job
func NewCleanupJob(service *Service) *CleanupJob {
	return &CleanupJob{service: service}
}

// Name returns the name of the job (implements scheduler.Job interface)
func (j *CleanupJob) Name() string {
	return "backup-cleanup"
}

// Execute runs the cleanup job (implements scheduler.Job interface)
func (j *CleanupJob) Execute(ctx context.Context) error {
	return j.service.Cleanup(ctx)
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

// Cleanup deletes backups older than the retention period
func (s *Service) Cleanup(ctx context.Context) error {
	traceID := traceid.FromContext(ctx)
	startTime := time.Now()

	// Skip cleanup if retention is disabled (0 days)
	if s.cfg.BACKUP_RETENTION_DAYS <= 0 {
		s.logger.Debug(traceID, "Backup retention is disabled, skipping cleanup", nil)
		return nil
	}

	s.logger.Info(traceID, "Starting backup cleanup", nil,
		log.String("component", "backup-service"),
		log.Int("retention_days", s.cfg.BACKUP_RETENTION_DAYS),
	)

	// Calculate cutoff time
	cutoff := time.Now().AddDate(0, 0, -s.cfg.BACKUP_RETENTION_DAYS)

	// Get backups older than cutoff
	records, err := s.GetBackupsOlderThan(ctx, cutoff)
	if err != nil {
		return err
	}

	if len(records) == 0 {
		s.logger.Info(traceID, "No old backups to clean up", nil, log.Any("cutoff", cutoff))
		return nil
	}

	s.logger.Info(traceID, "Found old backups to delete", nil,
		log.Int("count", len(records)),
		log.Any("cutoff", cutoff),
	)

	// Delete each backup
	deletedCount := 0
	for _, record := range records {
		if err := s.deleteBackup(ctx, traceID, record); err != nil {
			s.logger.Error(traceID, "Failed to delete backup", nil,
				log.Error(err),
				log.Int("backup_id", record.ID),
				log.String("backup_file", record.File),
			)
			continue
		}
		deletedCount++
	}

	s.logger.Info(traceID, "Backup cleanup completed", nil,
		log.String("component", "backup-service"),
		log.Int("deleted_count", deletedCount),
		log.String("duration", time.Since(startTime).String()),
	)

	return nil
}

// GetBackupsOlderThan retrieves backup records older than the given cutoff time
func (s *Service) GetBackupsOlderThan(ctx context.Context, cutoff time.Time) ([]BackupRecord, error) {
	traceID := traceid.FromContext(ctx)

	query := "SELECT id, backup_file, backup_time FROM database_backups WHERE backup_time < $1"

	rows, err := s.db.Query(ctx, query, cutoff)
	if err != nil {
		s.logger.Error(traceID, "Failed to query old backups", nil, log.Error(err))
		return nil, fmt.Errorf("unable to query old backups: %w", err)
	}
	defer rows.Close()

	var records []BackupRecord
	for rows.Next() {
		var record BackupRecord
		if err := rows.Scan(&record.ID, &record.File, &record.Time); err != nil {
			s.logger.Error(traceID, "Failed to scan backup record", nil, log.Error(err))
			return nil, fmt.Errorf("unable to scan backup record: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating backup rows: %w", err)
	}

	return records, nil
}

// deleteBackup deletes a backup file from storage and its record from the database
func (s *Service) deleteBackup(ctx context.Context, traceID string, record BackupRecord) error {
	// Delete from storage
	err := s.storage.Delete(ctx, record.File)
	if err != nil {
		// If file not found in storage, log warning but continue to delete the database record
		// This handles the case where the file was manually deleted from storage
		if isNotFoundError(err) {
			s.logger.Warn(traceID, "Backup file not found in storage, will delete database record only", nil,
				log.String("backup_file", record.File),
				log.Error(err),
			)
		} else {
			return fmt.Errorf("unable to delete backup from storage: %w", err)
		}
	} else {
		s.logger.Info(traceID, "Backup file deleted from storage", nil,
			log.String("backup_file", record.File),
		)
	}

	// Delete from database
	if err := s.DeleteBackupRecord(ctx, record.ID); err != nil {
		return fmt.Errorf("unable to delete backup record: %w", err)
	}

	s.logger.Info(traceID, "Backup record deleted from database", nil,
		log.Int("backup_id", record.ID),
	)

	return nil
}

func isNotFoundError(err error) bool {
	return errors.Is(err, storage.ErrNotFound)
}

// DeleteBackupRecord deletes a backup record from the database
func (s *Service) DeleteBackupRecord(ctx context.Context, id int) error {
	_, err := s.db.Exec(ctx, "DELETE FROM database_backups WHERE id = $1", id)
	return err
}
