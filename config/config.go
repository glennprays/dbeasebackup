package config

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/creasty/defaults"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config holds all configuration for the application with flat structure
type Config struct {
	// App
	ENV string `mapstructure:"ENV" default:"production"`

	// Backup Provider
	BACKUP_PROVIDER string `mapstructure:"BACKUP_PROVIDER" default:"postgres"`

	// Storage Type
	STORAGE_TYPE string `mapstructure:"STORAGE_TYPE" default:"google-drive"`

	// Database (PostgreSQL)
	PG_HOST     string `mapstructure:"PG_HOST" default:""`
	PG_PORT     string `mapstructure:"PG_PORT" default:"5432"`
	PG_USER     string `mapstructure:"PG_USER" default:""`
	PG_PASSWORD string `mapstructure:"PG_PASSWORD" default:""`
	PG_DATABASE string `mapstructure:"PG_DATABASE" default:""`

	// Storage
	BACKUP_DIR     string `mapstructure:"BACKUP_DIR" default:"backups/postgres"`
	BACKUP_TIMEOUT string `mapstructure:"BACKUP_TIMEOUT" default:"30m"`

	// Scheduler
	CRON_SCHEDULE      string `mapstructure:"CRON_SCHEDULE" default:""`
	SCHEDULER_TIMEZONE string `mapstructure:"SCHEDULER_TIMEZONE" default:"UTC"`

	// Logging
	LOG_LEVEL  string `mapstructure:"LOG_LEVEL" default:"info"`
	LOG_FORMAT string `mapstructure:"LOG_FORMAT" default:"text"`

	// Google Drive
	GOOGLE_DRIVE_FOLDER_ID string `mapstructure:"GOOGLE_DRIVE_FOLDER_ID" default:""`
	GOOGLE_DRIVE_KEY_FILE  string `mapstructure:"GOOGLE_DRIVE_KEY_FILE" default:"service-account-key.json"`

	// S3 Storage
	S3_BUCKET            string `mapstructure:"S3_BUCKET" default:""`
	S3_REGION            string `mapstructure:"S3_REGION" default:"us-east-1"`
	S3_ACCESS_KEY_ID     string `mapstructure:"S3_ACCESS_KEY_ID" default:""`
	S3_SECRET_ACCESS_KEY string `mapstructure:"S3_SECRET_ACCESS_KEY" default:""`
	S3_ENDPOINT          string `mapstructure:"S3_ENDPOINT" default:""`
}

// Environment represents the application environment
type Environment string

const (
	LOCAL Environment = "local"
	DEV   Environment = "development"
	PROD  Environment = "production"
)

// ProvideConfig loads configuration from environment variables using viper
func ProvideConfig() (*Config, error) {
	cfg := Config{}
	if err := defaults.Set(&cfg); err != nil {
		log.Fatalf("Error setting default values: %v", err)
	}

	// Detect environment
	envStr := strings.ToLower(os.Getenv("ENV"))
	env := Environment(envStr)
	if env == "" {
		env = LOCAL
	}

	// Local/Development -> load .env file first
	if env == LOCAL || env == DEV {
		_ = godotenv.Load(".env")
	}

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Auto-bind each struct field by key
	t := reflect.TypeOf(cfg)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		key := field.Tag.Get("mapstructure")
		if key != "" {
			_ = viper.BindEnv(key)
		}
	}

	// Fill struct
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks that all required configuration values are set
func (c *Config) Validate() error {
	// Validate backup provider
	validBackupProviders := map[string]bool{
		"postgres": true,
	}
	if !validBackupProviders[c.BACKUP_PROVIDER] {
		return fmt.Errorf("invalid BACKUP_PROVIDER: %s (valid: postgres)", c.BACKUP_PROVIDER)
	}

	// Validate storage type
	validStorageTypes := map[string]bool{
		"google-drive": true,
		"s3":           true,
	}
	if !validStorageTypes[c.STORAGE_TYPE] {
		return fmt.Errorf("invalid STORAGE_TYPE: %s (valid: google-drive, s3)", c.STORAGE_TYPE)
	}

	// PostgreSQL provider validation
	if c.BACKUP_PROVIDER == "postgres" {
		if c.PG_HOST == "" {
			return fmt.Errorf("PG_HOST environment variable is required")
		}
		if c.PG_PORT == "" {
			return fmt.Errorf("PG_PORT environment variable is required")
		}
		if c.PG_USER == "" {
			return fmt.Errorf("PG_USER environment variable is required")
		}
		if c.PG_PASSWORD == "" {
			return fmt.Errorf("PG_PASSWORD environment variable is required")
		}
		if c.PG_DATABASE == "" {
			return fmt.Errorf("PG_DATABASE environment variable is required")
		}
	}

	if c.CRON_SCHEDULE == "" {
		return fmt.Errorf("CRON_SCHEDULE environment variable is required")
	}

	// Storage type specific validation
	if c.STORAGE_TYPE == "google-drive" {
		if c.GOOGLE_DRIVE_FOLDER_ID == "" {
			return fmt.Errorf("GOOGLE_DRIVE_FOLDER_ID environment variable is required when STORAGE_TYPE=google-drive")
		}
	}

	if c.STORAGE_TYPE == "s3" {
		if c.S3_BUCKET == "" {
			return fmt.Errorf("S3_BUCKET environment variable is required when STORAGE_TYPE=s3")
		}
		if c.S3_ACCESS_KEY_ID == "" {
			return fmt.Errorf("S3_ACCESS_KEY_ID environment variable is required when STORAGE_TYPE=s3")
		}
		if c.S3_SECRET_ACCESS_KEY == "" {
			return fmt.Errorf("S3_SECRET_ACCESS_KEY environment variable is required when STORAGE_TYPE=s3")
		}
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.LOG_LEVEL] {
		return fmt.Errorf("invalid LOG_LEVEL: %s (valid: debug, info, warn, error)", c.LOG_LEVEL)
	}

	// Validate log format
	validLogFormats := map[string]bool{
		"json": true,
		"text": true,
	}
	if !validLogFormats[c.LOG_FORMAT] {
		return fmt.Errorf("invalid LOG_FORMAT: %s (valid: json, text)", c.LOG_FORMAT)
	}

	return nil
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.ENV == "production"
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return !c.IsProduction()
}

// GetBackupTimeout parses the BACKUP_TIMEOUT string into a time.Duration
func (c *Config) GetBackupTimeout() time.Duration {
	d, err := time.ParseDuration(c.BACKUP_TIMEOUT)
	if err != nil {
		return 30 * time.Minute // fallback to default
	}
	return d
}
