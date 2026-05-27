package config

import (
	"testing"
)

func TestConfig_Validate_Success(t *testing.T) {
	cfg := Config{
		ENV:                    "development",
		BACKUP_PROVIDER:        "postgres",
		STORAGE_TYPE:           "google-drive",
		PG_HOST:                "localhost",
		PG_PORT:                "5432",
		PG_USER:                "user",
		PG_PASSWORD:            "password",
		PG_DATABASE:            "testdb",
		PG_SSLMODE:             "disable",
		CRON_SCHEDULE:          "0 * * * *",
		GOOGLE_DRIVE_FOLDER_ID: "folder-id",
		LOG_LEVEL:              "info",
		LOG_FORMAT:             "text",
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() unexpected error = %v", err)
	}
}

func TestConfig_Validate_InvalidBackupProvider(t *testing.T) {
	cfg := Config{
		BACKUP_PROVIDER: "invalid",
		STORAGE_TYPE:    "google-drive",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() expected error for invalid backup provider")
	}
}

func TestConfig_Validate_InvalidStorageType(t *testing.T) {
	cfg := Config{
		BACKUP_PROVIDER: "postgres",
		STORAGE_TYPE:    "invalid",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() expected error for invalid storage type")
	}
}

func TestConfig_Validate_MissingPGFields(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "missing PG_HOST",
			config: Config{
				BACKUP_PROVIDER: "postgres",
				STORAGE_TYPE:    "google-drive",
				PG_PORT:         "5432",
				PG_USER:         "user",
				PG_PASSWORD:     "password",
				PG_DATABASE:     "testdb",
			},
		},
		{
			name: "missing PG_PORT",
			config: Config{
				BACKUP_PROVIDER: "postgres",
				STORAGE_TYPE:    "google-drive",
				PG_HOST:         "localhost",
				PG_USER:         "user",
				PG_PASSWORD:     "password",
				PG_DATABASE:     "testdb",
			},
		},
		{
			name: "missing PG_USER",
			config: Config{
				BACKUP_PROVIDER: "postgres",
				STORAGE_TYPE:    "google-drive",
				PG_HOST:         "localhost",
				PG_PORT:         "5432",
				PG_PASSWORD:     "password",
				PG_DATABASE:     "testdb",
			},
		},
		{
			name: "missing PG_PASSWORD",
			config: Config{
				BACKUP_PROVIDER: "postgres",
				STORAGE_TYPE:    "google-drive",
				PG_HOST:         "localhost",
				PG_PORT:         "5432",
				PG_USER:         "user",
				PG_DATABASE:     "testdb",
			},
		},
		{
			name: "missing PG_DATABASE",
			config: Config{
				BACKUP_PROVIDER: "postgres",
				STORAGE_TYPE:    "google-drive",
				PG_HOST:         "localhost",
				PG_PORT:         "5432",
				PG_USER:         "user",
				PG_PASSWORD:     "password",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err == nil {
				t.Errorf("Validate() expected error for %s", tt.name)
			}
		})
	}
}

func TestConfig_Validate_MissingCronSchedule(t *testing.T) {
	cfg := Config{
		BACKUP_PROVIDER:        "postgres",
		STORAGE_TYPE:           "google-drive",
		PG_HOST:                "localhost",
		PG_PORT:                "5432",
		PG_USER:                "user",
		PG_PASSWORD:            "password",
		PG_DATABASE:            "testdb",
		PG_SSLMODE:             "disable",
		CRON_SCHEDULE:          "",
		GOOGLE_DRIVE_FOLDER_ID: "folder-id",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() expected error for missing CRON_SCHEDULE")
	}
}

func TestConfig_Validate_MissingGoogleDriveFields(t *testing.T) {
	cfg := Config{
		BACKUP_PROVIDER:        "postgres",
		STORAGE_TYPE:           "google-drive",
		PG_HOST:                "localhost",
		PG_PORT:                "5432",
		PG_USER:                "user",
		PG_PASSWORD:            "password",
		PG_DATABASE:            "testdb",
		PG_SSLMODE:             "disable",
		CRON_SCHEDULE:          "0 * * * *",
		GOOGLE_DRIVE_FOLDER_ID: "",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() expected error for missing GOOGLE_DRIVE_FOLDER_ID")
	}
}

func TestConfig_Validate_MissingS3Fields(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "missing S3_BUCKET",
			config: Config{
				BACKUP_PROVIDER:      "postgres",
				STORAGE_TYPE:         "s3",
				PG_HOST:              "localhost",
				PG_PORT:              "5432",
				PG_USER:              "user",
				PG_PASSWORD:          "password",
				PG_DATABASE:          "testdb",
				PG_SSLMODE:           "disable",
				CRON_SCHEDULE:        "0 * * * *",
				S3_ACCESS_KEY_ID:     "key",
				S3_SECRET_ACCESS_KEY: "secret",
			},
		},
		{
			name: "missing S3_ACCESS_KEY_ID",
			config: Config{
				BACKUP_PROVIDER:      "postgres",
				STORAGE_TYPE:         "s3",
				PG_HOST:              "localhost",
				PG_PORT:              "5432",
				PG_USER:              "user",
				PG_PASSWORD:          "password",
				PG_DATABASE:          "testdb",
				PG_SSLMODE:           "disable",
				CRON_SCHEDULE:        "0 * * * *",
				S3_BUCKET:            "bucket",
				S3_SECRET_ACCESS_KEY: "secret",
			},
		},
		{
			name: "missing S3_SECRET_ACCESS_KEY",
			config: Config{
				BACKUP_PROVIDER:  "postgres",
				STORAGE_TYPE:     "s3",
				PG_HOST:          "localhost",
				PG_PORT:          "5432",
				PG_USER:          "user",
				PG_PASSWORD:      "password",
				PG_DATABASE:      "testdb",
				PG_SSLMODE:       "disable",
				CRON_SCHEDULE:    "0 * * * *",
				S3_BUCKET:        "bucket",
				S3_ACCESS_KEY_ID: "key",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err == nil {
				t.Errorf("Validate() expected error for %s", tt.name)
			}
		})
	}
}

