// Package build renders the public DBEaseBackup site (landing page + docs)
// into a static directory that GitHub Pages serves as-is.
package build

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	// DefaultSiteURL is the canonical origin of the published site.
	DefaultSiteURL = "https://dbease.glennprays.com"
	// RepoURL is the source repository, used for GitHub links.
	RepoURL = "https://github.com/glennprays/dbeasebackup"
)

// Options controls one build.
type Options struct {
	// SrcDir is the site/ directory (templates, content, assets).
	SrcDir string
	// OutDir receives the rendered site. It is removed and recreated.
	OutDir string
	// Version is shown in the header badge; empty hides it.
	Version string
	// SiteURL is the canonical origin without a trailing slash.
	SiteURL string
}

// Site holds data shared by every page.
type Site struct {
	URL     string
	Version string
	Repo    string
	// assets maps "/assets/..." paths to a short content hash.
	assets map[string]string
}

// Page is the template data for one rendered HTML page.
type Page struct {
	Site        *Site
	Title       string
	Description string
	// Path is the URL path of the page, e.g. "/" or "/docs/retention/".
	Path    string
	NoIndex bool
	// Section marks the active top-nav item ("home" or "docs").
	Section string
	// Data carries page-type specific values.
	Data any
}

// Canonical returns the absolute canonical URL of the page.
func (p *Page) Canonical() string { return p.Site.URL + p.Path }

// Builder renders a site from Options.
type Builder struct {
	opts  Options
	site  *Site
	pages []*Page
	nav   *Nav
	docs  map[string]*Doc
}

// Build renders the full site into opts.OutDir.
func Build(opts Options) error {
	if opts.SiteURL == "" {
		opts.SiteURL = DefaultSiteURL
	}
	opts.SiteURL = strings.TrimRight(opts.SiteURL, "/")
	b := &Builder{
		opts: opts,
		site: &Site{URL: opts.SiteURL, Version: opts.Version, Repo: RepoURL, assets: map[string]string{}},
	}
	return b.run()
}

func (b *Builder) run() error {
	if err := os.RemoveAll(b.opts.OutDir); err != nil {
		return fmt.Errorf("clean output: %w", err)
	}
	if err := os.MkdirAll(b.opts.OutDir, 0o755); err != nil {
		return err
	}
	if err := b.copyAssets(); err != nil {
		return fmt.Errorf("copy assets: %w", err)
	}
	if err := b.bundleCSS(); err != nil {
		return fmt.Errorf("bundle css: %w", err)
	}

	if err := b.renderStyleguide(); err != nil {
		return err
	}
	if err := b.loadDocs(); err != nil {
		return err
	}
	// The index only needs the parsed docs; pages embed its hashed URL.
	if err := b.writeSearchIndex(); err != nil {
		return err
	}
	if err := b.renderDocs(); err != nil {
		return err
	}

	if err := b.renderLanding(); err != nil {
		return err
	}
	// After all pages: 404, sitemap (needs the page list), robots, llms.txt.
	if err := b.writeExtras(); err != nil {
		return err
	}

	// GitHub Pages must not run Jekyll over the output.
	if err := os.WriteFile(filepath.Join(b.opts.OutDir, ".nojekyll"), nil, 0o644); err != nil {
		return err
	}
	// Last: every internal link and anchor in the output must resolve.
	return CheckLinks(b.opts.OutDir)
}

// copyAssets copies site/assets to <out>/assets and records content hashes
// used for cache-busting query strings.
func (b *Builder) copyAssets() error {
	src := filepath.Join(b.opts.SrcDir, "assets")
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		dst := filepath.Join(b.opts.OutDir, "assets", rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return b.writeAsset("/assets/"+filepath.ToSlash(rel), data)
	})
}

// writeAsset writes a file under OutDir at the URL path urlPath and records
// its hash.
func (b *Builder) writeAsset(urlPath string, data []byte) error {
	dst := filepath.Join(b.opts.OutDir, filepath.FromSlash(strings.TrimPrefix(urlPath, "/")))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	b.site.assets[urlPath] = hex.EncodeToString(sum[:])[:10]
	return os.WriteFile(dst, data, 0o644)
}

func (b *Builder) funcs() template.FuncMap {
	return template.FuncMap{
		// dict builds a map for passing named arguments to partials.
		"dict": func(kv ...any) (map[string]any, error) {
			if len(kv)%2 != 0 {
				return nil, fmt.Errorf("dict: odd number of arguments")
			}
			m := make(map[string]any, len(kv)/2)
			for i := 0; i < len(kv); i += 2 {
				k, ok := kv[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict: key %v is not a string", kv[i])
				}
				m[k] = kv[i+1]
			}
			return m, nil
		},
		// code highlights a snippet for the "code" partial.
		"code": NewCodeBlock,
		// asset returns a cache-busted URL for a file under /assets/.
		"asset": func(p string) (string, error) {
			u := "/assets/" + strings.TrimPrefix(p, "/")
			h, ok := b.site.assets[u]
			if !ok {
				return "", fmt.Errorf("unknown asset %s", u)
			}
			return u + "?v=" + h, nil
		},
		// searchIndex returns the cache-busted URL of the docs search index.
		"searchIndex": func() (string, error) {
			h, ok := b.site.assets[SearchIndexPath]
			if !ok {
				return "", fmt.Errorf("search index not written yet")
			}
			return SearchIndexPath + "?v=" + h, nil
		},
	}
}

// parse loads base + partial templates plus one page template.
func (b *Builder) parse(page string) (*template.Template, error) {
	dir := filepath.Join(b.opts.SrcDir, "templates")
	files := []string{filepath.Join(dir, "base.html")}
	partials, err := filepath.Glob(filepath.Join(dir, "partials", "*.html"))
	if err != nil {
		return nil, err
	}
	files = append(files, partials...)
	files = append(files, filepath.Join(dir, page))
	return template.New("base.html").Funcs(b.funcs()).ParseFiles(files...)
}

// renderPage renders a page template to the file matching p.Path.
func (b *Builder) renderPage(tmplName string, p *Page) error {
	t, err := b.parse(tmplName)
	if err != nil {
		return fmt.Errorf("parse %s: %w", tmplName, err)
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "base.html", p); err != nil {
		return fmt.Errorf("render %s: %w", p.Path, err)
	}
	if err := b.writeHTML(p.Path, buf.Bytes()); err != nil {
		return err
	}
	b.pages = append(b.pages, p)
	return nil
}

// writeHTML writes rendered HTML for a URL path. Paths ending in "/" become
// <path>/index.html; paths ending in ".html" are written as-is.
func (b *Builder) writeHTML(urlPath string, data []byte) error {
	rel := strings.TrimPrefix(urlPath, "/")
	if rel == "" || strings.HasSuffix(rel, "/") {
		rel = path.Join(rel, "index.html")
	}
	dst := filepath.Join(b.opts.OutDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
