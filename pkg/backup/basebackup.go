package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/glennprays/dbeasebackup/pkg/traceid"
	"github.com/glennprays/log"
)

// BasebackupConfig holds pg_basebackup-specific configuration.
type BasebackupConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	// TargetName identifies the owning target in filenames and logs.
	TargetName string
}

// BasebackupProvider implements the Provider interface using physical
// cluster copies (pg_basebackup) verified with pg_verifybackup before upload.
type BasebackupProvider struct {
	cfg     BasebackupConfig
	logger  *log.Logger
	timeout time.Duration
}

// NewBasebackupProvider creates a new pg_basebackup provider.
func NewBasebackupProvider(cfg BasebackupConfig, logger *log.Logger, timeout time.Duration) *BasebackupProvider {
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	return &BasebackupProvider{
		cfg:     cfg,
		logger:  logger,
		timeout: timeout,
	}
}

// Name returns the provider name.
func (p *BasebackupProvider) Name() string {
	return "postgres-basebackup"
}

// Dump runs pg_basebackup into a plain-format directory, verifies it with
// pg_verifybackup, streams the directory into a tar.gz file, removes the
// directory, and returns the tarball path.
//
// Disk peak is ~2x the cluster size (plain dir + tarball) — accepted for
// this phase.
func (p *BasebackupProvider) Dump(ctx context.Context, backupDir string) (string, error) {
	traceID := traceid.FromContext(ctx)
	startTime := time.Now()

	p.logger.Info(traceID, "Starting base backup", nil,
		log.String("component", "basebackup-provider"),
		log.String("target", p.cfg.TargetName),
		log.String("timeout", p.timeout.String()),
	)

	// Create context with timeout (covers basebackup + verify + tar)
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	if err := os.MkdirAll(backupDir, 0750); err != nil {
		p.logger.Error(traceID, "Failed to create backup directory", nil,
			log.Error(err),
			log.String("directory", backupDir),
			log.String("component", "basebackup-provider"),
		)
		return "", fmt.Errorf("unable to create backup directory: %w", err)
	}

	backupTime := time.Now()
	plainDir := filepath.Join(backupDir, BackupFilename(p.cfg.TargetName, "basebackup", "data", backupTime))

	// Stage 1: pg_basebackup in plain format so backup_manifest is written.
	runCmd := exec.CommandContext(ctx, "pg_basebackup",
		"-h", p.cfg.Host,
		"-p", p.cfg.Port,
		"-U", p.cfg.User,
		"-D", plainDir,
		"-Fp", // plain format
		"-Xs", // stream WAL
		"-c", "fast",
	)
	runCmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", p.cfg.Password))

	stderr := NewLimitedBuffer()
	runCmd.Stderr = stderr

	p.logger.Debug(traceID, "Executing pg_basebackup command", nil,
		log.String("component", "basebackup-provider"),
		log.String("command", runCmd.String()),
	)

	if err := runCmd.Run(); err != nil {
		os.RemoveAll(plainDir)
		basebackupErr := strings.TrimSpace(stderr.String())
		if basebackupErr == "" {
			basebackupErr = "(no stderr output)"
		}
		if ctx.Err() != nil {
			p.logger.Error(traceID, "Base backup context cancelled", nil,
				log.String("component", "basebackup-provider"),
				log.String("ctx_error", ctx.Err().Error()),
			)
			return "", fmt.Errorf("base backup cancelled: %w", ctx.Err())
		}
		p.logger.Error(traceID, "Failed to run pg_basebackup", nil,
			log.Error(err),
			log.String("component", "basebackup-provider"),
			log.String("pg_basebackup_error", basebackupErr),
		)
		return "", fmt.Errorf("unable to run pg_basebackup: %w (pg_basebackup: %s)", err, basebackupErr)
	}

	// Stage 2: verify before upload. Failure means no backup is uploaded.
	if err := p.verifyBackupDir(ctx, traceID, plainDir); err != nil {
		os.RemoveAll(plainDir)
		return "", fmt.Errorf("base backup verification failed: %w", err)
	}

	// Stage 3: stream the directory into a single tar.gz.
	tarballPath := filepath.Join(backupDir, BackupFilename(p.cfg.TargetName, "basebackup", "tar.gz", backupTime))
	if err := tarDirectory(plainDir, tarballPath); err != nil {
		os.RemoveAll(plainDir)
		os.Remove(tarballPath)
		p.logger.Error(traceID, "Failed to tar base backup directory", nil,
			log.Error(err),
			log.String("component", "basebackup-provider"),
		)
		return "", fmt.Errorf("unable to archive base backup: %w", err)
	}

	// Stage 4: remove the plain directory.
	if err := os.RemoveAll(plainDir); err != nil {
		p.logger.Warn(traceID, "Failed to remove plain backup directory after archiving", nil,
			log.Error(err),
			log.String("path", plainDir),
			log.String("component", "basebackup-provider"),
		)
	}

	p.logger.Info(traceID, "Base backup completed", nil,
		log.String("path", tarballPath),
		log.String("component", "basebackup-provider"),
		log.String("duration", time.Since(startTime).String()),
	)

	return tarballPath, nil
}

