// Package config centralizes application configuration: the flat global
// Config, the per-target Target model, the targets-file loader, and the
// legacy environment-variable fallback.
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
)

// StorageOverrides holds per-target storage placement overrides.
type StorageOverrides struct {
	// FolderID is the Google Drive folder for this target's backups.
	FolderID string `mapstructure:"folder_id"`
	// Prefix is an extra S3 key segment for this target's backups.
	Prefix string `mapstructure:"prefix"`
}

// Target describes one backup target: its own connection, schedule,
// retention, mode, and storage placement.
type Target struct {
	Name          string            `mapstructure:"name"`
	Mode          string            `mapstructure:"mode"` // "dump" | "basebackup"; default "dump"
	Host          string            `mapstructure:"host"`
	Port          string            `mapstructure:"port"`      // default "5432"
	User          string            `mapstructure:"user"`
	Password      string            `mapstructure:"password"`     // literal
	PasswordEnv   string            `mapstructure:"password_env"` // env var name; wins over password
	Database      string            `mapstructure:"database"`     // required iff mode=dump
	SSLMode       string            `mapstructure:"sslmode"`      // default "disable"
	Schedule      string            `mapstructure:"schedule"`     // cron, required
	Timeout       string            `mapstructure:"timeout"`      // Go duration, default "30m"
	RetentionDays *int              `mapstructure:"retention_days"` // default 30; 0 = keep forever
	Verify        *bool             `mapstructure:"verify"`         // default true (nil = true)
	Retries       *int              `mapstructure:"retries"`        // default 2; explicit 0 = no retry; negatives clamp to 0
	BackupDir     string            `mapstructure:"backup_dir"`     // default "backups/<name>"
	Storage       StorageOverrides `mapstructure:"storage"`
}

var (
	targetNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	validModes   = map[string]bool{
		"dump":       true,
		"basebackup": true,
	}
	validSSLModes = map[string]bool{
		"disable":    true,
		"require":    true,
		"verify-ca":  true,
		"verify-full": true,
	}
)

// ResolvedPassword returns the password from PasswordEnv (winning when the
// env var is set and non-empty), falling back to the literal Password.
func (t Target) ResolvedPassword() string {
	if t.PasswordEnv != "" {
		if v := os.Getenv(t.PasswordEnv); v != "" {
			return v
		}
	}
	return t.Password
}

// TimeoutDuration parses Target.Timeout into a time.Duration.
// Empty means the 30m default.
func (t Target) TimeoutDuration() (time.Duration, error) {
	if t.Timeout == "" {
		return 30 * time.Minute, nil
	}
	d, err := time.ParseDuration(t.Timeout)
	if err != nil {
		return 0, fmt.Errorf("target %q: invalid timeout %q: %w", t.Name, t.Timeout, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("target %q: timeout must be positive, got %s", t.Name, t.Timeout)
	}
	return d, nil
}

// ShouldVerify reports whether backup integrity verification is enabled
// for this target (nil Verify means enabled).
func (t Target) ShouldVerify() bool {
	return t.Verify == nil || *t.Verify
}

// RetryCount returns the number of retries after the first attempt.
// nil means the default of 2; negatives clamp to 0.
func (t Target) RetryCount() int {
	if t.Retries == nil {
		return 2
	}
	if *t.Retries < 0 {
		return 0
	}
	return *t.Retries
}

// RetentionCount returns the retention window in days.
// nil means the default of 30; 0 (or negative) = keep forever.
func (t Target) RetentionCount() int {
	if t.RetentionDays == nil {
		return 30
	}
	if *t.RetentionDays < 0 {
		return 0
	}
	return *t.RetentionDays
}

// Normalize fills defaults for unset fields (mode, port, sslmode,
// backup_dir). Loaders apply it automatically; direct constructors of
// Target (tests, callers) must call it before use.
func (t *Target) Normalize() {
	if t.Mode == "" {
		t.Mode = "dump"
	}
	if t.Port == "" {
		t.Port = "5432"
	}
	if t.SSLMode == "" {
		t.SSLMode = "disable"
	}
	if t.BackupDir == "" {
		t.BackupDir = "backups/" + t.Name
	}
}

// Validate checks one target's required fields and value ranges.
func (t *Target) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("target %q: name is required", t.Name)
	}
	if !targetNameRe.MatchString(t.Name) {
		return fmt.Errorf("target %q: name must match ^[a-z0-9][a-z0-9-]{0,62}$", t.Name)
	}
	if t.Schedule == "" {
		return fmt.Errorf("target %q: schedule is required", t.Name)
	}
	if _, err := cron.ParseStandard(t.Schedule); err != nil {
		return fmt.Errorf("target %q: invalid schedule %q: %v", t.Name, t.Schedule, err)
	}
	if !validModes[t.Mode] {
		return fmt.Errorf("target %q: invalid mode %q (valid: dump, basebackup)", t.Name, t.Mode)
	}
	switch t.Mode {
	case "dump":
		if t.Database == "" {
			return fmt.Errorf("target %q: database is required when mode=dump", t.Name)
		}
	case "basebackup":
		if t.Database != "" {
			return fmt.Errorf("target %q: database is not valid for mode=basebackup (pg_basebackup is cluster-wide)", t.Name)
		}
	}
	if t.Host == "" {
		return fmt.Errorf("target %q: host is required", t.Name)
	}
	if t.User == "" {
		return fmt.Errorf("target %q: user is required", t.Name)
	}
	if !validSSLModes[t.SSLMode] {
		return fmt.Errorf("target %q: invalid sslmode %q (valid: disable, require, verify-ca, verify-full)", t.Name, t.SSLMode)
	}
	if t.RetentionDays != nil && *t.RetentionDays < 0 {
		return fmt.Errorf("target %q: retention_days must be >= 0, got %d", t.Name, *t.RetentionDays)
	}
	if _, err := t.TimeoutDuration(); err != nil {
		return err
	}
	return nil
}

