package build

import (
	"errors"
	"fmt"
	"html"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	refAttr = regexp.MustCompile(`\s(?:href|src)="([^"]*)"`)
	idAttr  = regexp.MustCompile(`\sid="([^"]*)"`)
)

// CheckLinks verifies every internal href/src in the HTML files under dir:
// the target file must exist and any #fragment must match an id on the
// target page. External (scheme or protocol-relative) links are skipped.
func CheckLinks(dir string) error {
	pages := map[string]string{} // url path of file -> contents
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".html") {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		pages["/"+filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		return err
	}

	ids := map[string]map[string]bool{}
	idsOf := func(file string) map[string]bool {
		if m, ok := ids[file]; ok {
			return m
		}
		m := map[string]bool{}
		for _, mm := range idAttr.FindAllStringSubmatch(pages[file], -1) {
			m[html.UnescapeString(mm[1])] = true
		}
		ids[file] = m
		return m
	}

	var problems []string
	files := make([]string, 0, len(pages))
	for f := range pages {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, from := range files {
		for _, m := range refAttr.FindAllStringSubmatch(pages[from], -1) {
			raw := html.UnescapeString(m[1])
			if raw == "" || strings.HasPrefix(raw, "//") {
				continue
			}
			u, err := url.Parse(raw)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s: unparseable link %q", from, raw))
				continue
			}
			if u.Scheme != "" {
				continue // http(s), mailto, data, ...
			}
			target := from
			if u.Path != "" {
				p := u.Path
				if !strings.HasPrefix(p, "/") {
					p = path.Join(path.Dir(from), p)
				}
				target = resolveFile(dir, p)
				if target == "" {
					problems = append(problems, fmt.Sprintf("%s: broken link %q", from, raw))
					continue
				}
			}
			if u.Fragment != "" {
				if _, isHTML := pages[target]; !isHTML || !idsOf(target)[u.Fragment] {
					problems = append(problems, fmt.Sprintf("%s: missing anchor %q", from, raw))
				}
			}
		}
	}
	if len(problems) > 0 {
		return errors.New("link check failed:\n  " + strings.Join(problems, "\n  "))
	}
	return nil
}

// resolveFile maps a URL path to the file GitHub Pages would serve, as a
// "/"-rooted path, or "" when nothing exists.
func resolveFile(dir, p string) string {
	candidates := []string{p}
	if strings.HasSuffix(p, "/") {
		candidates = []string{p + "index.html"}
	} else if path.Ext(p) == "" {
		candidates = append(candidates, p+".html", p+"/index.html")
	}
	for _, c := range candidates {
		info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(c, "/"))))
		if err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}
