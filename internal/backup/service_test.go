package backup

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glennprays/dbeasebackup/config"
	"github.com/glennprays/dbeasebackup/pkg/notifier"
	"github.com/glennprays/dbeasebackup/pkg/storage"
	"github.com/glennprays/dbeasebackup/pkg/testutils"
	"github.com/glennprays/dbeasebackup/pkg/traceid"
)

// newOKProvider returns a mock provider that produces a real temp file
// inside the passed backupDir.
func newOKProvider() *testutils.MockProvider {
	return &testutils.MockProvider{
		DumpFunc: func(ctx context.Context, backupDir string) (string, error) {
			if err := os.MkdirAll(backupDir, 0750); err != nil {
				return "", err
			}
			path := filepath.Join(backupDir, "target_dump_20250101T000000Z.dump")
			if err := os.WriteFile(path, []byte("dump-content"), 0644); err != nil {
				return "", err
			}
			return path, nil
		},
	}
}

func ctxWithTrace() context.Context {
	return traceid.NewContext(context.Background(), "test-trace-id")
}

// newTestTargetTemp returns a normalized target whose BackupDir lives in a
// per-test temp directory, keeping the suite isolated from the repo cwd.
func newTestTargetTemp(t *testing.T, name string) *config.Target {
	t.Helper()
	target := newTestTarget(name)
	target.BackupDir = t.TempDir()
	return target
}

