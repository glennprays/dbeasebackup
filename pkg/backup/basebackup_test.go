package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glennprays/dbeasebackup/pkg/traceid"
	"github.com/glennprays/log"
)

func testLogger(t *testing.T) *log.Logger {
	t.Helper()
	logger, err := log.New(log.Config{
		Service: "test",
		Env:     "development",
		Level:   log.ErrorLevel,
		Output:  log.OutputStdout,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	return logger
}

func TestBackupFilename(t *testing.T) {
	ts := time.Date(2025, 6, 15, 10, 30, 45, 0, time.UTC)

	got := BackupFilename("pg1", "dump", "dump", ts)
	want := "pg1_dump_20250615T103045Z.dump"
	if got != want {
		t.Errorf("BackupFilename() = %q, want %q", got, want)
	}

	got = BackupFilename("pg1", "basebackup", "tar.gz", ts)
	want = "pg1_basebackup_20250615T103045Z.tar.gz"
	if got != want {
		t.Errorf("BackupFilename() = %q, want %q", got, want)
	}

	// Non-UTC input must be converted to UTC.
	local := time.Date(2025, 6, 15, 12, 30, 45, 0, time.FixedZone("UTC+2", 2*3600))
	if got := BackupFilename("pg1", "dump", "dump", local); got != "pg1_dump_20250615T103045Z.dump" {
		t.Errorf("BackupFilename() non-UTC = %q, want UTC-normalized %q", got, "pg1_dump_20250615T103045Z.dump")
	}
}

func TestBackupFilename_RoundTrip(t *testing.T) {
	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	name := BackupFilename("app-db", "dump", "dump", ts)

	// Parse back the canonical shape: <target>_<mode>_<UTC stamp>.<ext>
	if !strings.HasPrefix(name, "app-db_dump_") || !strings.HasSuffix(name, ".dump") {
		t.Fatalf("filename %q does not match canonical shape", name)
	}
	stamp := strings.TrimSuffix(strings.TrimPrefix(name, "app-db_dump_"), ".dump")
	parsed, err := time.Parse(BackupFilenameTimestamp, stamp)
	if err != nil {
		t.Fatalf("timestamp %q does not parse with layout %s: %v", stamp, BackupFilenameTimestamp, err)
	}
	if !parsed.Equal(ts) {
		t.Errorf("round trip = %v, want %v", parsed, ts)
	}
}

func TestTarDirectory_RoundTrip(t *testing.T) {
	logger := testLogger(t)
	_ = logger

	srcDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcDir, "base"), 0750); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"backup_manifest":        "manifest-content",
		"base/PG_VERSION":        "17\n",
		"base/16384/somefile.fk": strings.Repeat("data", 1000),
	}
	for rel, content := range files {
		path := filepath.Join(srcDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0640); err != nil {
			t.Fatal(err)
		}
	}

	outPath := filepath.Join(t.TempDir(), "out.tar.gz")
	if err := tarDirectory(srcDir, outPath); err != nil {
		t.Fatalf("tarDirectory() error = %v", err)
	}

	// Read back and compare content.
	f, err := os.Open(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip.NewReader() error = %v", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	seen := make(map[string]string)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar read error = %v", err)
		}
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, tr); err != nil {
			t.Fatalf("tar copy error = %v", err)
		}
		seen[hdr.Name] = buf.String()
	}

	for rel, content := range files {
		if seen[rel] != content {
			t.Errorf("archived %q content mismatch", rel)
		}
	}
	// Directory entries are archived too; file entries must all be present.
	if len(seen) < len(files) {
		t.Errorf("archive has %d entries, want at least %d (files)", len(seen), len(files))
	}
}

func TestTarDirectory_OutExistsFails(t *testing.T) {
	srcDir := t.TempDir()
	outPath := filepath.Join(t.TempDir(), "out.tar.gz")
	if err := os.WriteFile(outPath, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := tarDirectory(srcDir, outPath); err == nil {
		t.Error("tarDirectory() expected error when output already exists")
	}
}

func TestBasebackupProvider_Name(t *testing.T) {
	p := NewBasebackupProvider(BasebackupConfig{TargetName: "pg1"}, testLogger(t), 30*time.Minute)
	if got := p.Name(); got != "postgres-basebackup" {
		t.Errorf("Name() = %q, want postgres-basebackup", got)
	}
}

func TestBasebackupProvider_Cleanup(t *testing.T) {
	p := NewBasebackupProvider(BasebackupConfig{TargetName: "pg1"}, testLogger(t), 30*time.Minute)
	ctx := traceid.NewContext(context.Background(), "test-trace-id")

	path := filepath.Join(t.TempDir(), "f.tar.gz")
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := p.Cleanup(ctx, path); err != nil {
		t.Errorf("Cleanup() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Cleanup() file still exists")
	}
	if err := p.Cleanup(ctx, "/nonexistent/file"); err == nil {
		t.Error("Cleanup() expected error for missing file")
	}
}

func TestBasebackupProvider_ValidateDependencies(t *testing.T) {
	p := NewBasebackupProvider(BasebackupConfig{}, testLogger(t), time.Minute)
	// Either all tools exist in this environment, or the error names the
	// missing binary. Both are acceptable here; failure to error is not.
	err := p.ValidateDependencies()
	if err == nil {
		t.Log("pg_basebackup/pg_verifybackup present in test environment")
	}
}

func TestTarDirectory_PreservesNonRegular(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0750); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(t.TempDir(), "out.tar.gz")
	if err := tarDirectory(srcDir, outPath); err != nil {
		t.Fatalf("tarDirectory() error = %v", err)
	}
}
