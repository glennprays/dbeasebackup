package config

import (
	"testing"
)

// baseValidConfig returns a Config that passes Validate() after the
// multi-target refactor: global concerns only, no PG_*/cron/drive checks.
func baseValidConfig() Config {
	return Config{
		ENV:          "development",
		STORAGE_TYPE: "google-drive",
		LOG_LEVEL:    "info",
		LOG_FORMAT:   "text",
	}
}

func TestConfig_Validate_Success(t *testing.T) {
	cfg := baseValidConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() unexpected error = %v", err)
	}
}

func TestConfig_Validate_NoPGFieldsRequired(t *testing.T) {
	// PG_*/CRON_SCHEDULE/GOOGLE_DRIVE_FOLDER_ID are only needed through the
	// legacy fallback path; global Validate must not require them.
	// STORAGE_TYPE="" is treated as unset (resolved at storage init).
	cfg := Config{LOG_LEVEL: "info", LOG_FORMAT: "text"}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() on config without storage/targets unexpected error = %v", err)
	}
}

func TestConfig_Validate_InvalidStorageType(t *testing.T) {
	cfg := baseValidConfig()
	cfg.STORAGE_TYPE = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error for invalid storage type")
	}
}

func TestConfig_Validate_InvalidLogLevel(t *testing.T) {
	cfg := baseValidConfig()
	cfg.LOG_LEVEL = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error for invalid LOG_LEVEL")
	}
}

func TestConfig_Validate_InvalidLogFormat(t *testing.T) {
	cfg := baseValidConfig()
	cfg.LOG_FORMAT = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error for invalid LOG_FORMAT")
	}
}

func TestConfig_Validate_NegativeMaxParallel(t *testing.T) {
	cfg := baseValidConfig()
	cfg.MAX_PARALLEL_BACKUPS = -1
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error for negative MAX_PARALLEL_BACKUPS")
	}
}

func TestConfig_Validate_ValidStorageTypes(t *testing.T) {
	for _, st := range []string{"google-drive", "s3"} {
		cfg := baseValidConfig()
		cfg.STORAGE_TYPE = st
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() unexpected error for STORAGE_TYPE=%s: %v", st, err)
		}
	}
}

func TestConfig_Validate_AllLogLevels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		cfg := baseValidConfig()
		cfg.LOG_LEVEL = level
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() unexpected error for LOG_LEVEL=%s: %v", level, err)
		}
	}
}

func TestConfig_Validate_AllLogFormats(t *testing.T) {
	for _, format := range []string{"json", "text"} {
		cfg := baseValidConfig()
		cfg.LOG_FORMAT = format
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() unexpected error for LOG_FORMAT=%s: %v", format, err)
		}
	}
}

func TestConfig_IsProduction(t *testing.T) {
	tests := []struct {
		name     string
		env      string
		expected bool
	}{
		{"production env", "production", true},
		{"development env", "development", false},
		{"empty env", "", false},
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
		{"development env", "development", true},
		{"production env", "production", false},
		{"empty env", "", true},
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
