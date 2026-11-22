package build

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
)

// NavLink is one sidebar or prev/next link.
type NavLink struct {
	Slug   string
	Title  string
	URL    string
	Active bool
}

// NavGroup is a sidebar section with its links.
type NavGroup struct {
	Title string
	Links []NavLink
}

// DocPage is the template data for templates/doc.html.
type DocPage struct {
	Doc     *Doc
	Groups  []NavGroup
	Prev    *NavLink
	Next    *NavLink
	EditURL string
}

// DocURL is the public URL path of a doc slug.
func DocURL(slug string) string { return "/docs/" + slug + "/" }

func (b *Builder) contentDir() string { return filepath.Join(b.opts.SrcDir, "content") }

// loadDocs validates nav.json against content/docs and renders every doc.
func (b *Builder) loadDocs() error {
	nav, err := LoadNav(filepath.Join(b.contentDir(), "nav.json"))
	if err != nil {
		return err
	}
	files, err := ListDocs(filepath.Join(b.contentDir(), "docs"))
	if err != nil {
		return err
	}
	if err := ValidateNav(nav, files); err != nil {
		return err
	}
	b.nav = nav
	b.docs = map[string]*Doc{}
	for _, s := range nav.Sections {
		for _, slug := range s.Pages {
			src, err := os.ReadFile(filepath.Join(b.contentDir(), "docs", slug+".md"))
			if err != nil {
				return err
			}
			doc, err := RenderDoc(slug, src)
			if err != nil {
				return err
			}
			doc.Section = s.Title
			b.docs[slug] = doc
		}
	}
	return nil
}

// renderDocs writes /docs/<slug>/ for every doc plus the /docs/ redirect.
func (b *Builder) renderDocs() error {
	order := b.nav.Slugs()
	for i, slug := range order {
		doc := b.docs[slug]
		data := DocPage{
			Doc:     doc,
			Groups:  b.navGroups(slug),
			EditURL: fmt.Sprintf("%s/edit/main/site/content/docs/%s.md", RepoURL, slug),
		}
		if i > 0 {
			data.Prev = b.link(order[i-1])
		}
		if i < len(order)-1 {
			data.Next = b.link(order[i+1])
		}
		if err := b.renderPage("doc.html", &Page{
			Site:        b.site,
			Title:       doc.Title + " · DBEaseBackup docs",
			Description: doc.Description,
			Path:        DocURL(slug),
			Section:     "docs",
			Data:        data,
		}); err != nil {
			return err
		}
	}
	return b.writeRedirect("/docs/", DocURL(b.nav.Default))
}

func (b *Builder) link(slug string) *NavLink {
	return &NavLink{Slug: slug, Title: b.docs[slug].Title, URL: DocURL(slug)}
}

func (b *Builder) navGroups(active string) []NavGroup {
	groups := make([]NavGroup, 0, len(b.nav.Sections))
	for _, s := range b.nav.Sections {
		g := NavGroup{Title: s.Title}
		for _, slug := range s.Pages {
			l := *b.link(slug)
			l.Active = slug == active
			g.Links = append(g.Links, l)
		}
		groups = append(groups, g)
	}
	return groups
}

var redirectTmpl = template.Must(template.New("redirect").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Redirecting to {{.To}}</title>
<meta name="robots" content="noindex">
<link rel="canonical" href="{{.Canonical}}">
<meta http-equiv="refresh" content="0; url={{.To}}">
</head>
<body>
<p><a href="{{.To}}">Continue to the documentation</a></p>
</body>
</html>
`))

// writeRedirect writes a static meta-refresh page at from pointing to to.
// GitHub Pages has no server-side redirects.
func (b *Builder) writeRedirect(from, to string) error {
	var sb strings.Builder
	if err := redirectTmpl.Execute(&sb, map[string]string{"To": to, "Canonical": b.site.URL + to}); err != nil {
		return err
	}
	return b.writeHTML(from, []byte(sb.String()))
}
