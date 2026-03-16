package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the application
type Config struct {
	GoEnv       string
	Database    DatabaseConfig
	Storage     StorageConfig
	Scheduler   SchedulerConfig
	Logging     LoggingConfig
	GoogleDrive GoogleDriveConfig
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

// StorageConfig holds storage provider settings
type StorageConfig struct {
	BackupDir string
}

// SchedulerConfig holds scheduler settings
type SchedulerConfig struct {
	Schedule string
	Timezone string
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level  string
	Format string
}

// GoogleDriveConfig holds Google Drive specific settings
type GoogleDriveConfig struct {
	FolderID    string
	KeyFilePath string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		GoEnv: getEnv("GO_ENV", "development"),
		Database: DatabaseConfig{
			Host:     os.Getenv("PG_HOST"),
			Port:     os.Getenv("PG_PORT"),
			User:     os.Getenv("PG_USER"),
			Password: os.Getenv("PG_PASSWORD"),
			Database: os.Getenv("PG_DATABASE"),
		},
		Storage: StorageConfig{
			BackupDir: getEnv("BACKUP_DIR", "backups/postgres"),
		},
		Scheduler: SchedulerConfig{
			Schedule: os.Getenv("CRON_SCHEDULE"),
			Timezone: getEnv("SCHEDULER_TIMEZONE", "UTC"),
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "text"),
		},
		GoogleDrive: GoogleDriveConfig{
			FolderID:    os.Getenv("GOOGLE_DRIVE_FOLDER_ID"),
			KeyFilePath: getEnv("GOOGLE_DRIVE_KEY_FILE", "service-account-key.json"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that all required configuration values are set
func (c *Config) Validate() error {
	if c.Database.Host == "" {
		return fmt.Errorf("PG_HOST environment variable is required")
	}
	if c.Database.Port == "" {
		return fmt.Errorf("PG_PORT environment variable is required")
	}
	if c.Database.User == "" {
		return fmt.Errorf("PG_USER environment variable is required")
	}
	if c.Database.Password == "" {
		return fmt.Errorf("PG_PASSWORD environment variable is required")
	}
	if c.Database.Database == "" {
		return fmt.Errorf("PG_DATABASE environment variable is required")
	}
	if c.Scheduler.Schedule == "" {
		return fmt.Errorf("CRON_SCHEDULE environment variable is required")
	}
	if c.GoogleDrive.FolderID == "" {
		return fmt.Errorf("GOOGLE_DRIVE_FOLDER_ID environment variable is required")
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.Logging.Level] {
		return fmt.Errorf("invalid LOG_LEVEL: %s (valid: debug, info, warn, error)", c.Logging.Level)
	}

	// Validate log format
	validLogFormats := map[string]bool{
		"json": true,
		"text": true,
	}
	if !validLogFormats[c.Logging.Format] {
		return fmt.Errorf("invalid LOG_FORMAT: %s (valid: json, text)", c.Logging.Format)
	}

	return nil
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.GoEnv != "production"
}

// getEnv retrieves an environment variable or returns the default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt retrieves an environment variable as an integer or returns the default value
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
