package backup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/glennprays/dbeasebackup/config"
	backupprovider "github.com/glennprays/dbeasebackup/pkg/backup"
	"github.com/glennprays/dbeasebackup/pkg/notifier"
	"github.com/glennprays/dbeasebackup/pkg/storage"
	"github.com/glennprays/dbeasebackup/pkg/traceid"
	"github.com/glennprays/log"
)

// TargetStatus is the observable per-target run state, exposed through the
// health endpoints.
type TargetStatus struct {
	Target      string    `json:"target"`
	LastRun     time.Time `json:"last_run"`
	LastSuccess time.Time `json:"last_success"`
	LastError   string    `json:"last_error"`
}

// Service implements backup orchestration for exactly one target. Storage
// listing is the source of truth for retention; there is no catalog table.
type Service struct {
	target   *config.Target
	storage  storage.Storage
	provider backupprovider.Provider
	logger   *log.Logger
	notifier notifier.Notifier

	mu         sync.Mutex
	lastResult TargetStatus
}

// NewService creates a new per-target backup service.
func NewService(st storage.Storage, provider backupprovider.Provider, target *config.Target, logger *log.Logger, n notifier.Notifier) (*Service, error) {
	if target == nil {
		return nil, fmt.Errorf("target must not be nil")
	}
	s := &Service{
		target:   target,
		storage:  st,
		provider: provider,
		logger:   logger,
		notifier: n,
		lastResult: TargetStatus{
			Target: target.Name,
		},
	}

	logger.Info("backup-service-init", "Backup service initialized", nil,
		log.String("target", target.Name),
		log.String("provider", provider.Name()),
		log.String("mode", target.Mode),
	)
	return s, nil
}

// Target returns the service's target name.
func (s *Service) Target() string {
	return s.target.Name
}

// Status returns the latest run state for health reporting.
func (s *Service) Status() TargetStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastResult
}

func (s *Service) recordStatus(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastResult.LastRun = time.Now()
	if err != nil {
		s.lastResult.LastError = err.Error()
		return
	}
	s.lastResult.LastSuccess = s.lastResult.LastRun
	s.lastResult.LastError = ""
}

func (s *Service) notify(ctx context.Context, event notifier.Event) {
	if s.notifier == nil {
		return
	}
	s.notifier.Notify(ctx, event)
}

// Name returns the job name (implements scheduler.Job interface).
func (s *Service) Name() string {
	return "backup:" + s.target.Name
}

// Execute runs the backup job (implements scheduler.Job interface).
func (s *Service) Execute(ctx context.Context) error {
	return s.Backup(ctx)
}

// Backup performs the complete backup workflow for this target:
// dump -> upload -> (dump-mode) verify -> local cleanup.
// Notifications are NOT sent here; the Runner owns notify-once semantics.
func (s *Service) Backup(ctx context.Context) error {
	traceID := traceid.FromContext(ctx)
	startTime := time.Now()

	s.logger.Info(traceID, "Starting backup workflow", nil,
		log.String("component", "backup-service"),
		log.String("target", s.target.Name),
		log.String("provider", s.provider.Name()),
	)

	backupFile, err := s.provider.Dump(ctx, s.target.BackupDir)
	retErr := err
	if err == nil {
		retErr = s.runBackupPipeline(ctx, traceID, backupFile, startTime)
	}

	s.recordStatus(retErr)
	return retErr
}

// runBackupPipeline handles everything after a successful dump.
func (s *Service) runBackupPipeline(ctx context.Context, traceID, backupFile string, startTime time.Time) error {
	var backupFilename string
	var backupFileSize int64

	// Capture file metadata
	backupFilename = filepath.Base(backupFile)
	if info, statErr := os.Stat(backupFile); statErr == nil {
		backupFileSize = info.Size()
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

	if err := s.uploadBackup(ctx, traceID, backupFile); err != nil {
		return err
	}

	// dump mode: download + pg_restore --list after upload (warn-not-fail).
	// basebackup mode: the provider already ran pg_verifybackup pre-upload.
	if s.target.Mode == "dump" && s.target.ShouldVerify() {
		if err := s.verifyBackup(ctx, traceID, backupFile); err != nil {
			s.logger.Warn(traceID, "Backup verification failed, but backup was uploaded successfully", nil,
				log.Error(err),
				log.String("component", "backup-service"),
			)
		}
	}

	if err := s.deleteLocalBackup(ctx, traceID, backupFile); err != nil {
		return err
	}
	cleanedUp = true

	s.logger.Info(traceID, "Backup workflow completed successfully", nil,
		log.String("component", "backup-service"),
		log.String("target", s.target.Name),
		log.String("filename", backupFilename),
		log.Int64("size", backupFileSize),
		log.String("duration", time.Since(startTime).String()),
	)
	return nil
}

// uploadBackup uploads the backup file to storage with per-target placement.
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

	fileInfo, err := file.Stat()
	if err != nil {
		s.logger.Error(traceID, "Failed to get file info", nil,
			log.Error(err),
			log.String("path", backupFilePath),
			log.String("component", "backup-service"),
		)
		return fmt.Errorf("unable to get file info: %w", err)
	}

	// FolderID doubles as per-target placement: Drive parent folder, or an
	// extra S3 key segment (see S3Storage.Upload).
	uploadOpts := storage.UploadOptions{
		FolderID: s.target.Storage.FolderID,
	}

	if err := s.storage.Upload(ctx, file, fileInfo.Name(), uploadOpts); err != nil {
		return fmt.Errorf("unable to upload backup file: %w", err)
	}

	s.logger.Info(traceID, "Backup uploaded to storage", nil,
		log.String("filename", fileInfo.Name()),
		log.String("target", s.target.Name),
		log.String("component", "backup-service"),
	)
	return nil
}

