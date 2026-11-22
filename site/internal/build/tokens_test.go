package build

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const srcRoot = "../.."

func TestEveryUsedVarIsDefined(t *testing.T) {
	tokens, err := LoadTokens(srcRoot)
	if err != nil {
		t.Fatal(err)
	}
	defined := map[string]bool{}
	for _, tok := range tokens {
		defined[tok.Name] = true
	}

	files, _ := filepath.Glob(filepath.Join(srcRoot, "styles", "*.css"))
	tmpls, _ := filepath.Glob(filepath.Join(srcRoot, "templates", "*.html"))
	parts, _ := filepath.Glob(filepath.Join(srcRoot, "templates", "partials", "*.html"))
	files = append(append(files, tmpls...), parts...)
	if len(files) == 0 {
		t.Fatal("no style or template files found")
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range UsedVars(string(data)) {
			if !defined[name] {
				t.Errorf("%s uses var(--%s), which tokens.css does not define", filepath.Base(f), name)
			}
		}
	}
}

func TestStyleOrderCoversEveryFile(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join(srcRoot, "styles", "*.css"))
	for _, f := range files {
		if !slices.Contains(StyleOrder, filepath.Base(f)) {
			t.Errorf("styles/%s is not listed in StyleOrder and would be dropped from the bundle", filepath.Base(f))
		}
	}
}

func TestFontsAndLicencesShip(t *testing.T) {
	want := []string{
		"ibm-plex-sans-latin-300-normal.woff2",
		"ibm-plex-sans-latin-400-normal.woff2",
		"ibm-plex-sans-latin-500-normal.woff2",
		"jetbrains-mono-latin-400-normal.woff2",
		"jetbrains-mono-latin-500-normal.woff2",
		"OFL-IBM-Plex.txt",
		"OFL-JetBrains-Mono.txt",
	}
	for _, f := range want {
		p := filepath.Join(srcRoot, "assets", "fonts", f)
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("missing %s", f)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("%s is empty", f)
		}
	}
	tokens, err := os.ReadFile(filepath.Join(srcRoot, "styles", "tokens.css"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range want {
		if strings.HasSuffix(f, ".woff2") && !strings.Contains(string(tokens), "/assets/fonts/"+f) {
			t.Errorf("tokens.css has no @font-face for %s", f)
		}
	}
}

func TestStyleguideRendersTokens(t *testing.T) {
	out := buildForTest(t)
	html := readOut(t, out, "_styleguide/index.html")
	for _, s := range []string{"--color-accent", "#b4441c", "--text-56", "--space-96", `content="noindex"`} {
		if !strings.Contains(html, s) {
			t.Errorf("style guide missing %q", s)
		}
	}
	if strings.Contains(html, "ZgotmplZ") {
		t.Error("style guide contains ZgotmplZ (html/template rejected an attribute value)")
	}
	css := readOut(t, out, "assets/css/site.css")
	if !strings.Contains(css, "--surface-canvas") || !strings.Contains(css, ".sg__chip") {
		t.Error("site.css bundle is missing tokens or style guide rules")
	}
}
