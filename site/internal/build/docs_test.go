package build

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestValidateNav(t *testing.T) {
	ok := &Nav{Default: "a", Sections: []NavSection{{Title: "S", Pages: []string{"a", "b"}}}}
	tests := []struct {
		name    string
		nav     *Nav
		files   []string
		wantErr string
	}{
		{"valid", ok, []string{"a", "b"}, ""},
		{"missing file", ok, []string{"a"}, `"b" has no file`},
		{"orphan file", ok, []string{"a", "b", "c"}, "c.md is not listed"},
		{"duplicate slug", &Nav{Default: "a", Sections: []NavSection{{Title: "S", Pages: []string{"a"}}, {Title: "T", Pages: []string{"a"}}}}, []string{"a"}, `duplicate slug "a"`},
		{"bad default", &Nav{Default: "zzz", Sections: ok.Sections}, []string{"a", "b"}, `default "zzz"`},
		{"empty section", &Nav{Default: "a", Sections: []NavSection{{Title: "S", Pages: []string{"a"}}, {Title: "E"}}}, []string{"a"}, `"E" has no pages`},
		{"bad slug", &Nav{Default: "a", Sections: []NavSection{{Title: "S", Pages: []string{"a", "Bad_Slug"}}}}, []string{"a", "Bad_Slug"}, "invalid slug"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNav(tt.nav, tt.files)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestRealNavMatchesContent(t *testing.T) {
	nav, err := LoadNav(filepath.Join(srcRoot, "content", "nav.json"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := ListDocs(filepath.Join(srcRoot, "content", "docs"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateNav(nav, files); err != nil {
		t.Fatal(err)
	}
}

func renderSample(t *testing.T) *Doc {
	t.Helper()
	src, err := os.ReadFile("testdata/sample.md")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := RenderDoc("sample", src)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestRenderDocHeadingsAndTOC(t *testing.T) {
	doc := renderSample(t)
	if doc.Title != "Sample page" || doc.Description == "" {
		t.Fatalf("front matter not parsed: %+v", doc)
	}
	want := []Heading{
		{ID: "first-section", Text: "First section", Level: 2},
		{ID: "nested-heading", Text: "Nested heading", Level: 3},
		{ID: "second-section", Text: "Second section", Level: 2},
	}
	if len(doc.TOC) != len(want) {
		t.Fatalf("TOC = %+v", doc.TOC)
	}
	for i := range want {
		if doc.TOC[i] != want[i] {
			t.Errorf("TOC[%d] = %+v, want %+v", i, doc.TOC[i], want[i])
		}
	}
	html := string(doc.HTML)
	for _, s := range []string{
		`<h2 id="first-section">`,
		`href="#first-section"`,
		`callout callout--warn`,
		`<p class="callout__label">Warning</p>`,
		`<div class="table-wrap"><table>`,
		`<span class="code__lang">targets.yaml</span>`,
		`<blockquote>`,
	} {
		if !strings.Contains(html, s) {
			t.Errorf("rendered HTML missing %q", s)
		}
	}
	if strings.Contains(html, "[!WARNING]") {
		t.Error("callout marker leaked into output")
	}
	if len(doc.Blocks) != 4 || !strings.Contains(doc.Blocks[0].Text, "Intro paragraph") || !strings.Contains(doc.Blocks[2].Text, "name: app-db") {
		t.Errorf("text blocks = %+v", doc.Blocks)
	}
	// Table cells must not run together in search text.
	if !strings.Contains(doc.Blocks[1].Text, "Field Default retries 2") {
		t.Errorf("table text not separated: %q", doc.Blocks[1].Text)
	}
}

func TestRenderDocGolden(t *testing.T) {
	doc := renderSample(t)
	golden := "testdata/sample.golden.html"
	if *update {
		if err := os.WriteFile(golden, []byte(doc.HTML), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run go test ./... -update to create it)", err)
	}
	if string(doc.HTML) != string(want) {
		t.Errorf("rendered HTML differs from %s; rerun with -update after checking the diff\n--- got ---\n%s", golden, doc.HTML)
	}
}

func TestRenderDocRejects(t *testing.T) {
	cases := map[string]string{
		"no front matter": "# Hi\n",
		"no description":  "---\ntitle: X\n---\nbody\n",
		"h1 in body":      "---\ntitle: X\ndescription: Y\n---\n# Again\n",
	}
	for name, src := range cases {
		if _, err := RenderDoc("x", []byte(src)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestDocsRendered(t *testing.T) {
	out := buildForTest(t)
	html := readOut(t, out, "docs/introduction/index.html")
	for _, s := range []string{
		`aria-current="page"`,
		"On this page",
		`rel="next"`,
		RepoURL + "/edit/main/site/content/docs/introduction.md",
		`<h1 id="doc-title">Introduction</h1>`,
	} {
		if !strings.Contains(html, s) {
			t.Errorf("introduction page missing %q", s)
		}
	}
	redirect := readOut(t, out, "docs/index.html")
	if !strings.Contains(redirect, `url=/docs/introduction/`) {
		t.Errorf("/docs/ redirect wrong: %s", redirect)
	}
}
