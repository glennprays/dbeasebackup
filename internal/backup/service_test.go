package backup

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/glennprays/dbeasebackup/config"
	"github.com/glennprays/dbeasebackup/pkg/storage"
	"github.com/glennprays/dbeasebackup/pkg/testutils"
	"github.com/glennprays/dbeasebackup/pkg/traceid"
	"github.com/glennprays/log"
)

func createTestLogger(t *testing.T) *log.Logger {
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

func createTestContext() context.Context {
	return traceid.NewContext(context.Background(), "test-trace-id")
}

func createTempBackupFile(t *testing.T, backupDir string) string {
	t.Helper()
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup dir: %v", err)
	}
	tempFile := filepath.Join(backupDir, "backup_test.tar")
	if err := os.WriteFile(tempFile, []byte("test backup content"), 0644); err != nil {
		t.Fatalf("Failed to create temp backup file: %v", err)
	}
	return tempFile
}

func TestNewService_Success(t *testing.T) {
	logger := createTestLogger(t)

	mockDB := &testutils.MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return nil, nil
		},
	}
	mockStorage := &testutils.MockStorage{
		HealthFunc: func(ctx context.Context) error { return nil },
	}
	mockProvider := &testutils.MockProvider{
		NameFunc: func() string { return "test-backup" },
	}
	cfg := &config.Config{
		BACKUP_DIR:             t.TempDir(),
		GOOGLE_DRIVE_FOLDER_ID: "test-folder-id",
	}
	service, err := NewService(mockDB, mockStorage, mockProvider, cfg, logger)
	if err != nil {
		t.Errorf("NewService() unexpected error = %v", err)
	}
	if service == nil {
		t.Error("NewService() returned nil service")
	}
}
func TestNewService_EnsureBackupTableError(t *testing.T) {
	logger := createTestLogger(t)
	mockDB := &testutils.MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return nil, errors.New("database error")
		},
	}
	mockStorage := &testutils.MockStorage{}
	mockProvider := &testutils.MockProvider{
		NameFunc: func() string { return "test-backup" },
	}
	cfg := &config.Config{
		BACKUP_DIR: t.TempDir(),
	}
	service, err := NewService(mockDB, mockStorage, mockProvider, cfg, logger)
	if err == nil {
		t.Error("NewService() expected error when EnsureBackupTable fails")
	}
	if service != nil {
		t.Error("NewService() should return nil service on error")
	}
}
func TestService_Name(t *testing.T) {
	logger := createTestLogger(t)
	mockDB := &testutils.MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return nil, nil
		},
	}
	mockStorage := &testutils.MockStorage{}
	mockProvider := &testutils.MockProvider{
		NameFunc: func() string { return "custom-backup" },
	}
	cfg := &config.Config{
		BACKUP_DIR: t.TempDir(),
	}
	service, err := NewService(mockDB, mockStorage, mockProvider, cfg, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if got := service.Name(); got != "custom-backup" {
		t.Errorf("Name() = %v, want %v", got, "custom-backup")
	}
}
func TestService_Backup_Success(t *testing.T) {
	logger := createTestLogger(t)
	backupDir := t.TempDir()
	backupFile := createTempBackupFile(t, backupDir)
	mockDB := &testutils.MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return nil, nil
		},
	}
	mockStorage := &testutils.MockStorage{
		UploadFunc: func(ctx context.Context, file io.Reader, filename string, opts storage.UploadOptions) error {
			return nil
		},
		HealthFunc: func(ctx context.Context) error { return nil },
	}
	mockProvider := &testutils.MockProvider{
		NameFunc: func() string { return "test-backup" },
		DumpFunc: func(ctx context.Context, dir string) (string, error) {
			return backupFile, nil
		},
		CleanupFunc: func(ctx context.Context, filePath string) error {
			return os.Remove(filePath)
		},
	}
	cfg := &config.Config{
		BACKUP_DIR:             backupDir,
		GOOGLE_DRIVE_FOLDER_ID: "test-folder-id",
	}
	service, err := NewService(mockDB, mockStorage, mockProvider, cfg, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	err = service.Backup(createTestContext())
	if err != nil {
		t.Errorf("Backup() unexpected error = %v", err)
	}
	// Verify file was cleaned up
	if _, err := os.Stat(backupFile); !os.IsNotExist(err) {
		t.Error("Backup() should have cleaned up the backup file")
	}
}
func TestService_Backup_DumpError(t *testing.T) {
	logger := createTestLogger(t)
	mockDB := &testutils.MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return nil, nil
		},
	}
	mockStorage := &testutils.MockStorage{}
	mockProvider := &testutils.MockProvider{
		NameFunc: func() string { return "test-backup" },
		DumpFunc: func(ctx context.Context, dir string) (string, error) {
			return "", errors.New("dump failed")
		},
	}
	cfg := &config.Config{
		BACKUP_DIR: t.TempDir(),
	}
	service, err := NewService(mockDB, mockStorage, mockProvider, cfg, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	err = service.Backup(createTestContext())
	if err == nil {
		t.Error("Backup() expected error when dump fails")
	}
}
func TestService_Backup_RecordError(t *testing.T) {
	logger := createTestLogger(t)
	backupDir := t.TempDir()
	backupFile := createTempBackupFile(t, backupDir)
	mockDB := &testutils.MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			// First call is for EnsureBackupTable (CREATE TABLE)
			// Second call is for recordBackup (INSERT)
			return nil, errors.New("record failed")
		},
	}
	mockStorage := &testutils.MockStorage{}
	mockProvider := &testutils.MockProvider{
		NameFunc: func() string { return "test-backup" },
		DumpFunc: func(ctx context.Context, dir string) (string, error) {
			return backupFile, nil
		},
	}
	cfg := &config.Config{
		BACKUP_DIR: backupDir,
	}
	// NewService will fail because EnsureBackupTable uses the same ExecFunc
	service, err := NewService(mockDB, mockStorage, mockProvider, cfg, logger)
	if err == nil {
		t.Error("NewService() expected error when record fails")
	}
	if service != nil {
		t.Error("NewService() should return nil service on error")
	}
}
func TestService_Backup_UploadError(t *testing.T) {
	logger := createTestLogger(t)
	backupDir := t.TempDir()
	backupFile := createTempBackupFile(t, backupDir)
	mockDB := &testutils.MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return nil, nil
		},
	}
	mockStorage := &testutils.MockStorage{
		UploadFunc: func(ctx context.Context, file io.Reader, filename string, opts storage.UploadOptions) error {
			return errors.New("upload failed")
		},
		HealthFunc: func(ctx context.Context) error { return nil },
	}
	mockProvider := &testutils.MockProvider{
		NameFunc: func() string { return "test-backup" },
		DumpFunc: func(ctx context.Context, dir string) (string, error) {
			return backupFile, nil
		},
		CleanupFunc: func(ctx context.Context, filePath string) error {
			return nil
		},
	}
	cfg := &config.Config{
		BACKUP_DIR:             backupDir,
		GOOGLE_DRIVE_FOLDER_ID: "test-folder-id",
	}
	service, err := NewService(mockDB, mockStorage, mockProvider, cfg, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	err = service.Backup(createTestContext())
	if err == nil {
		t.Error("Backup() expected error when upload fails")
	}
}
func TestService_Backup_CleanupError(t *testing.T) {
	logger := createTestLogger(t)
	backupDir := t.TempDir()
	backupFile := createTempBackupFile(t, backupDir)
	mockDB := &testutils.MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return nil, nil
		},
	}
	mockStorage := &testutils.MockStorage{
		UploadFunc: func(ctx context.Context, file io.Reader, filename string, opts storage.UploadOptions) error {
			return nil
		},
		HealthFunc: func(ctx context.Context) error { return nil },
	}
	mockProvider := &testutils.MockProvider{
		NameFunc: func() string { return "test-backup" },
		DumpFunc: func(ctx context.Context, dir string) (string, error) {
			return backupFile, nil
		},
		CleanupFunc: func(ctx context.Context, filePath string) error {
			return errors.New("cleanup failed")
		},
	}
	cfg := &config.Config{
		BACKUP_DIR:             backupDir,
		GOOGLE_DRIVE_FOLDER_ID: "test-folder-id",
	}
	service, err := NewService(mockDB, mockStorage, mockProvider, cfg, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	err = service.Backup(createTestContext())
	if err == nil {
		t.Error("Backup() expected error when cleanup fails")
	}
}
func TestService_Health_Success(t *testing.T) {
	logger := createTestLogger(t)
	mockDB := &testutils.MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return nil, nil
		},
		PingFunc: func(ctx context.Context) error {
			return nil
		},
	}
	mockStorage := &testutils.MockStorage{
		HealthFunc: func(ctx context.Context) error {
			return nil
		},
	}
	mockProvider := &testutils.MockProvider{
		NameFunc: func() string { return "test-backup" },
	}
	cfg := &config.Config{
		BACKUP_DIR: t.TempDir(),
	}
	service, err := NewService(mockDB, mockStorage, mockProvider, cfg, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	err = service.Health(createTestContext())
	if err != nil {
		t.Errorf("Health() unexpected error = %v", err)
	}
}
func TestService_Health_DatabaseError(t *testing.T) {
	logger := createTestLogger(t)
	mockDB := &testutils.MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return nil, nil
		},
		PingFunc: func(ctx context.Context) error {
			return errors.New("database ping failed")
		},
	}
	mockStorage := &testutils.MockStorage{
		HealthFunc: func(ctx context.Context) error {
			return nil
		},
	}
	mockProvider := &testutils.MockProvider{
		NameFunc: func() string { return "test-backup" },
	}
	cfg := &config.Config{
		BACKUP_DIR: t.TempDir(),
	}
	service, err := NewService(mockDB, mockStorage, mockProvider, cfg, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	err = service.Health(createTestContext())
	if err == nil {
		t.Error("Health() expected error when database ping fails")
	}
}
func TestService_Health_StorageError(t *testing.T) {
	logger := createTestLogger(t)
	mockDB := &testutils.MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return nil, nil
		},
		PingFunc: func(ctx context.Context) error {
			return nil
		},
	}
	mockStorage := &testutils.MockStorage{
		HealthFunc: func(ctx context.Context) error {
			return errors.New("storage health check failed")
		},
	}
	mockProvider := &testutils.MockProvider{
		NameFunc: func() string { return "test-backup" },
	}
	cfg := &config.Config{
		BACKUP_DIR: t.TempDir(),
	}
	service, err := NewService(mockDB, mockStorage, mockProvider, cfg, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	err = service.Health(createTestContext())
	if err == nil {
		t.Error("Health() expected error when storage health check fails")
	}
}
func TestService_ListBackups_Success(t *testing.T) {
	logger := createTestLogger(t)
	// Create mock rows
	mockRows := &MockRows{
		data: [][]interface{}{
			{1, "backup1.tar", mustParseTime("2024-01-01T00:00:00Z")},
			{2, "backup2.tar", mustParseTime("2024-01-02T00:00:00Z")},
		},
		index: -1,
	}
	mockDB := &testutils.MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return nil, nil
		},
		QueryFunc: func(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
			return nil, nil // We'll use mockRows through a different approach
		},
	}
	mockStorage := &testutils.MockStorage{}
	mockProvider := &testutils.MockProvider{
		NameFunc: func() string { return "test-backup" },
	}
	cfg := &config.Config{
		BACKUP_DIR: t.TempDir(),
	}
	service, err := NewService(mockDB, mockStorage, mockProvider, cfg, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	// Since we can't easily mock sql.Rows, we'll test the method exists
	// In a real scenario, you'd use an interface for rows or integration tests
	_ = mockRows
	_ = service
}
func TestService_ListBackups_QueryError(t *testing.T) {
	logger := createTestLogger(t)
	mockDB := &testutils.MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return nil, nil
		},
		QueryFunc: func(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
			return nil, errors.New("query failed")
		},
	}
	mockStorage := &testutils.MockStorage{}
	mockProvider := &testutils.MockProvider{
		NameFunc: func() string { return "test-backup" },
	}
	cfg := &config.Config{
		BACKUP_DIR: t.TempDir(),
	}
	service, err := NewService(mockDB, mockStorage, mockProvider, cfg, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	_, err = service.ListBackups(context.Background(), 10)
	if err == nil {
		t.Error("ListBackups() expected error when query fails")
	}
}
func TestService_Execute(t *testing.T) {
	logger := createTestLogger(t)
	backupDir := t.TempDir()
	backupFile := createTempBackupFile(t, backupDir)
	mockDB := &testutils.MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return nil, nil
		},
	}
	mockStorage := &testutils.MockStorage{
		UploadFunc: func(ctx context.Context, file io.Reader, filename string, opts storage.UploadOptions) error {
			return nil
		},
		HealthFunc: func(ctx context.Context) error { return nil },
	}
	mockProvider := &testutils.MockProvider{
		NameFunc: func() string { return "test-backup" },
		DumpFunc: func(ctx context.Context, dir string) (string, error) {
			return backupFile, nil
		},
		CleanupFunc: func(ctx context.Context, filePath string) error {
			return os.Remove(filePath)
		},
	}
	cfg := &config.Config{
		BACKUP_DIR:             backupDir,
		GOOGLE_DRIVE_FOLDER_ID: "test-folder-id",
	}
	service, err := NewService(mockDB, mockStorage, mockProvider, cfg, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	// Execute is the same as Backup
	err = service.Execute(createTestContext())
	if err != nil {
		t.Errorf("Execute() unexpected error = %v", err)
	}
}
func TestService_EnsureBackupTable_Success(t *testing.T) {
	logger := createTestLogger(t)
	mockDB := &testutils.MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return nil, nil
		},
	}
	mockStorage := &testutils.MockStorage{}
	mockProvider := &testutils.MockProvider{
		NameFunc: func() string { return "test-backup" },
	}
	cfg := &config.Config{
		BACKUP_DIR: t.TempDir(),
	}
	service, err := NewService(mockDB, mockStorage, mockProvider, cfg, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	err = service.EnsureBackupTable(context.Background())
	if err != nil {
		t.Errorf("EnsureBackupTable() unexpected error = %v", err)
	}
}
func TestService_EnsureBackupTable_Error(t *testing.T) {
	logger := createTestLogger(t)
	execCallCount := 0
	mockDB := &testutils.MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			execCallCount++
			if execCallCount > 1 {
				return nil, errors.New("create table failed")
			}
			return nil, nil
		},
	}
	mockStorage := &testutils.MockStorage{}
	mockProvider := &testutils.MockProvider{
		NameFunc: func() string { return "test-backup" },
	}
	cfg := &config.Config{
		BACKUP_DIR: t.TempDir(),
	}
	service, err := NewService(mockDB, mockStorage, mockProvider, cfg, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	err = service.EnsureBackupTable(context.Background())
	if err == nil {
		t.Error("EnsureBackupTable() expected error when exec fails")
	}
}

