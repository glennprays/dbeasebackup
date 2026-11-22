package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTargetsFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "targets.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write targets file: %v", err)
	}
	return path
}

func TestLoadTargets_ValidYAML(t *testing.T) {
	path := writeTargetsFile(t, `
targets:
  - name: app-db
    mode: dump
    host: localhost
    port: 5432
    user: postgres
    password: secret
    database: appdb
    schedule: "0 2 * * *"
    retention_days: 7
    timeout: 45m
    storage:
      folder_id: folder-123
      prefix: app/
  - name: cluster
    mode: basebackup
    host: db.internal
    user: repl
    password_env: REPL_PASSWORD
    schedule: "0 4 * * 0"
    verify: false
`)
	targets, err := LoadTargets(path)
	if err != nil {
		t.Fatalf("LoadTargets() unexpected error = %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("LoadTargets() got %d targets, want 2", len(targets))
	}

	app := targets[0]
	if app.Name != "app-db" || app.Mode != "dump" || app.Host != "localhost" ||
		app.Port != "5432" || app.User != "postgres" || app.Password != "secret" ||
		app.Database != "appdb" || app.Schedule != "0 2 * * *" ||
		app.RetentionDays == nil || *app.RetentionDays != 7 || app.Timeout != "45m" ||
		app.Storage.FolderID != "folder-123" || app.Storage.Prefix != "app/" {
		t.Errorf("Dump target fields not parsed correctly: %+v", app)
	}
	if app.BackupDir != "backups/app-db" {
		t.Errorf("BackupDir default = %q, want backups/app-db", app.BackupDir)
	}
	if app.SSLMode != "disable" {
		t.Errorf("SSLMode default = %q, want disable", app.SSLMode)
	}

	cluster := targets[1]
	if cluster.Mode != "basebackup" || cluster.Database != "" {
		t.Errorf("Basebackup target fields not parsed correctly: %+v", cluster)
	}
	if cluster.ShouldVerify() {
		t.Error("ShouldVerify() = true, want false (verify: false set)")
	}

	// Defaults
	dumpVerify := app.ShouldVerify()
	if !dumpVerify {
		t.Error("ShouldVerify() default = false, want true")
	}
	if got := app.RetryCount(); got != 2 {
		t.Errorf("RetryCount() default = %d, want 2", got)
	}
	if d, err := app.TimeoutDuration(); err != nil || d != 45*time.Minute {
		t.Errorf("TimeoutDuration() = %v, %v; want 45m, nil", d, err)
	}
}

func TestLoadTargets_DuplicateNames(t *testing.T) {
	path := writeTargetsFile(t, `
targets:
  - name: same
    host: h1
    user: u
    database: d
    schedule: "* * * * *"
  - name: same
    host: h2
    user: u
    database: d
    schedule: "* * * * *"
`)
	_, err := LoadTargets(path)
	if err == nil {
		t.Error("LoadTargets() expected error for duplicate names")
	}
}

func TestLoadTargets_BadCron(t *testing.T) {
	path := writeTargetsFile(t, `
targets:
  - name: a
    host: h
    user: u
    database: d
    schedule: "not-a-cron"
`)
	_, err := LoadTargets(path)
	if err == nil {
		t.Error("LoadTargets() expected error for invalid schedule")
	}
}

func TestLoadTargets_DatabaseRequiredIffDump(t *testing.T) {
	dumpWithoutDB := `
targets:
  - name: a
    mode: dump
    host: h
    user: u
    schedule: "* * * * *"
`
	_, err := LoadTargetsFromContent(dumpWithoutDB)
	if err == nil {
		t.Error("expected error: dump target without database")
	}

	baseWithDB := `
targets:
  - name: b
    mode: basebackup
    host: h
    user: u
    database: oops
    schedule: "* * * * *"
`
	_, err = LoadTargetsFromContent(baseWithDB)
	if err == nil {
		t.Error("expected error: basebackup target with database")
	}

	baseOK := `
targets:
  - name: c
    mode: basebackup
    host: h
    user: u
    schedule: "* * * * *"
`
	targets, err := LoadTargetsFromContent(baseOK)
	if err != nil {
		t.Errorf("LoadTargetsFromContent() unexpected error = %v", err)
	}
	if targets[0].Mode != "basebackup" {
		t.Errorf("Mode = %q, want basebackup", targets[0].Mode)
	}
}

func TestLoadTargets_InvalidName(t *testing.T) {
	path := writeTargetsFile(t, `
targets:
  - name: "Bad Name!"
    host: h
    user: u
    database: d
    schedule: "* * * * *"
`)
	_, err := LoadTargets(path)
	if err == nil {
		t.Error("LoadTargets() expected error for invalid name")
	}
}

func TestLoadTargets_MissingSchedule(t *testing.T) {
	path := writeTargetsFile(t, `
targets:
  - name: a
    host: h
    user: u
    database: d
`)
	_, err := LoadTargets(path)
	if err == nil {
		t.Error("LoadTargets() expected error for missing schedule")
	}
}

func TestLoadTargets_EmptyTargets(t *testing.T) {
	path := writeTargetsFile(t, `
targets: []
`)
	_, err := LoadTargets(path)
	if err == nil {
		t.Error("LoadTargets() expected error for empty targets list")
	}
}

func TestTarget_ResolvedPassword(t *testing.T) {
	t.Setenv("TEST_PW", "from-env")

	// password_env wins
	tgt := Target{Name: "a", Password: "literal", PasswordEnv: "TEST_PW"}
	if got := tgt.ResolvedPassword(); got != "from-env" {
		t.Errorf("ResolvedPassword() = %q, want from-env", got)
	}

	// env unset falls back to literal
	tgt = Target{Name: "a", Password: "literal", PasswordEnv: "TEST_PW_UNSET"}
	if got := tgt.ResolvedPassword(); got != "literal" {
		t.Errorf("ResolvedPassword() = %q, want literal", got)
	}

	// literal only
	tgt = Target{Name: "a", Password: "literal"}
	if got := tgt.ResolvedPassword(); got != "literal" {
		t.Errorf("ResolvedPassword() = %q, want literal", got)
	}
}

func TestTarget_RetryCount(t *testing.T) {
	zero := 0
	five := 5
	neg := -3
	if got := (Target{Retries: nil}).RetryCount(); got != 2 {
		t.Errorf("RetryCount() nil = %d, want 2", got)
	}
	if got := (Target{Retries: &zero}).RetryCount(); got != 0 {
		t.Errorf("RetryCount() 0 = %d, want 0", got)
	}
	if got := (Target{Retries: &five}).RetryCount(); got != 5 {
		t.Errorf("RetryCount() 5 = %d, want 5", got)
	}
	if got := (Target{Retries: &neg}).RetryCount(); got != 0 {
		t.Errorf("RetryCount() -3 = %d, want 0", got)
	}
}

func TestTarget_TimeoutDuration(t *testing.T) {
	if d, err := (Target{Name: "a"}).TimeoutDuration(); err != nil || d != 30*time.Minute {
		t.Errorf("TimeoutDuration() empty = %v, %v; want 30m, nil", d, err)
	}
	if d, err := (Target{Name: "a", Timeout: "1h15m"}).TimeoutDuration(); err != nil || d != 75*time.Minute {
		t.Errorf("TimeoutDuration() 1h15m = %v, %v; want 75m, nil", d, err)
	}
	if _, err := (Target{Name: "a", Timeout: "bogus"}).TimeoutDuration(); err == nil {
		t.Error("TimeoutDuration() expected error for bogus duration")
	}
	if _, err := (Target{Name: "a", Timeout: "-5m"}).TimeoutDuration(); err == nil {
		t.Error("TimeoutDuration() expected error for negative duration")
	}
}

func makeLegacyCfg() *Config {
	return &Config{
		ENV:                    "development",
		STORAGE_TYPE:           "s3",
		PG_HOST:                "db.local",
		PG_PORT:                "5433",
		PG_USER:                "legacy",
		PG_PASSWORD:            "pw",
		PG_DATABASE:            "legacydb",
		PG_SSLMODE:             "require",
		CRON_SCHEDULE:          "0 */6 * * *",
		BACKUP_TIMEOUT:         "1h",
		BACKUP_RETENTION_DAYS:  14,
		BACKUP_VERIFY:          false,
		BACKUP_DIR:             "backups/legacy",
		GOOGLE_DRIVE_FOLDER_ID: "gfolder",
		S3_PREFIX:              "legacy/prefix",
		LOG_LEVEL:              "info",
		LOG_FORMAT:             "text",
	}
}

func TestTargetsFromEnv_LegacySynthesis(t *testing.T) {
	cfg := makeLegacyCfg()
	targets, err := targetsFromEnv(cfg)
	if err != nil {
		t.Fatalf("targetsFromEnv() unexpected error = %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("targetsFromEnv() got %d targets, want 1", len(targets))
	}
	tgt := targets[0]
	if tgt.Name != "default" || tgt.Host != "db.local" || tgt.Port != "5433" ||
		tgt.User != "legacy" || tgt.Password != "pw" || tgt.Database != "legacydb" ||
		tgt.SSLMode != "require" || tgt.Schedule != "0 */6 * * *" ||
		tgt.Timeout != "1h" || tgt.RetentionDays == nil || *tgt.RetentionDays != 14 || tgt.BackupDir != "backups/legacy" {
		t.Errorf("Legacy target fields wrong: %+v", tgt)
	}
	if tgt.ShouldVerify() {
		t.Error("ShouldVerify() = true, want false (BACKUP_VERIFY=false)")
	}
	if tgt.Storage.FolderID != "gfolder" || tgt.Storage.Prefix != "legacy/prefix" {
		t.Errorf("Storage overrides wrong: %+v", tgt.Storage)
	}
}

func TestTargetsFromEnv_BothAbsentError(t *testing.T) {
	_, err := targetsFromEnv(&Config{})
	want := "no targets configured: provide TARGETS_FILE or legacy PG_* env vars"
	if err == nil || err.Error() != want {
		t.Errorf("targetsFromEnv() error = %v, want %q", err, want)
	}
}

func TestLoadTargetsOrDefault_FileWins(t *testing.T) {
	path := writeTargetsFile(t, `
targets:
  - name: from-file
    host: h
    user: u
    database: d
    schedule: "* * * * *"
`)
	cfg := makeLegacyCfg()
	cfg.TARGETS_FILE = path

	targets, fromLegacy, err := LoadTargetsOrDefault(cfg)
	if err != nil {
		t.Fatalf("LoadTargetsOrDefault() unexpected error = %v", err)
	}
	if fromLegacy {
		t.Error("fromLegacy = true, want false when TARGETS_FILE set")
	}
	if len(targets) != 1 || targets[0].Name != "from-file" {
		t.Errorf("targets = %+v, want single from-file target", targets)
	}
}

func TestLoadTargetsOrDefault_LegacyFallback(t *testing.T) {
	targets, fromLegacy, err := LoadTargetsOrDefault(makeLegacyCfg())
	if err != nil {
		t.Fatalf("LoadTargetsOrDefault() unexpected error = %v", err)
	}
	if !fromLegacy {
		t.Error("fromLegacy = false, want true for env fallback")
	}
	if len(targets) != 1 || targets[0].Name != "default" {
		t.Errorf("targets = %+v, want single default target", targets)
	}
}

func TestLoadTargetsOrDefault_NeitherError(t *testing.T) {
	_, _, err := LoadTargetsOrDefault(&Config{})
	if err == nil {
		t.Error("LoadTargetsOrDefault() expected error when neither file nor legacy env")
	}
}
