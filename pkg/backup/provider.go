package backup

import (
	"context"
	"fmt"

	"github.com/glennprays/dbeasebackup/config"
	"github.com/glennprays/log"
)

// Provider defines the interface for database backup providers
type Provider interface {
	// Name returns the provider name for logging and identification
	Name() string

	// Dump creates a backup dump and returns the file path
	Dump(ctx context.Context, backupDir string) (filePath string, err error)

	// Cleanup removes temporary files after backup
	Cleanup(ctx context.Context, filePath string) error

	// ValidateDependencies checks if required external tools are available
	ValidateDependencies() error
}

// NewProviderFor creates the provider matching the target's mode.
// dump -> pg_dump custom-format provider; basebackup -> pg_basebackup provider.
func NewProviderFor(t *config.Target, logger *log.Logger) (Provider, error) {
	timeout, err := t.TimeoutDuration()
	if err != nil {
		return nil, fmt.Errorf("target %q: %w", t.Name, err)
	}

	switch t.Mode {
	case "dump":
		return NewPostgresProvider(PostgresConfig{
			Host:       t.Host,
			Port:       t.Port,
			User:       t.User,
			Password:   t.ResolvedPassword(),
			Database:   t.Database,
			TargetName: t.Name,
		}, logger, timeout), nil
	case "basebackup":
		return NewBasebackupProvider(BasebackupConfig{
			Host:       t.Host,
			Port:       t.Port,
			User:       t.User,
			Password:   t.ResolvedPassword(),
			TargetName: t.Name,
		}, logger, timeout), nil
	default:
		return nil, fmt.Errorf("unsupported backup mode %q for target %q (valid: dump, basebackup)", t.Mode, t.Name)
	}
}
