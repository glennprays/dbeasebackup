package build

import (
	"bytes"
	"strings"
	"testing"
)

// newTestBuilder returns a Builder with assets copied, ready to parse and
// execute individual partials.
func newTestBuilder(t *testing.T) *Builder {
	t.Helper()
	b := &Builder{
		opts: Options{SrcDir: srcRoot, OutDir: t.TempDir(), SiteURL: DefaultSiteURL},
		site: &Site{URL: DefaultSiteURL, Version: "v1.2.3", Repo: RepoURL, assets: map[string]string{}},
	}
	if err := b.copyAssets(); err != nil {
		t.Fatal(err)
	}
	if err := b.bundleCSS(); err != nil {
		t.Fatal(err)
	}
	return b
}

func execPartial(t *testing.T, b *Builder, name string, data any) string {
	t.Helper()
	tmpl, err := b.parse("styleguide.html")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		t.Fatalf("execute %s: %v", name, err)
	}
	return buf.String()
}

func TestComponentPartials(t *testing.T) {
	b := newTestBuilder(t)
	m := func(kv ...any) map[string]any {
		out := map[string]any{}
		for i := 0; i < len(kv); i += 2 {
			out[kv[i].(string)] = kv[i+1]
		}
		return out
	}
	cb, err := NewCodeBlock("yaml", "targets.yaml", "name: app-db\nverify: true\n")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		data any
		want []string
	}{
		{"btn", m("Href", "/docs/", "Label", "Read the docs", "Kind", "primary"), []string{`class="btn btn--primary"`, `href="/docs/"`, "Read the docs"}},
		{"btn", m("Href", "https://x", "Label", "GitHub", "Kind", "outline"), []string{"btn--outline"}},
		{"link-more", m("Href", "/docs/retention/", "Label", "Retention"), []string{`class="link-more"`, `aria-hidden="true"`}},
		{"tag", m("Label", "ok", "Kind", "ok"), []string{`class="tag tag--ok"`, ">ok<"}},
		{"tag", m("Label", "dump"), []string{`class="tag"`}},
		{"copy-cmd", m("Cmd", "docker run x"), []string{`data-copy-source>docker run x<`, `data-copy hidden`}},
		{"code", cb, []string{`class="chroma"`, "targets.yaml", `class="nt"`, "data-copy hidden"}},
		{"callout", m("Kind", "warn", "Body", "careful"), []string{"callout--warn", "Warning", "careful"}},
		{"callout", m("Kind", "note", "Body", "fyi"), []string{"callout--note", ">Note<"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := execPartial(t, b, tt.name, tt.data)
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("%s output missing %q\n%s", tt.name, w, got)
				}
			}
		})
	}
}

func TestChromeShowsVersionAndActiveSection(t *testing.T) {
	b := newTestBuilder(t)
	p := &Page{Site: b.site, Path: "/docs/x/", Section: "docs"}
	header := execPartial(t, b, "site-header", p)
	for _, w := range []string{"v1.2.3", RepoURL + "/releases/tag/v1.2.3", `aria-current="page"`, "data-search-open"} {
		if !strings.Contains(header, w) {
			t.Errorf("header missing %q", w)
		}
	}
	b.site.Version = ""
	home := execPartial(t, b, "site-header", &Page{Site: b.site, Path: "/", Section: "home"})
	if strings.Contains(home, "tag--version") || strings.Contains(home, "data-search-open") {
		t.Error("home header should hide the empty version badge and the search trigger")
	}
	footer := execPartial(t, b, "site-footer", p)
	if !strings.Contains(footer, "hub.docker.com/r/glennprays/dbeasebackup") {
		t.Error("footer missing Docker Hub link")
	}
}

func TestStyleguideShowsEveryComponent(t *testing.T) {
	out := buildForTest(t)
	html := readOut(t, out, "_styleguide/index.html")
	for _, cls := range []string{"btn--primary", "btn--outline", "link-more", "tag--ok", "tag--warn", "tag--fail",
		`class="cmd"`, `class="code"`, "callout--note", "callout--warn", "callout--fail", `class="ledger"`, `class="envlist"`,
		"site-header", "site-footer"} {
		if !strings.Contains(html, cls) {
			t.Errorf("style guide missing component %q", cls)
		}
	}
}

func TestHighlightUnknownLanguageEscapes(t *testing.T) {
	h, err := Highlight("no-such-lang", "<b>&</b>")
	if err != nil {
		t.Fatal(err)
	}
	if string(h) != "&lt;b&gt;&amp;&lt;/b&gt;" {
		t.Errorf("Highlight escaped = %q", h)
	}
}