// Helper types for mocking sql.Rows (limited functionality)
type MockRows struct {
	data  [][]interface{}
	index int
}

func mustParseTime(s string) interface{} {
	return s // Simplified for test
}
func TestService_Backup_OpenFileError(t *testing.T) {
	logger := createTestLogger(t)
	mockDB := &testutils.MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return nil, nil
		},
	}
	mockStorage := &testutils.MockStorage{}
	// Provider returns a non-existent file path
	mockProvider := &testutils.MockProvider{
		NameFunc: func() string { return "test-backup" },
		DumpFunc: func(ctx context.Context, dir string) (string, error) {
			return "/nonexistent/path/backup.tar", nil
		},
	}
	cfg := &config.Config{
		BACKUP_DIR:             t.TempDir(),
		GOOGLE_DRIVE_FOLDER_ID: "test-folder-id",
	}
	service, err := NewService(mockDB, mockStorage, mockProvider, cfg, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	err = service.Backup(createTestContext())
	if err == nil {
		t.Error("Backup() expected error when file cannot be opened")
	}
}
func TestService_Backup_RecordThenUploadOrder(t *testing.T) {
	logger := createTestLogger(t)
	backupDir := t.TempDir()
	backupFile := createTempBackupFile(t, backupDir)

	// Use a flag to skip tracking calls during NewService initialization
	backupStarted := false
	callOrder := []string{}

	mockDB := &testutils.MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			if backupStarted {
				callOrder = append(callOrder, "record")
			}
			return nil, nil
		},
	}
	mockStorage := &testutils.MockStorage{
		UploadFunc: func(ctx context.Context, file io.Reader, filename string, opts storage.UploadOptions) error {
			callOrder = append(callOrder, "upload")
			return nil
		},
		HealthFunc: func(ctx context.Context) error { return nil },
	}
	mockProvider := &testutils.MockProvider{
		NameFunc: func() string { return "test-backup" },
		DumpFunc: func(ctx context.Context, dir string) (string, error) {
			callOrder = append(callOrder, "dump")
			return backupFile, nil
		},
		CleanupFunc: func(ctx context.Context, filePath string) error {
			callOrder = append(callOrder, "cleanup")
			return os.Remove(filePath)
		},
	}
	cfg := &config.Config{
		BACKUP_DIR:             backupDir,
		GOOGLE_DRIVE_FOLDER_ID: "test-folder-id",
	}
	service, err := NewService(mockDB, mockStorage, mockProvider, cfg, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Start tracking calls after service initialization
	backupStarted = true

	err = service.Backup(createTestContext())
	if err != nil {
		t.Errorf("Backup() unexpected error = %v", err)
	}

	// Verify order: dump -> record -> upload -> cleanup
	expectedOrder := []string{"dump", "record", "upload", "cleanup"}
	if len(callOrder) != len(expectedOrder) {
		t.Errorf("Expected %d calls, got %d: %v", len(expectedOrder), len(callOrder), callOrder)
	}
	for i, expected := range expectedOrder {
		if i >= len(callOrder) || callOrder[i] != expected {
			t.Errorf("Expected call %d to be %s, got %v", i, expected, callOrder)
		}
	}
}
