package backup

import "context"

// Provider defines the interface for database backup providers
type Provider interface {
	// Name returns the provider name for logging and identification
	Name() string

	// Dump creates a backup dump and returns the file path
	Dump(ctx context.Context, backupDir string) (filePath string, err error)

	// Cleanup removes temporary files after backup
	Cleanup(filePath string) error
}