// verifyBackupDir runs pg_verifybackup against the plain directory.
func (p *BasebackupProvider) verifyBackupDir(ctx context.Context, traceID, dir string) error {
	startTime := time.Now()

	cmd := exec.CommandContext(ctx, "pg_verifybackup", dir)
	stderr := NewLimitedBuffer()
	cmd.Stderr = stderr

	p.logger.Info(traceID, "Verifying base backup with pg_verifybackup", nil,
		log.String("dir", dir),
		log.String("component", "basebackup-provider"),
	)

	if err := cmd.Run(); err != nil {
		verifyErr := strings.TrimSpace(stderr.String())
		if verifyErr == "" {
			verifyErr = "(no stderr output)"
		}
		p.logger.Error(traceID, "pg_verifybackup failed", nil,
			log.Error(err),
			log.String("component", "basebackup-provider"),
			log.String("pg_verifybackup_error", verifyErr),
		)
		return fmt.Errorf("pg_verifybackup failed: %w (pg_verifybackup: %s)", err, verifyErr)
	}

	p.logger.Info(traceID, "Base backup verification completed", nil,
		log.String("component", "basebackup-provider"),
		log.String("duration", time.Since(startTime).String()),
	)
	return nil
}

// tarDirectory streams dir into a gzipped tar at outPath. Walk order is
// lexical (filepath.WalkDir); no shell out.
func tarDirectory(dir, outPath string) error {
	out, err := os.OpenFile(outPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0750)
	if err != nil {
		return fmt.Errorf("unable to create archive %s: %w", outPath, err)
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)

	writeErr := func() error {
		return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			info, err := d.Info()
			if err != nil {
				return fmt.Errorf("unable to stat %s: %w", path, err)
			}

			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return fmt.Errorf("unable to relativize %s: %w", path, err)
			}

			hdr, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return fmt.Errorf("unable to build tar header for %s: %w", path, err)
			}
			hdr.Name = rel

			if err := tw.WriteHeader(hdr); err != nil {
				return fmt.Errorf("unable to write tar header for %s: %w", path, err)
			}

			if !info.Mode().IsRegular() {
				return nil
			}

			f, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("unable to open %s: %w", path, err)
			}
			_, copyErr := io.Copy(tw, f)
			closeErr := f.Close()
			if copyErr != nil {
				return fmt.Errorf("unable to archive %s: %w", path, copyErr)
			}
			return closeErr
		})
	}()

	if writeErr != nil {
		return writeErr
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("unable to finalize tar: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("unable to finalize gzip: %w", err)
	}
	return nil
}

// Cleanup removes the backup file.
func (p *BasebackupProvider) Cleanup(ctx context.Context, filePath string) error {
	traceID := traceid.FromContext(ctx)

	p.logger.Info(traceID, "Starting cleanup", nil,
		log.String("path", filePath),
		log.String("component", "basebackup-provider"),
	)

	if err := os.Remove(filePath); err != nil {
		p.logger.Error(traceID, "Failed to cleanup backup file", nil,
			log.Error(err),
			log.String("path", filePath),
			log.String("component", "basebackup-provider"),
		)
		return fmt.Errorf("unable to cleanup backup file: %w", err)
	}

	p.logger.Info(traceID, "Cleanup completed", nil,
		log.String("path", filePath),
		log.String("component", "basebackup-provider"),
	)
	return nil
}

// ValidateDependencies checks pg_basebackup and pg_verifybackup in PATH.
func (p *BasebackupProvider) ValidateDependencies() error {
	if _, err := exec.LookPath("pg_basebackup"); err != nil {
		return fmt.Errorf("pg_basebackup not found in PATH: please install PostgreSQL client tools (postgresql-client on Debian/Ubuntu, libpq/homebrew postgresql on macOS)")
	}
	if _, err := exec.LookPath("pg_verifybackup"); err != nil {
		return fmt.Errorf("pg_verifybackup not found in PATH: requires PostgreSQL client tools >= 13 (postgresql-client on Debian/Ubuntu, libpq/homebrew postgresql on macOS)")
	}
	return nil
}