// verifyBackup downloads and validates the backup integrity using
// pg_restore --list (streamed; no disk writes).
func (s *Service) verifyBackup(ctx context.Context, traceID, backupFilePath string) error {
	startTime := time.Now()

	filename := filepath.Base(backupFilePath)

	s.logger.Info(traceID, "Starting backup verification", nil,
		log.String("filename", filename),
		log.String("component", "backup-service"),
	)

	reader, err := s.storage.Download(ctx, filename)
	if err != nil {
		return fmt.Errorf("unable to download backup for verification: %w", err)
	}
	if reader == nil {
		return fmt.Errorf("storage returned nil reader for %s", filename)
	}
	defer reader.Close()

	// Run pg_restore --list to validate the backup structure.
	// This parses the archive without actually restoring data.
	// No filename argument: pg_restore reads standard input.
	cmd := exec.CommandContext(ctx, "pg_restore", "--list")
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

// deleteLocalBackup removes the local backup file using the provider's Cleanup.
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

// CleanupJob is a scheduler job that cleans up one target's expired backups.
type CleanupJob struct {
	service *Service
}

// NewCleanupJob creates a new cleanup job for a target service.
func NewCleanupJob(service *Service) *CleanupJob {
	return &CleanupJob{service: service}
}

// Name returns the job name (implements scheduler.Job interface).
func (j *CleanupJob) Name() string {
	return "backup-cleanup:" + j.service.Target()
}

// Execute runs the cleanup job (implements scheduler.Job interface).
func (j *CleanupJob) Execute(ctx context.Context) error {
	return j.service.Cleanup(ctx)
}

// Health checks whether this target's storage dependency is reachable.
func (s *Service) Health(ctx context.Context) error {
	if err := s.storage.Health(ctx); err != nil {
		return fmt.Errorf("storage health check failed: %w", err)
	}
	return nil
}

// Cleanup deletes storage objects older than the target's retention window.
// Storage listing is the source of truth; filenames not matching the
// canonical <target>_<mode>_<UTC stamp>.<ext> pattern are skipped with a warn.
func (s *Service) Cleanup(ctx context.Context) error {
	traceID := traceid.FromContext(ctx)
	startTime := time.Now()

	// Retention disabled (0 days = keep forever)
	if s.target.RetentionCount() <= 0 {
		s.logger.Debug(traceID, "Backup retention is disabled, skipping cleanup", nil,
			log.String("target", s.target.Name),
		)
		return nil
	}

	s.logger.Info(traceID, "Starting backup cleanup", nil,
		log.String("component", "backup-service"),
		log.String("target", s.target.Name),
		log.Int("retention_days", s.target.RetentionCount()),
	)

	files, err := s.storage.List(ctx, storage.ListOptions{
		FolderID: s.target.Storage.FolderID,
		Prefix:   s.target.Name + "_",
	})
	if err != nil {
		s.logger.Error(traceID, "Failed to list backups for cleanup", nil,
			log.Error(err),
			log.String("target", s.target.Name),
			log.String("component", "backup-service"),
		)
		return fmt.Errorf("unable to list backups for cleanup: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -s.target.RetentionCount())
	deletedCount, skippedCount := 0, 0
	for _, f := range files {
		ts, ok := ParseBackupFilenameTime(f.Name)
		if !ok {
			s.logger.Warn(traceID, "Skipping file with unparseable backup name", nil,
				log.String("file", f.Name),
				log.String("target", s.target.Name),
				log.String("component", "backup-service"),
			)
			skippedCount++
			continue
		}
		if !ts.Before(cutoff) {
			continue
		}

		if err := s.storage.Delete(ctx, f.Name); err != nil {
			if isNotFoundError(err) {
				s.logger.Warn(traceID, "Backup already absent in storage", nil,
					log.String("file", f.Name),
					log.String("component", "backup-service"),
				)
				continue
			}
			s.logger.Error(traceID, "Failed to delete expired backup", nil,
				log.Error(err),
				log.String("file", f.Name),
				log.String("component", "backup-service"),
			)
			continue
		}
		deletedCount++
		s.logger.Info(traceID, "Expired backup deleted", nil,
			log.String("file", f.Name),
			log.String("component", "backup-service"),
		)
	}

	s.logger.Info(traceID, "Backup cleanup completed", nil,
		log.String("component", "backup-service"),
		log.String("target", s.target.Name),
		log.Int("deleted_count", deletedCount),
		log.Int("skipped_count", skippedCount),
		log.String("duration", time.Since(startTime).String()),
	)
	return nil
}

// ParseBackupFilenameTime extracts the UTC timestamp from a canonical
// backup filename: <target>_<mode>_<timestamp>.<ext>.
func ParseBackupFilenameTime(name string) (time.Time, bool) {
	// Strip extension (tar.gz has two dots; handle both).
	base := name
	if strings.HasSuffix(base, ".tar.gz") {
		base = strings.TrimSuffix(base, ".tar.gz")
	} else if i := strings.LastIndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}

	// Last underscore-separated segment is the timestamp.
	i := strings.LastIndexByte(base, '_')
	if i < 0 {
		return time.Time{}, false
	}
	stamp := base[i+1:]

	ts, err := time.Parse(backupprovider.BackupFilenameTimestamp, stamp)
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
}

func isNotFoundError(err error) bool {
	return errors.Is(err, storage.ErrNotFound)
}