func TestConfig_Validate_S3Success(t *testing.T) {
	cfg := Config{
		ENV:                  "development",
		BACKUP_PROVIDER:      "postgres",
		STORAGE_TYPE:         "s3",
		PG_HOST:              "localhost",
		PG_PORT:              "5432",
		PG_USER:              "user",
		PG_PASSWORD:          "password",
		PG_DATABASE:          "testdb",
		PG_SSLMODE:           "disable",
		CRON_SCHEDULE:        "0 * * * *",
		S3_BUCKET:            "bucket",
		S3_REGION:            "us-east-1",
		S3_ACCESS_KEY_ID:     "key",
		S3_SECRET_ACCESS_KEY: "secret",
		LOG_LEVEL:            "info",
		LOG_FORMAT:           "text",
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() unexpected error = %v", err)
	}
}

func TestConfig_Validate_InvalidLogLevel(t *testing.T) {
	cfg := Config{
		BACKUP_PROVIDER:        "postgres",
		STORAGE_TYPE:           "google-drive",
		PG_HOST:                "localhost",
		PG_PORT:                "5432",
		PG_USER:                "user",
		PG_PASSWORD:            "password",
		PG_DATABASE:            "testdb",
		PG_SSLMODE:             "disable",
		CRON_SCHEDULE:          "0 * * * *",
		GOOGLE_DRIVE_FOLDER_ID: "folder-id",
		LOG_LEVEL:              "invalid",
		LOG_FORMAT:             "text",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() expected error for invalid LOG_LEVEL")
	}
}

func TestConfig_Validate_InvalidLogFormat(t *testing.T) {
	cfg := Config{
		BACKUP_PROVIDER:        "postgres",
		STORAGE_TYPE:           "google-drive",
		PG_HOST:                "localhost",
		PG_PORT:                "5432",
		PG_USER:                "user",
		PG_PASSWORD:            "password",
		PG_DATABASE:            "testdb",
		PG_SSLMODE:             "disable",
		CRON_SCHEDULE:          "0 * * * *",
		GOOGLE_DRIVE_FOLDER_ID: "folder-id",
		LOG_LEVEL:              "info",
		LOG_FORMAT:             "invalid",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() expected error for invalid LOG_FORMAT")
	}
}

func TestConfig_IsProduction(t *testing.T) {
	tests := []struct {
		name     string
		env      string
		expected bool
	}{
		{
			name:     "production env",
			env:      "production",
			expected: true,
		},
		{
			name:     "development env",
			env:      "development",
			expected: false,
		},
		{
			name:     "empty env",
			env:      "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{ENV: tt.env}
			if got := cfg.IsProduction(); got != tt.expected {
				t.Errorf("IsProduction() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfig_IsDevelopment(t *testing.T) {
	tests := []struct {
		name     string
		env      string
		expected bool
	}{
		{
			name:     "development env",
			env:      "development",
			expected: true,
		},
		{
			name:     "production env",
			env:      "production",
			expected: false,
		},
		{
			name:     "empty env",
			env:      "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{ENV: tt.env}
			if got := cfg.IsDevelopment(); got != tt.expected {
				t.Errorf("IsDevelopment() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfig_Validate_AllLogLevels(t *testing.T) {
	validLevels := []string{"debug", "info", "warn", "error"}

	for _, level := range validLevels {
		t.Run("log_level_"+level, func(t *testing.T) {
			cfg := Config{
				BACKUP_PROVIDER:        "postgres",
				STORAGE_TYPE:           "google-drive",
				PG_HOST:                "localhost",
				PG_PORT:                "5432",
				PG_USER:                "user",
				PG_PASSWORD:            "password",
				PG_DATABASE:            "testdb",
				PG_SSLMODE:             "disable",
				CRON_SCHEDULE:          "0 * * * *",
				GOOGLE_DRIVE_FOLDER_ID: "folder-id",
				LOG_LEVEL:              level,
				LOG_FORMAT:             "text",
			}

			err := cfg.Validate()
			if err != nil {
				t.Errorf("Validate() unexpected error for LOG_LEVEL=%s: %v", level, err)
			}
		})
	}
}

func TestConfig_Validate_AllLogFormats(t *testing.T) {
	validFormats := []string{"json", "text"}

	for _, format := range validFormats {
		t.Run("log_format_"+format, func(t *testing.T) {
			cfg := Config{
				BACKUP_PROVIDER:        "postgres",
				STORAGE_TYPE:           "google-drive",
				PG_HOST:                "localhost",
				PG_PORT:                "5432",
				PG_USER:                "user",
				PG_PASSWORD:            "password",
				PG_DATABASE:            "testdb",
				PG_SSLMODE:             "disable",
				CRON_SCHEDULE:          "0 * * * *",
				GOOGLE_DRIVE_FOLDER_ID: "folder-id",
				LOG_LEVEL:              "info",
				LOG_FORMAT:             format,
			}

			err := cfg.Validate()
			if err != nil {
				t.Errorf("Validate() unexpected error for LOG_FORMAT=%s: %v", format, err)
			}
		})
	}
}
