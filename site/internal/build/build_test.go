package build

import (
	"os"
	"path/filepath"
	"testing"
)

// buildForTest renders the real site/ sources into a temp dir once per test.
func buildForTest(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "_site")
	if err := Build(Options{SrcDir: "../..", OutDir: out, Version: "v0.0.0-test"}); err != nil {
		t.Fatalf("Build: %v", err)
	}
	return out
}

func readOut(t *testing.T, out, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func TestBuildWritesCoreFiles(t *testing.T) {
	out := buildForTest(t)
	for _, f := range []string{"index.html", ".nojekyll"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
}