func TestService_Backup_SuccessWorkflowOrder(t *testing.T) {
	var order []string
	okProvider := &testutils.MockProvider{
		DumpFunc: func(ctx context.Context, backupDir string) (string, error) {
			order = append(order, "dump")
			path := filepath.Join(t.TempDir(), "t_dump_x.dump")
			if err := os.WriteFile(path, []byte("d"), 0644); err != nil {
				return "", err
			}
			return path, nil
		},
		CleanupFunc: func(ctx context.Context, filePath string) error {
			order = append(order, "cleanup")
			return os.Remove(filePath)
		},
	}
	mockStorage := &testutils.MockStorage{
		UploadFunc: func(ctx context.Context, file io.Reader, filename string, opts storage.UploadOptions) error {
			order = append(order, "upload")
			return nil
		},
	}

	svc, err := NewService(mockStorage, okProvider, newTestTargetTemp(t, "t"), testLogger(t), nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := svc.Backup(ctxWithTrace()); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	want := []string{"dump", "upload", "cleanup"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Errorf("workflow order = %v, want %v", order, want)
	}
}

func TestService_Backup_VerifyEnabledForDump(t *testing.T) {
	okProvider := newOKProvider()
	verified := false
	mockStorage := &testutils.MockStorage{
		UploadFunc: func(ctx context.Context, file io.Reader, filename string, opts storage.UploadOptions) error {
			return nil
		},
		DownloadFunc: func(ctx context.Context, filename string) (io.ReadCloser, error) {
			verified = true
			return io.NopCloser(bytes.NewReader([]byte("d"))), nil
		},
	}

	svc, err := NewService(mockStorage, okProvider, newTestTargetTemp(t, "t"), testLogger(t), nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := svc.Backup(ctxWithTrace()); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	if !verified {
		t.Error("verify path (Download) not invoked for dump-mode target with verify enabled")
	}
}

func TestService_Backup_VerifyWarnsNotFails(t *testing.T) {
	okProvider := newOKProvider()
	mockStorage := &testutils.MockStorage{
		UploadFunc: func(ctx context.Context, file io.Reader, filename string, opts storage.UploadOptions) error {
			return nil
		},
		DownloadFunc: func(ctx context.Context, filename string) (io.ReadCloser, error) {
			return nil, errors.New("download exploded")
		},
	}

	svc, err := NewService(mockStorage, okProvider, newTestTargetTemp(t, "t"), testLogger(t), nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	// Verify failure must NOT fail the workflow (warn-not-fail semantics).
	if err := svc.Backup(ctxWithTrace()); err != nil {
		t.Errorf("Backup() error = %v, want nil (verify is warn-not-fail)", err)
	}
}

func TestService_Backup_UploadErrorCleansUpLocal(t *testing.T) {
	cleaned := false
	okProvider := &testutils.MockProvider{
		DumpFunc: func(ctx context.Context, backupDir string) (string, error) {
			path := filepath.Join(t.TempDir(), "t_dump_x.dump")
			if err := os.WriteFile(path, []byte("d"), 0644); err != nil {
				return "", err
			}
			return path, nil
		},
		CleanupFunc: func(ctx context.Context, filePath string) error {
			cleaned = true
			return os.Remove(filePath)
		},
	}
	mockStorage := &testutils.MockStorage{
		UploadFunc: func(ctx context.Context, file io.Reader, filename string, opts storage.UploadOptions) error {
			return errors.New("upload failed")
		},
	}

	svc, err := NewService(mockStorage, okProvider, newTestTargetTemp(t, "t"), testLogger(t), nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := svc.Backup(ctxWithTrace()); err == nil {
		t.Error("Backup() expected error on upload failure")
	}
	if !cleaned {
		t.Error("local file not cleaned up after upload failure")
	}
}

func TestService_Backup_DumpErrorNoUpload(t *testing.T) {
	uploaded := false
	failProvider := &testutils.MockProvider{
		DumpFunc: func(ctx context.Context, backupDir string) (string, error) {
			return "", errors.New("dump failed")
		},
	}
	mockStorage := &testutils.MockStorage{
		UploadFunc: func(ctx context.Context, file io.Reader, filename string, opts storage.UploadOptions) error {
			uploaded = true
			return nil
		},
	}

	svc, err := NewService(mockStorage, failProvider, newTestTarget("t"), testLogger(t), nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := svc.Backup(ctxWithTrace()); err == nil {
		t.Error("Backup() expected error on dump failure")
	}
	if uploaded {
		t.Error("upload must not run when dump fails")
	}
}

func TestService_Backup_PassesTargetFolderOverride(t *testing.T) {
	okProvider := newOKProvider()
	var gotOpts storage.UploadOptions
	mockStorage := &testutils.MockStorage{
		UploadFunc: func(ctx context.Context, file io.Reader, filename string, opts storage.UploadOptions) error {
			gotOpts = opts
			return nil
		},
	}

	target := newTestTarget("t")
	target.Storage.FolderID = "folder-42"
	svc, err := NewService(mockStorage, okProvider, target, testLogger(t), nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := svc.Backup(ctxWithTrace()); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	if gotOpts.FolderID != "folder-42" {
		t.Errorf("UploadOptions.FolderID = %q, want folder-42", gotOpts.FolderID)
	}
}

func TestService_NewService_NilTarget(t *testing.T) {
	if _, err := NewService(&testutils.MockStorage{}, &testutils.MockProvider{}, nil, testLogger(t), nil); err == nil {
		t.Error("NewService() expected error for nil target")
	}
}

func TestService_Name(t *testing.T) {
	svc, err := NewService(&testutils.MockStorage{}, &testutils.MockProvider{}, newTestTarget("special"), testLogger(t), nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if got := svc.Name(); got != "backup:special" {
		t.Errorf("Name() = %q, want backup:special", got)
	}
}

func TestService_CleanupJob(t *testing.T) {
	svc, err := NewService(&testutils.MockStorage{}, &testutils.MockProvider{}, newTestTarget("t"), testLogger(t), nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	job := NewCleanupJob(svc)
	if job.Name() != "backup-cleanup:t" {
		t.Errorf("Name() = %q, want backup-cleanup:t", job.Name())
	}
	if err := job.Execute(ctxWithTrace()); err != nil {
		// retention default 30 triggers List on MockStorage (nil ListFunc
		// returns nil,nil) — must not error.
		t.Errorf("CleanupJob.Execute() error = %v", err)
	}
}

func TestService_Health(t *testing.T) {
	healthy := &testutils.MockStorage{
		HealthFunc: func(ctx context.Context) error { return nil },
	}
	unhealthy := &testutils.MockStorage{
		HealthFunc: func(ctx context.Context) error { return errors.New("down") },
	}

	svcOK, _ := NewService(healthy, &testutils.MockProvider{}, newTestTarget("t"), testLogger(t), nil)
	svcBad, _ := NewService(unhealthy, &testutils.MockProvider{}, newTestTarget("t"), testLogger(t), nil)

	if err := svcOK.Health(ctxWithTrace()); err != nil {
		t.Errorf("Health() = %v, want nil", err)
	}
	if err := svcBad.Health(ctxWithTrace()); err == nil {
		t.Error("Health() expected error when storage unhealthy")
	}
}

func TestService_NotifierReceivesEvent_Type(t *testing.T) {
	// Notifications are owned by the Runner; Service.Backup must not send.
	var sent []notifier.Event
	mockNotifier := &testutils.MockNotifier{
		NotifyFunc: func(ctx context.Context, e notifier.Event) {
			sent = append(sent, e)
		},
	}
	okProvider := newOKProvider()
	mockStorage := &testutils.MockStorage{}

	svc, err := NewService(mockStorage, okProvider, newTestTarget("t"), testLogger(t), mockNotifier)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := svc.Backup(ctxWithTrace()); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	if len(sent) != 0 {
		t.Errorf("Service.Backup sent %d events; notification belongs to Runner only", len(sent))
	}
}

func TestService_VerifySkippedWhenDisabled(t *testing.T) {
	okProvider := newOKProvider()
	downloaded := false
	mockStorage := &testutils.MockStorage{
		DownloadFunc: func(ctx context.Context, filename string) (io.ReadCloser, error) {
			downloaded = true
			return io.NopCloser(strings.NewReader("d")), nil
		},
	}

	target := newTestTarget("t")
	target.Verify = new(bool) // verify off

	svc, err := NewService(mockStorage, okProvider, target, testLogger(t), nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := svc.Backup(ctxWithTrace()); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	if downloaded {
		t.Error("verify ran despite verify: false")
	}
}

func TestService_Cleanup_ListError(t *testing.T) {
	listErr := errors.New("list boom")
	mockStorage := &testutils.MockStorage{
		ListFunc: func(ctx context.Context, opts storage.ListOptions) ([]storage.FileInfo, error) {
			return nil, listErr
		},
	}
	target := newTestTarget("t") // default retention 30
	svc, err := NewService(mockStorage, &testutils.MockProvider{}, target, testLogger(t), nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	err = svc.Cleanup(ctxWithTrace())
	if err == nil {
		t.Fatal("Cleanup() expected error when List fails")
	}
	if !errors.Is(err, listErr) {
		t.Errorf("Cleanup() error = %v, want wrap of list boom", err)
	}
}
