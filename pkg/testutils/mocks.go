package testutils

import (
	"context"
	"io"

	"github.com/glennprays/dbeasebackup/pkg/notifier"
	"github.com/glennprays/dbeasebackup/pkg/storage"
)

// MockStorage implements storage.Storage interface
type MockStorage struct {
	UploadFunc   func(ctx context.Context, file io.Reader, filename string, opts storage.UploadOptions) error
	DownloadFunc func(ctx context.Context, filename string) (io.ReadCloser, error)
	DeleteFunc   func(ctx context.Context, filename string) error
	ListFunc     func(ctx context.Context, opts storage.ListOptions) ([]storage.FileInfo, error)
	HealthFunc   func(ctx context.Context) error
}

func (m *MockStorage) Upload(ctx context.Context, file io.Reader, filename string, opts storage.UploadOptions) error {
	if m.UploadFunc != nil {
		return m.UploadFunc(ctx, file, filename, opts)
	}
	return nil
}

func (m *MockStorage) Download(ctx context.Context, filename string) (io.ReadCloser, error) {
	if m.DownloadFunc != nil {
		return m.DownloadFunc(ctx, filename)
	}
	return nil, nil
}

func (m *MockStorage) Delete(ctx context.Context, filename string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, filename)
	}
	return nil
}

func (m *MockStorage) List(ctx context.Context, opts storage.ListOptions) ([]storage.FileInfo, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, opts)
	}
	return nil, nil
}

func (m *MockStorage) Health(ctx context.Context) error {
	if m.HealthFunc != nil {
		return m.HealthFunc(ctx)
	}
	return nil
}

// MockProvider implements backup.Provider interface
type MockProvider struct {
	NameFunc                 func() string
	DumpFunc                 func(ctx context.Context, backupDir string) (string, error)
	CleanupFunc              func(ctx context.Context, filePath string) error
	ValidateDependenciesFunc func() error
}

func (m *MockProvider) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock-provider"
}

func (m *MockProvider) Dump(ctx context.Context, backupDir string) (string, error) {
	if m.DumpFunc != nil {
		return m.DumpFunc(ctx, backupDir)
	}
	return "", nil
}

func (m *MockProvider) Cleanup(ctx context.Context, filePath string) error {
	if m.CleanupFunc != nil {
		return m.CleanupFunc(ctx, filePath)
	}
	return nil
}

func (m *MockProvider) ValidateDependencies() error {
	if m.ValidateDependenciesFunc != nil {
		return m.ValidateDependenciesFunc()
	}
	return nil
}

// MockJob implements scheduler.Job interface
type MockJob struct {
	NameFunc     func() string
	ExecuteFunc  func(ctx context.Context) error
	ExecuteCount int
}

func (m *MockJob) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock-job"
}

func (m *MockJob) Execute(ctx context.Context) error {
	m.ExecuteCount++
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx)
	}
	return nil
}

// MockNotifier implements notifier.Notifier interface
type MockNotifier struct {
	NotifyFunc  func(ctx context.Context, event notifier.Event)
	NotifyCount int
	LastEvent   notifier.Event
}

func (m *MockNotifier) Notify(ctx context.Context, event notifier.Event) {
	m.NotifyCount++
	m.LastEvent = event
	if m.NotifyFunc != nil {
		m.NotifyFunc(ctx, event)
	}
}
