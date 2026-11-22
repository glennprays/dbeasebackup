package build

import (
	"encoding/xml"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestSitemapURLsExist(t *testing.T) {
	out := buildForTest(t)
	var set sitemapSet
	if err := xml.Unmarshal([]byte(readOut(t, out, "sitemap.xml")), &set); err != nil {
		t.Fatal(err)
	}
	if len(set.URLs) < 20 {
		t.Fatalf("sitemap has %d URLs, want the landing page plus every doc", len(set.URLs))
	}
	seen := map[string]bool{}
	for _, u := range set.URLs {
		p, ok := strings.CutPrefix(u.Loc, DefaultSiteURL)
		if !ok {
			t.Errorf("sitemap URL %s is not on %s", u.Loc, DefaultSiteURL)
			continue
		}
		seen[p] = true
		if resolveFile(out, p) == "" {
			t.Errorf("sitemap lists %s, which does not exist in the output", u.Loc)
		}
	}
	for _, p := range []string{"/", "/docs/introduction/", "/docs/troubleshooting/"} {
		if !seen[p] {
			t.Errorf("sitemap missing %s", p)
		}
	}
	for _, p := range []string{"/_styleguide/", "/404.html", "/docs/"} {
		if seen[p] {
			t.Errorf("sitemap must not list %s", p)
		}
	}
	if !strings.Contains(readOut(t, out, "robots.txt"), "Sitemap: "+DefaultSiteURL+"/sitemap.xml") {
		t.Error("robots.txt does not point at the sitemap")
	}
}

var (
	canonicalRe = regexp.MustCompile(`<link rel="canonical" href="https://dbease\.glennprays\.com/[^"]*">`)
	descRe      = regexp.MustCompile(`<meta name="description" content="[^"]{20,}">`)
	ogImageRe   = regexp.MustCompile(`<meta property="og:image" content="https://dbease\.glennprays\.com/assets/img/og-image\.png">`)
)

func TestEveryPageHasMeta(t *testing.T) {
	out := buildForTest(t)
	n := 0
	err := filepath.WalkDir(out, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".html") {
			return err
		}
		rel, _ := filepath.Rel(out, p)
		if rel == filepath.Join("docs", "index.html") {
			return nil // meta-refresh redirect, checked in TestDocsRendered
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		html := string(b)
		n++
		for name, re := range map[string]*regexp.Regexp{"canonical": canonicalRe, "description": descRe, "og:image": ogImageRe} {
			if !re.MatchString(html) {
				t.Errorf("%s: missing %s", rel, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n < 22 {
		t.Errorf("checked only %d pages", n)
	}
	if _, err := os.Stat(filepath.Join(out, "favicon.ico")); err != nil {
		t.Error("missing /favicon.ico")
	}
	if _, err := os.Stat(filepath.Join(out, "assets", "img", "og-image.png")); err != nil {
		t.Error("missing og-image.png")
	}
}

func TestNotFoundPage(t *testing.T) {
	out := buildForTest(t)
	html := readOut(t, out, "404.html")
	for _, s := range []string{`content="noindex"`, `href="/docs/"`, `href="/docs/retention/"`, "/assets/css/site.css?v="} {
		if !strings.Contains(html, s) {
			t.Errorf("404 page missing %q", s)
		}
	}
}

func TestLLMsTxt(t *testing.T) {
	out := buildForTest(t)
	txt := readOut(t, out, "llms.txt")
	if !strings.HasPrefix(txt, "# DBEaseBackup\n\n> ") {
		t.Errorf("llms.txt should start with an H1 and a blockquote summary:\n%s", txt[:min(200, len(txt))])
	}
	files, _ := ListDocs(filepath.Join(srcRoot, "content", "docs"))
	for _, slug := range files {
		if !strings.Contains(txt, "("+DefaultSiteURL+DocURL(slug)+"): ") {
			t.Errorf("llms.txt missing %s", slug)
		}
	}
}
