package build

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Doc is one rendered documentation page.
type Doc struct {
	Slug        string
	Title       string
	Description string
	Section     string // nav section title, set by the nav
	HTML        template.HTML
	TOC         []Heading
	// Blocks holds plain text per heading, used for the search index.
	Blocks []TextBlock
}

// Heading is an h2/h3 entry in the on-page table of contents.
type Heading struct {
	ID    string
	Text  string
	Level int
}

// TextBlock is the plain text under one heading (ID "" = intro).
type TextBlock struct {
	HeadingID string
	Heading   string
	Text      string
}

// ParseFrontMatter splits "---\nkey: value\n---\n" from the body.
func ParseFrontMatter(src []byte) (map[string]string, []byte, error) {
	s := string(src)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return nil, nil, errors.New("missing front matter (file must start with ---)")
	}
	end := strings.Index(s[4:], "\n---\n")
	if end < 0 {
		return nil, nil, errors.New("unterminated front matter")
	}
	meta := map[string]string{}
	for _, line := range strings.Split(s[4:4+end], "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			return nil, nil, fmt.Errorf("bad front matter line %q", line)
		}
		meta[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"`)
	}
	return meta, []byte(s[4+end+5:]), nil
}

// newMarkdown builds the goldmark pipeline used for every doc.
func newMarkdown() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(util.Prioritized(calloutTransformer{}, 100)),
		),
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(util.Prioritized(&docRenderer{}, 100)),
		),
	)
}

// RenderDoc parses a markdown file (with front matter) into a Doc.
func RenderDoc(slug string, src []byte) (*Doc, error) {
	meta, body, err := ParseFrontMatter(src)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", slug, err)
	}
	doc := &Doc{Slug: slug, Title: meta["title"], Description: meta["description"]}
	if doc.Title == "" || doc.Description == "" {
		return nil, fmt.Errorf("%s: front matter needs title and description", slug)
	}

	md := newMarkdown()
	root := md.Parser().Parse(text.NewReader(body))

	// Collect TOC + per-heading text; reject h1 (the title is the only h1).
	current := TextBlock{}
	var walkErr error
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		if h, ok := n.(*ast.Heading); ok {
			if h.Level == 1 {
				walkErr = fmt.Errorf("%s: use front matter title instead of a level-1 heading", slug)
				break
			}
			id := headingID(h)
			txt := nodeText(h, body)
			if h.Level <= 3 {
				doc.TOC = append(doc.TOC, Heading{ID: id, Text: txt, Level: h.Level})
			}
			doc.Blocks = append(doc.Blocks, current)
			current = TextBlock{HeadingID: id, Heading: txt}
			continue
		}
		if t := strings.TrimSpace(nodeText(n, body)); t != "" {
			if current.Text != "" {
				current.Text += " "
			}
			current.Text += t
		}
	}
	if walkErr != nil {
		return nil, walkErr
	}
	doc.Blocks = append(doc.Blocks, current)

	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, body, root); err != nil {
		return nil, fmt.Errorf("%s: render: %w", slug, err)
	}
	out := buf.String()
	// Tables scroll horizontally inside a wrapper on narrow screens.
	out = strings.ReplaceAll(out, "<table>", `<div class="table-wrap"><table>`)
	out = strings.ReplaceAll(out, "</table>", "</table></div>")
	doc.HTML = template.HTML(out)
	return doc, nil
}

func headingID(h *ast.Heading) string {
	if v, ok := h.AttributeString("id"); ok {
		if b, ok := v.([]byte); ok {
			return string(b)
		}
	}
	return ""
}

// nodeText returns the visible plain text of n and its descendants.
func nodeText(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := c.(type) {
		case *ast.Text:
			b.Write(t.Segment.Value(src))
			if t.SoftLineBreak() || t.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(t.Value)
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			lines := c.Lines()
			for i := 0; i < lines.Len(); i++ {
				seg := lines.At(i)
				b.Write(seg.Value(src))
			}
			b.WriteByte(' ')
			return ast.WalkSkipChildren, nil
		case *ast.Paragraph, *ast.ListItem, *ast.TextBlock:
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
		}
		return ast.WalkContinue, nil
	})
	return strings.Join(strings.Fields(b.String()), " ")
}

