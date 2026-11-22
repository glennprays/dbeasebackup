package build

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Nav is content/nav.json: the docs sidebar, in reading order.
type Nav struct {
	Default  string       `json:"default"`
	Sections []NavSection `json:"sections"`
}

// NavSection groups pages under a sidebar heading.
type NavSection struct {
	Title string   `json:"title"`
	Pages []string `json:"pages"` // slugs = content/docs/<slug>.md
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// LoadNav reads and decodes nav.json (unknown fields are rejected).
func LoadNav(path string) (*Nav, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var n Nav
	if err := dec.Decode(&n); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &n, nil
}

// Slugs returns every slug in reading order.
func (n *Nav) Slugs() []string {
	var out []string
	for _, s := range n.Sections {
		out = append(out, s.Pages...)
	}
	return out
}

// ListDocs returns the slugs of content/docs/*.md.
func ListDocs(docsDir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(docsDir, "*.md"))
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, strings.TrimSuffix(filepath.Base(f), ".md"))
	}
	sort.Strings(out)
	return out, nil
}

// ValidateNav fails when the nav and the markdown files disagree: a nav
// entry without a file, a file missing from the nav, a duplicate slug, an
// empty section, a malformed slug, or a default that is not in the nav.
func ValidateNav(n *Nav, files []string) error {
	var errs []error
	have := map[string]bool{}
	for _, f := range files {
		have[f] = true
	}
	seen := map[string]bool{}
	for _, s := range n.Sections {
		if strings.TrimSpace(s.Title) == "" {
			errs = append(errs, errors.New("nav: section with empty title"))
		}
		if len(s.Pages) == 0 {
			errs = append(errs, fmt.Errorf("nav: section %q has no pages", s.Title))
		}
		for _, slug := range s.Pages {
			if !slugRe.MatchString(slug) {
				errs = append(errs, fmt.Errorf("nav: invalid slug %q", slug))
			}
			if seen[slug] {
				errs = append(errs, fmt.Errorf("nav: duplicate slug %q", slug))
			}
			seen[slug] = true
			if !have[slug] {
				errs = append(errs, fmt.Errorf("nav: %q has no file content/docs/%s.md", slug, slug))
			}
		}
	}
	for _, f := range files {
		if !seen[f] {
			errs = append(errs, fmt.Errorf("nav: content/docs/%s.md is not listed in nav.json", f))
		}
	}
	if !seen[n.Default] {
		errs = append(errs, fmt.Errorf("nav: default %q is not a nav page", n.Default))
	}
	return errors.Join(errs...)
}
