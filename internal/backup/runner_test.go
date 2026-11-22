package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/glennprays/dbeasebackup/config"
	"github.com/glennprays/dbeasebackup/pkg/notifier"
	"github.com/glennprays/dbeasebackup/pkg/storage"
	"github.com/glennprays/dbeasebackup/pkg/testutils"
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

func newTestTarget(name string) *config.Target {
	tgt := &config.Target{
		Name:     name,
		Mode:     "dump",
		Host:     "localhost",
		Port:     "5432",
		User:     "postgres",
		Database: "db",
		Schedule: "* * * * *",
	}
	tgt.Normalize()
	return tgt
}

func newTestService(t *testing.T, name string, provider *testutils.MockProvider, st storage.Storage) *Service {
	t.Helper()
	svc, err := NewService(st, provider, newTestTarget(name), testLogger(t), nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return svc
}

func emptyStorage() storage.Storage {
	return &testutils.MockStorage{}
}

func TestRunner_SuccessNotifiesOnce(t *testing.T) {
	dumps := 0
	provider := &testutils.MockProvider{
		DumpFunc: func(ctx context.Context, dir string) (string, error) {
			dumps++
			if err := os.MkdirAll(dir, 0750); err != nil {
				return "", err
			}
			path := filepath.Join(dir, "x.dump")
			if err := os.WriteFile(path, []byte("d"), 0644); err != nil {
				return "", err
			}
			return path, nil
		},
	}
	svc := newTestService(t, "pg1", provider, emptyStorage())

	notified := make(chan notifier.Event, 4)
	mockNotifier := &testutils.MockNotifier{
		NotifyFunc: func(ctx context.Context, e notifier.Event) {
			notified <- e
		},
	}

	sem := make(chan struct{}, 1)
	runner := NewRunner(svc, newTestTargetTemp(t, "pg1"), sem, testLogger(t), mockNotifier)

	ctx := traceid.NewContext(context.Background(), "test-trace-id")
	if err := runner.Execute(ctx); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	select {
	case e := <-notified:
		if e.Status != notifier.StatusSuccess {
			t.Errorf("event status = %q, want success", e.Status)
		}
		if e.Target != "pg1" {
			t.Errorf("event target = %q, want pg1", e.Target)
		}
	default:
		t.Error("no notification sent on success")
	}
	if len(notified) != 0 {
		t.Errorf("notification count = %d, want exactly 1", 1+len(notified))
	}
	if dumps != 1 {
		t.Errorf("dump count = %d, want 1", dumps)
	}
}

func TestRunner_RetryCount(t *testing.T) {
	dumps := 0
	provider := &testutils.MockProvider{
		DumpFunc: func(ctx context.Context, dir string) (string, error) {
			dumps++
			return "", errors.New("boom")
		},
	}
	svc := newTestService(t, "pg1", provider, emptyStorage())

	target := newTestTarget("pg1")
	zero := 1 // 1 retry = 2 attempts
	target.Retries = &zero

	runner := NewRunner(svc, target, make(chan struct{}, 1), testLogger(t), nil)
	runner.backoffBase = time.Millisecond

	ctx := traceid.NewContext(context.Background(), "test-trace-id")
	err := runner.Execute(ctx)
	if err == nil {
		t.Fatal("Execute() expected error after all attempts fail")
	}
	if dumps != 2 {
		t.Errorf("dump count = %d, want exactly 2 (1 + 1 retry)", dumps)
	}
}

func TestRunner_SuccessStopsRetrying(t *testing.T) {
	dumps := 0
	provider := &testutils.MockProvider{
		DumpFunc: func(ctx context.Context, dir string) (string, error) {
			dumps++
			if dumps == 1 {
				return "", errors.New("transient")
			}
			if err := os.MkdirAll(dir, 0750); err != nil {
				return "", err
			}
			path := filepath.Join(dir, "x.dump")
			if err := os.WriteFile(path, []byte("d"), 0644); err != nil {
				return "", err
			}
			return path, nil
		},
	}
	svc := newTestService(t, "pg1", provider, emptyStorage())

	runner := NewRunner(svc, newTestTargetTemp(t, "pg1"), make(chan struct{}, 1), testLogger(t), nil)
	runner.backoffBase = time.Millisecond

	ctx := traceid.NewContext(context.Background(), "test-trace-id")
	if err := runner.Execute(ctx); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if dumps != 2 {
		t.Errorf("dump count = %d, want 2 (fail then success)", dumps)
	}
}

func TestRunner_OverlapSkip(t *testing.T) {
	// First Execute blocks inside Dump on a channel we control.
	release := make(chan struct{})
	started := make(chan struct{})
	provider := &testutils.MockProvider{
		DumpFunc: func(ctx context.Context, dir string) (string, error) {
			close(started)
			<-release
			return "", errors.New("still failing")
		},
	}
	svc := newTestService(t, "pg1", provider, emptyStorage())

	// retries=0 so the first run doesn't retry-loop; retries nil = default 2,
	// so explicitly zero it.
	noRetry := 0
	target := newTestTarget("pg1")
	target.Retries = &noRetry

	runner := NewRunner(svc, target, make(chan struct{}, 1), testLogger(t), nil)
	ctx := traceid.NewContext(context.Background(), "test-trace-id")

	done := make(chan error, 1)
	go func() { done <- runner.Execute(ctx) }()

	<-started

	// Second Execute must skip immediately (overlap lock held).
	skipDone := make(chan error, 1)
	go func() { skipDone <- runner.Execute(ctx) }()
	select {
	case err := <-skipDone:
		if err != nil {
			t.Errorf("overlap Execute() = %v, want nil skip", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("overlap Execute() did not skip; still blocked")
	}

	close(release)
	if err := <-done; err == nil {
		t.Error("first Execute() expected the provider's error")
	}
}

func TestRunner_PoolBlocksThenRuns(t *testing.T) {
	provider := &testutils.MockProvider{
		DumpFunc: func(ctx context.Context, dir string) (string, error) {
			path := filepath.Join(dir, "x.dump")
			if err := os.WriteFile(path, []byte("d"), 0644); err != nil {
				return "", err
			}
			return path, nil
		},
	}
	svc := newTestService(t, "pg1", provider, emptyStorage())

	// Pool at capacity: Execute must wait, then run.
	sem := make(chan struct{}, 1)
	sem <- struct{}{} // occupy the only slot

	runner := NewRunner(svc, newTestTargetTemp(t, "pg1"), sem, testLogger(t), nil)
	ctx := traceid.NewContext(context.Background(), "test-trace-id")

	done := make(chan error, 1)
	go func() { done <- runner.Execute(ctx) }()

	select {
	case err := <-done:
		t.Fatalf("Execute() finished while pool was full: %v", err)
	case <-time.After(100 * time.Millisecond):
		// Still blocked as expected.
	}

	<-sem // free the slot
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Execute() after slot freed = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Execute() did not run after slot freed")
	}
}

func TestRunner_PoolContextCancelledWhileWaiting(t *testing.T) {
	provider := &testutils.MockProvider{
		DumpFunc: func(ctx context.Context, dir string) (string, error) {
			t.Error("Dump must not run when context cancelled while pool-blocked")
			return "", nil
		},
	}
	svc := newTestService(t, "pg1", provider, emptyStorage())

	sem := make(chan struct{}, 1)
	sem <- struct{}{}

	runner := NewRunner(svc, newTestTargetTemp(t, "pg1"), sem, testLogger(t), nil)

	ctx, cancel := context.WithCancel(traceid.NewContext(context.Background(), "test-trace-id"))
	done := make(chan error, 1)
	go func() { done <- runner.Execute(ctx) }()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Execute() = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Execute() did not return after cancellation")
	}
}

func TestRunner_BackoffRespectsContext(t *testing.T) {
	dumps := 0
	provider := &testutils.MockProvider{
		DumpFunc: func(ctx context.Context, dir string) (string, error) {
			dumps++
			return "", errors.New("boom")
		},
	}
	svc := newTestService(t, "pg1", provider, emptyStorage())

	runner := NewRunner(svc, newTestTargetTemp(t, "pg1"), make(chan struct{}, 1), testLogger(t), nil)

	ctx, cancel := context.WithCancel(traceid.NewContext(context.Background(), "test-trace-id"))
	start := time.Now()
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	err := runner.Execute(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Execute() expected error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Execute() = %v, want wrap context.Canceled", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("Execute() took %v; backoff sleep was not interrupted", elapsed)
	}
	if dumps > 2 {
		t.Errorf("dump count = %d, want <= 2 before cancellation", dumps)
	}
}

func TestService_StatusTracking(t *testing.T) {
	dumpDir := t.TempDir()
	provider := &testutils.MockProvider{
		DumpFunc: func(ctx context.Context, dir string) (string, error) {
			return "", errors.New("kaboom")
		},
	}
	target := newTestTarget("pg1")
	target.BackupDir = dumpDir
	svc, err := NewService(emptyStorage(), provider, target, testLogger(t), nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	before := svc.Status()
	if before.Target != "pg1" {
		t.Errorf("initial status target = %q, want pg1", before.Target)
	}

	ctx := traceid.NewContext(context.Background(), "test-trace-id")
	_ = svc.Backup(ctx)

	after := svc.Status()
	if after.LastError != "kaboom" {
		t.Errorf("LastError = %q, want kaboom", after.LastError)
	}
	if after.LastRun.IsZero() {
		t.Error("LastRun not recorded")
	}

	provider.DumpFunc = func(ctx context.Context, dir string) (string, error) {
		path := filepath.Join(dumpDir, "ok.dump")
		if err := os.WriteFile(path, []byte("d"), 0644); err != nil {
			return "", err
		}
		return path, nil
	}
	if err := svc.Backup(ctx); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	recovered := svc.Status()
	if recovered.LastError != "" {
		t.Errorf("LastError after success = %q, want empty", recovered.LastError)
	}
	if recovered.LastSuccess.IsZero() {
		t.Error("LastSuccess not recorded after success")
	}
}

func TestService_CleanupRetentionFromStorage(t *testing.T) {
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	fresh := time.Now().Add(-time.Hour)

	deleted := make(map[string]bool)
	mockStorage := &testutils.MockStorage{
		ListFunc: func(ctx context.Context, opts storage.ListOptions) ([]storage.FileInfo, error) {
			if opts.Prefix != "pg1_" {
				t.Errorf("List prefix = %q, want pg1_", opts.Prefix)
			}
			return []storage.FileInfo{
				{Name: "pg1_dump_20200101T000000Z.dump"},                 // expired, parseable
				{Name: "pg1_basebackup_20190601T000000Z.tar.gz"},         // expired, parseable
				{Name: "pg1_dump_" + fresh.Format("20060102T150405Z") + ".dump"}, // fresh
				{Name: "junk.txt"},                                       // unparseable
			}, nil
		},
		DeleteFunc: func(ctx context.Context, filename string) error {
			deleted[filename] = true
			return nil
		},
	}

	target := newTestTarget("pg1")
	retention := 30
	target.RetentionDays = &retention
	svc := newTestServiceForTarget(t, target, mockStorage)

	if err := svc.Cleanup(traceid.NewContext(context.Background(), "test-trace-id")); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if !deleted["pg1_dump_20200101T000000Z.dump"] {
		t.Error("expired dump not deleted")
	}
	if !deleted["pg1_basebackup_20190601T000000Z.tar.gz"] {
		t.Error("expired basebackup not deleted")
	}
	if deleted["junk.txt"] {
		t.Error("unparseable name must not be deleted")
	}
	if len(deleted) != 2 {
		t.Errorf("deleted = %v, want exactly the 2 expired files", deleted)
	}
	_ = old
}

func TestService_Cleanup_DisabledRetention(t *testing.T) {
	called := false
	mockStorage := &testutils.MockStorage{
		ListFunc: func(ctx context.Context, opts storage.ListOptions) ([]storage.FileInfo, error) {
			called = true
			return nil, nil
		},
	}
	target := newTestTarget("pg1")
	zero := 0
	target.RetentionDays = &zero
	svc := newTestServiceForTarget(t, target, mockStorage)

	if err := svc.Cleanup(traceid.NewContext(context.Background(), "test-trace-id")); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if called {
		t.Error("List must not be called when retention disabled")
	}
}

func TestService_Cleanup_NotFoundIgnored(t *testing.T) {
	mockStorage := &testutils.MockStorage{
		ListFunc: func(ctx context.Context, opts storage.ListOptions) ([]storage.FileInfo, error) {
			return []storage.FileInfo{{Name: "pg1_dump_20200101T000000Z.dump"}}, nil
		},
		DeleteFunc: func(ctx context.Context, filename string) error {
			return storage.ErrNotFound
		},
	}
	target := newTestTarget("pg1")
	retention := 30
	target.RetentionDays = &retention
	svc := newTestServiceForTarget(t, target, mockStorage)

	if err := svc.Cleanup(traceid.NewContext(context.Background(), "test-trace-id")); err != nil {
		t.Errorf("Cleanup() error = %v, want ErrNotFound tolerated", err)
	}
}

func TestParseBackupFilenameTime(t *testing.T) {
	cases := []struct {
		name    string
		wantOK  bool
		wantUTC time.Time
	}{
		{"pg1_dump_20250615T103045Z.dump", true, time.Date(2025, 6, 15, 10, 30, 45, 0, time.UTC)},
		{"pg1_basebackup_20250615T103045Z.tar.gz", true, time.Date(2025, 6, 15, 10, 30, 45, 0, time.UTC)},
		{"pg1_dump_20240102T030405Z.dump", true, time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)},
		{"junk.txt", false, time.Time{}},
		{"nodate.dump", false, time.Time{}},
		{"pg1_dump_notastamp.dump", false, time.Time{}},
	}
	for _, tc := range cases {
		ts, ok := ParseBackupFilenameTime(tc.name)
		if ok != tc.wantOK {
			t.Errorf("ParseBackupFilenameTime(%q) ok = %v, want %v", tc.name, ok, tc.wantOK)
			continue
		}
		if tc.wantOK && !ts.Equal(tc.wantUTC) {
			t.Errorf("ParseBackupFilenameTime(%q) = %v, want %v", tc.name, ts, tc.wantUTC)
		}
	}
}

func TestConcurrency_PoolCapacity(t *testing.T) {
	// Two runners sharing a 1-slot pool: only one may run at a time.
	var active, maxActive int
	var mu sync.Mutex
	release := make(chan struct{})

	provider := &testutils.MockProvider{
		DumpFunc: func(ctx context.Context, dir string) (string, error) {
			mu.Lock()
			active++
			if active > maxActive {
				maxActive = active
			}
			mu.Unlock()
			<-release
			mu.Lock()
			active--
			mu.Unlock()
			return "", errors.New("done")
		},
	}

	noRetry := 0
	target := newTestTarget("pg1")
	target.Retries = &noRetry

	logger := testLogger(t)
	svc, err := NewService(emptyStorage(), provider, target, logger, nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	sem := make(chan struct{}, 1)
	r1 := NewRunner(svc, target, sem, logger, nil)
	r2 := NewRunner(svc, target, sem, logger, nil)
	ctx := traceid.NewContext(context.Background(), "test-trace-id")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = r1.Execute(ctx) }()
	time.Sleep(20 * time.Millisecond)
	go func() { defer wg.Done(); _ = r2.Execute(ctx) }()

	// Give r2 time to block on the pool.
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if maxActive != 1 {
		t.Errorf("max concurrent dumps = %d, want 1", maxActive)
	}
}

func newTestServiceForTarget(t *testing.T, target *config.Target, st storage.Storage) *Service {
	t.Helper()
	svc, err := NewService(st, &testutils.MockProvider{}, target, testLogger(t), nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return svc
}
