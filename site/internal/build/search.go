package build

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// SearchIndexPath is where the docs search index is published.
const SearchIndexPath = "/docs/search-index.json"

// maxSnippet caps the text stored per entry; search matches only this text.
const maxSnippet = 600

// SearchEntry is one searchable block: a page intro or one heading's text.
// Short JSON keys keep the index small.
type SearchEntry struct {
	Title   string `json:"t"` // page title
	Section string `json:"s"` // nav section
	Heading string `json:"h"` // heading text, empty for the page intro
	URL     string `json:"u"` // page URL plus #fragment for headings
	Text    string `json:"x"` // plain text, truncated
}

// BuildSearchIndex returns entries for every doc in nav order.
func (b *Builder) BuildSearchIndex() []SearchEntry {
	var out []SearchEntry
	for _, slug := range b.nav.Slugs() {
		doc := b.docs[slug]
		for i, blk := range doc.Blocks {
			text := blk.Text
			if i == 0 {
				// The intro block also carries the description.
				text = strings.TrimSpace(doc.Description + " " + text)
			}
			if text == "" && blk.Heading == "" {
				continue
			}
			url := DocURL(slug)
			if blk.HeadingID != "" {
				url += "#" + blk.HeadingID
			}
			out = append(out, SearchEntry{
				Title:   doc.Title,
				Section: doc.Section,
				Heading: blk.Heading,
				URL:     url,
				Text:    truncate(text, maxSnippet),
			})
		}
	}
	return out
}

// truncate cuts s to at most n bytes on a rune and word boundary.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	if i := strings.LastIndexByte(s[:cut], ' '); i > n/2 {
		cut = i
	}
	return s[:cut] + "…"
}

func (b *Builder) writeSearchIndex() error {
	data, err := json.Marshal(b.BuildSearchIndex())
	if err != nil {
		return err
	}
	return b.writeAsset(SearchIndexPath, data)
}
