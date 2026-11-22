package build

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCheckLinks(t *testing.T) {
	base := map[string]string{
		"index.html":         `<a href="/docs/a/">a</a> <a href="https://example.com/x">ext</a> <a href="mailto:x@y">m</a> <img src="/assets/x.png?v=1">`,
		"docs/a/index.html":  `<h2 id="sec">S</h2><a href="#sec">self</a> <a href="/docs/b/#top">b</a> <a href="../b/">rel</a>`,
		"docs/b/index.html":  `<p id="top">top</p>`,
		"assets/x.png":       "png",
		"docs/index.html":    `<a href="/docs/a/">x</a>`,
		"docs/a/plain.html":  `<a href="/docs/a/plain">ext-less</a>`,
		"docs/a/plain/x.txt": "",
	}
	if err := CheckLinks(writeFiles(t, base)); err != nil {
		t.Fatalf("valid site failed: %v", err)
	}

	cases := map[string]struct {
		page, want string
	}{
		"broken page":    {`<a href="/docs/missing/">x</a>`, `broken link "/docs/missing/"`},
		"missing anchor": {`<a href="/docs/b/#nope">x</a>`, `missing anchor "/docs/b/#nope"`},
		"self anchor":    {`<a href="#ghost">x</a>`, `missing anchor "#ghost"`},
		"missing asset":  {`<img src="/assets/none.png">`, `broken link "/assets/none.png"`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			files := map[string]string{"docs/b/index.html": `<p id="top"></p>`, "index.html": c.page}
			err := CheckLinks(writeFiles(t, files))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want containing %q", err, c.want)
			}
		})
	}
}

// configKeys returns every mapstructure key in the app's config package.
func configKeys(t *testing.T, file string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(srcRoot, "..", "config", file))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, m := range regexp.MustCompile("mapstructure:\"([^\"]+)\"").FindAllStringSubmatch(string(data), -1) {
		out = append(out, m[1])
	}
	return out
}

// tableKeys returns the first-column `code` values of markdown table rows.
func tableKeys(t *testing.T, doc string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(srcRoot, "content", "docs", doc))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, m := range regexp.MustCompile("(?m)^\\| `([^`]+)` \\|").FindAllStringSubmatch(string(data), -1) {
		out[m[1]] = true
	}
	return out
}

func TestEnvVarDocsMatchConfig(t *testing.T) {
	keys := configKeys(t, "config.go")
	if len(keys) < 20 {
		t.Fatalf("found only %d config keys; did config.go move?", len(keys))
	}
	documented := tableKeys(t, "environment-variables.md")
	inConfig := map[string]bool{}
	for _, k := range keys {
		inConfig[k] = true
		if !documented[k] {
			t.Errorf("config.go reads %s, but environment-variables.md does not document it", k)
		}
	}
	for k := range documented {
		if !inConfig[k] {
			t.Errorf("environment-variables.md documents %s, which config.go does not read", k)
		}
	}
}

func TestTargetsFieldDocsMatchConfig(t *testing.T) {
	documented := tableKeys(t, "targets-file.md")
	keys := configKeys(t, "targets.go")
	want := map[string]bool{}
	for _, k := range keys {
		switch k {
		case "targets":
			continue // document root, not a field
		case "storage":
			continue // documented as storage.folder_id / storage.prefix
		case "folder_id", "prefix":
			k = "storage." + k
		}
		want[k] = true
		if !documented[k] {
			t.Errorf("targets.go has field %s, but targets-file.md does not document it", k)
		}
	}
	// Only the Fields table uses `field` rows plus the filename table, whose
	// first column is a mode name.
	for k := range documented {
		if !want[k] && k != "dump" && k != "basebackup" {
			t.Errorf("targets-file.md documents %s, which targets.go does not define", k)
		}
	}
}
