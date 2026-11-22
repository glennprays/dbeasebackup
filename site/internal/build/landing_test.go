package build

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLandingSectionsAndLinks(t *testing.T) {
	out := buildForTest(t) // version v0.0.0-test: not the pre-release version
	html := readOut(t, out, "index.html")
	for _, s := range []string{
		`id="hero-title"`, `class="ledger-panel"`, "Example data",
		`id="run-title"`, `id="config-title"`, `id="modes-title"`,
		`id="storage-title"`, `id="ops-title"`, `id="start-title"`,
		`class="cmd"`, "glennprays/dbeasebackup",
		`href="/docs/quick-start/"`, `href="` + RepoURL + `"`,
		`class="btn btn--primary"`, `class="btn btn--outline"`,
		"targets.example.yaml (excerpt)",
	} {
		if !strings.Contains(html, s) {
			t.Errorf("landing page missing %q", s)
		}
	}
	if strings.Contains(html, "hero__note") {
		t.Error("pre-release note should be hidden for versions after " + lastReleaseWithoutTargets)
	}
}

func TestLandingPreReleaseNote(t *testing.T) {
	out := filepath.Join(t.TempDir(), "_site")
	if err := Build(Options{SrcDir: srcRoot, OutDir: out, Version: lastReleaseWithoutTargets}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readOut(t, out, "index.html"), "hero__note") {
		t.Errorf("pre-release note missing while the latest tag is %s", lastReleaseWithoutTargets)
	}
}

func TestTargetsExcerptComesFromExample(t *testing.T) {
	got, err := targetsExcerpt(srcRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "targets:\n  - name: app-db\n") {
		t.Errorf("excerpt should start with the first example target, got:\n%s", got)
	}
	if !strings.Contains(got, "backup_dir:") || strings.Contains(got, "etl-db") || strings.Contains(got, "storage:") {
		t.Errorf("excerpt should stop at the first target's backup_dir:\n%s", got)
	}
}
