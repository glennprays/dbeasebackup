package backup

import (
	"testing"
	"time"

	"github.com/glennprays/dbeasebackup/config"
)

// targetForTest returns a normalized config.Target with the given mode.
func targetForTest(mode string) *config.Target {
	tgt := &config.Target{
		Name:   "pg1",
		Mode:   mode,
		Host:   "localhost",
		Port:   "5432",
		User:   "postgres",
		Schedule: "* * * * *",
	}
	tgt.Normalize()
	return tgt
}

func TestNewProviderFor_Dump(t *testing.T) {
	verify := true
	target := targetForTest("dump")
	target.Database = "appdb"
	target.Verify = &verify

	p, err := NewProviderFor(target, testLogger(t))
	if err != nil {
		t.Fatalf("NewProviderFor() error = %v", err)
	}
	if _, ok := p.(*PostgresProvider); !ok {
		t.Errorf("NewProviderFor(dump) returned %T, want *PostgresProvider", p)
	}
	if got := p.Name(); got != "postgres-backup" {
		t.Errorf("Name() = %q, want postgres-backup", got)
	}
}

func TestNewProviderFor_Basebackup(t *testing.T) {
	target := targetForTest("basebackup")
	p, err := NewProviderFor(target, testLogger(t))
	if err != nil {
		t.Fatalf("NewProviderFor() error = %v", err)
	}
	if _, ok := p.(*BasebackupProvider); !ok {
		t.Errorf("NewProviderFor(basebackup) returned %T, want *BasebackupProvider", p)
	}
	if got := p.Name(); got != "postgres-basebackup" {
		t.Errorf("Name() = %q, want postgres-basebackup", got)
	}
}

func TestNewProviderFor_InvalidMode(t *testing.T) {
	target := targetForTest("weird")
	if _, err := NewProviderFor(target, testLogger(t)); err == nil {
		t.Error("NewProviderFor() expected error for unknown mode")
	}
}

func TestNewProviderFor_BadTimeout(t *testing.T) {
	target := targetForTest("dump")
	target.Timeout = "not-a-duration"
	if _, err := NewProviderFor(target, testLogger(t)); err == nil {
		t.Error("NewProviderFor() expected error for unparseable timeout")
	}
}

func TestNewProviderFor_ResolvesPasswordEnv(t *testing.T) {
	t.Setenv("PROVIDER_TEST_PW", "env-secret")
	target := targetForTest("dump")
	target.Password = "literal"
	target.PasswordEnv = "PROVIDER_TEST_PW"

	p, err := NewProviderFor(target, testLogger(t))
	if err != nil {
		t.Fatalf("NewProviderFor() error = %v", err)
	}
	pg := p.(*PostgresProvider)
	if pg.cfg.Password != "env-secret" {
		t.Errorf("Password = %q, want env-secret (password_env must win)", pg.cfg.Password)
	}
	if pg.timeout != 30*time.Minute {
		t.Errorf("timeout = %v, want default 30m", pg.timeout)
	}
}