// ValidateTargets validates every target and enforces unique names.
func ValidateTargets(targets []Target) error {
	seen := make(map[string]bool, len(targets))
	for i := range targets {
		t := &targets[i]
		if seen[t.Name] {
			return fmt.Errorf("duplicate target name %q", t.Name)
		}
		seen[t.Name] = true
		if err := t.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// targetsFile is the on-disk document shape.
type targetsFile struct {
	Targets []Target `mapstructure:"targets"`
}

// LoadTargets reads and validates a targets file (YAML or JSON, chosen by
// extension via viper).
func LoadTargets(path string) ([]Target, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("unable to read targets file %s: %w", path, err)
	}
	return parseTargets(v, path)
}

// LoadTargetsFromContent parses and validates targets from an in-memory
// YAML document. Test helper.
func LoadTargetsFromContent(content string) ([]Target, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(content)); err != nil {
		return nil, fmt.Errorf("unable to parse targets content: %w", err)
	}
	return parseTargets(v, "<memory>")
}

func parseTargets(v *viper.Viper, source string) ([]Target, error) {
	var file targetsFile
	if err := v.Unmarshal(&file); err != nil {
		return nil, fmt.Errorf("unable to parse targets file %s: %w", source, err)
	}
	if len(file.Targets) == 0 {
		return nil, fmt.Errorf("targets file %s contains no targets", source)
	}

	for i := range file.Targets {
		file.Targets[i].Normalize()
	}
	if err := ValidateTargets(file.Targets); err != nil {
		return nil, err
	}
	return file.Targets, nil
}

// targetsFromEnv synthesizes a single "default" target from the legacy
// flat PG_*/backup env vars.
func targetsFromEnv(cfg *Config) ([]Target, error) {
	if cfg.PG_HOST == "" {
		return nil, fmt.Errorf("no targets configured: provide TARGETS_FILE or legacy PG_* env vars")
	}
	if cfg.CRON_SCHEDULE == "" {
		return nil, fmt.Errorf("legacy configuration incomplete: CRON_SCHEDULE is required when using legacy PG_* env vars")
	}
	if cfg.PG_USER == "" {
		return nil, fmt.Errorf("legacy configuration incomplete: PG_USER is required when using legacy PG_* env vars")
	}

	verify := cfg.BACKUP_VERIFY
	retention := cfg.BACKUP_RETENTION_DAYS
	t := Target{
		Name:          "default",
		Mode:          "dump",
		Host:          cfg.PG_HOST,
		Port:          cfg.PG_PORT,
		User:          cfg.PG_USER,
		Password:      cfg.PG_PASSWORD,
		Database:      cfg.PG_DATABASE,
		SSLMode:       cfg.PG_SSLMODE,
		Schedule:      cfg.CRON_SCHEDULE,
		Timeout:       cfg.BACKUP_TIMEOUT,
		RetentionDays: &retention,
		Verify:        &verify,
		BackupDir:     cfg.BACKUP_DIR,
		Storage: StorageOverrides{
			FolderID: cfg.GOOGLE_DRIVE_FOLDER_ID,
			Prefix:   cfg.S3_PREFIX,
		},
	}
	t.Normalize()
	if err := t.Validate(); err != nil {
		return nil, err
	}
	return []Target{t}, nil
}

// LoadTargetsOrDefault loads targets from TARGETS_FILE when set, otherwise
// synthesizes one target from legacy env vars. The bool result reports
// whether targets came from the legacy env fallback.
func LoadTargetsOrDefault(cfg *Config) ([]Target, bool, error) {
	if cfg.TARGETS_FILE != "" {
		targets, err := LoadTargets(cfg.TARGETS_FILE)
		return targets, false, err
	}
	targets, err := targetsFromEnv(cfg)
	return targets, err == nil, err
}