// ---------- callouts: "> [!NOTE]" / "> [!WARNING]" / "> [!CAUTION]" ----------

var calloutMarker = regexp.MustCompile(`^\[!(NOTE|TIP|WARNING|CAUTION)\]\s*$`)

var calloutKinds = map[string]string{"NOTE": "note", "TIP": "note", "WARNING": "warn", "CAUTION": "fail"}

type calloutTransformer struct{}

func (calloutTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		bq, ok := n.(*ast.Blockquote)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		p, ok := bq.FirstChild().(*ast.Paragraph)
		if !ok || p.Lines().Len() == 0 {
			return ast.WalkContinue, nil
		}
		first := p.Lines().At(0)
		m := calloutMarker.FindStringSubmatch(strings.TrimSpace(string(first.Value(src))))
		if m == nil {
			return ast.WalkContinue, nil
		}
		// Drop inline nodes belonging to the marker line.
		for c := p.FirstChild(); c != nil; {
			next := c.NextSibling()
			endOfLine := false
			if t, ok := c.(*ast.Text); ok && (t.SoftLineBreak() || t.HardLineBreak()) {
				endOfLine = true
			}
			p.RemoveChild(p, c)
			if endOfLine {
				break
			}
			c = next
		}
		if p.ChildCount() == 0 {
			bq.RemoveChild(bq, p)
		}
		bq.SetAttributeString("data-callout", []byte(calloutKinds[m[1]]))
		return ast.WalkSkipChildren, nil
	})
}

// ---------- renderer overrides ----------

type docRenderer struct{}

func (r *docRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.renderFenced)
	reg.Register(ast.KindBlockquote, r.renderBlockquote)
	reg.Register(ast.KindHeading, r.renderHeading)
}

var calloutLabels = map[string]string{"note": "Note", "warn": "Warning", "fail": "Caution"}

func (r *docRenderer) renderBlockquote(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	kind := ""
	if v, ok := n.AttributeString("data-callout"); ok {
		kind = string(v.([]byte))
	}
	if kind == "" {
		if entering {
			_, _ = w.WriteString("<blockquote>\n")
		} else {
			_, _ = w.WriteString("</blockquote>\n")
		}
		return ast.WalkContinue, nil
	}
	if entering {
		fmt.Fprintf(w, `<aside class="callout callout--%s"><p class="callout__label">%s</p><div class="callout__body">`, kind, calloutLabels[kind])
	} else {
		_, _ = w.WriteString("</div></aside>\n")
	}
	return ast.WalkContinue, nil
}

func (r *docRenderer) renderHeading(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	h := node.(*ast.Heading)
	id := headingID(h)
	if entering {
		fmt.Fprintf(w, `<h%d id="%s">`, h.Level, util.EscapeHTML([]byte(id)))
		return ast.WalkContinue, nil
	}
	if h.Level == 2 || h.Level == 3 {
		fmt.Fprintf(w, `<a class="heading-anchor" href="#%s" aria-label="Link to this section">#</a>`, util.EscapeHTML([]byte(id)))
	}
	fmt.Fprintf(w, "</h%d>\n", h.Level)
	return ast.WalkContinue, nil
}

// infoCaption pulls title="..." out of a fence info string.
var infoCaption = regexp.MustCompile(`title="([^"]*)"`)

func (r *docRenderer) renderFenced(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.FencedCodeBlock)
	lang := string(n.Language(src))
	caption := lang
	if n.Info != nil {
		if m := infoCaption.FindSubmatch(n.Info.Segment.Value(src)); m != nil {
			caption = string(m[1])
		}
	}
	var code strings.Builder
	for i := 0; i < n.Lines().Len(); i++ {
		seg := n.Lines().At(i)
		code.Write(seg.Value(src))
	}
	hl, err := Highlight(lang, code.String())
	if err != nil {
		return ast.WalkStop, err
	}
	fmt.Fprintf(w, `<figure class="code"><figcaption class="code__bar"><span class="code__lang">%s</span>`+
		`<button type="button" class="code__copy" data-copy hidden>Copy</button></figcaption>`+
		`<pre class="chroma" tabindex="0"><code data-copy-source>%s</code></pre></figure>`+"\n",
		template.HTMLEscapeString(caption), hl)
	return ast.WalkSkipChildren, nil
}
