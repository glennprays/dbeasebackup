package build

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// render404 writes /404.html, which GitHub Pages serves for unknown paths.
// All asset and link URLs are root-relative, so it works at any depth.
func (b *Builder) render404() error {
	return b.renderPage("404.html", &Page{
		Site:        b.site,
		Title:       "Page not found · DBEaseBackup",
		Description: "This page does not exist. Find what you need in the DBEaseBackup documentation.",
		Path:        "/404.html",
		NoIndex:     true,
		Data:        b.navGroups(""),
	})
}

type sitemapURL struct {
	Loc string `xml:"loc"`
}

type sitemapSet struct {
	XMLName xml.Name     `xml:"urlset"`
	NS      string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

// SitemapPaths returns the indexable page paths in render order.
func (b *Builder) SitemapPaths() []string {
	var out []string
	for _, p := range b.pages {
		if !p.NoIndex {
			out = append(out, p.Path)
		}
	}
	return out
}

func (b *Builder) writeSitemap() error {
	set := sitemapSet{NS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	for _, p := range b.SitemapPaths() {
		set.URLs = append(set.URLs, sitemapURL{Loc: b.site.URL + p})
	}
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(set); err != nil {
		return err
	}
	buf.WriteByte('\n')
	return b.writeFile("sitemap.xml", buf.Bytes())
}

func (b *Builder) writeRobots() error {
	return b.writeFile("robots.txt", []byte(fmt.Sprintf("User-agent: *\nAllow: /\n\nSitemap: %s/sitemap.xml\n", b.site.URL)))
}

// writeLLMs writes /llms.txt (https://llmstxt.org): a plain-text map of the
// docs for language models and other tools, generated from the nav.
func (b *Builder) writeLLMs() error {
	var sb strings.Builder
	sb.WriteString("# DBEaseBackup\n\n")
	sb.WriteString("> Scheduled database backups to Google Drive or S3. One Go process backs up each target on cron (PostgreSQL today via pg_dump or pg_basebackup; more engines planned), verifies the result, uploads it and enforces retention. MIT licensed.\n\n")
	fmt.Fprintf(&sb, "Source: %s\nImage: docker.io/glennprays/dbeasebackup\n", RepoURL)
	if b.site.Version != "" {
		fmt.Fprintf(&sb, "Latest release: %s\n", b.site.Version)
	}
	for _, s := range b.nav.Sections {
		fmt.Fprintf(&sb, "\n## %s\n\n", s.Title)
		for _, slug := range s.Pages {
			d := b.docs[slug]
			fmt.Fprintf(&sb, "- [%s](%s%s): %s\n", d.Title, b.site.URL, DocURL(slug), d.Description)
		}
	}
	return b.writeFile("llms.txt", []byte(sb.String()))
}

// writeRootFavicon copies the .ico to /favicon.ico, which browsers request
// without reading any markup.
func (b *Builder) writeRootFavicon() error {
	data, err := os.ReadFile(filepath.Join(b.opts.SrcDir, "assets", "img", "favicon.ico"))
	if err != nil {
		return err
	}
	return b.writeFile("favicon.ico", data)
}

func (b *Builder) writeFile(rel string, data []byte) error {
	return os.WriteFile(filepath.Join(b.opts.OutDir, filepath.FromSlash(rel)), data, 0o644)
}

func (b *Builder) writeExtras() error {
	for _, f := range []func() error{b.render404, b.writeSitemap, b.writeRobots, b.writeLLMs, b.writeRootFavicon} {
		if err := f(); err != nil {
			return err
		}
	}
	return nil
}
