package build

import (
	"bytes"
	"html/template"
	"strings"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// formatter emits class-based markup; colors come from the --code-* tokens
// in components.css, so no chroma style colors reach the page.
var formatter = chromahtml.New(
	chromahtml.WithClasses(true),
	chromahtml.PreventSurroundingPre(true),
)

// Highlight returns syntax-highlighted HTML (without the <pre> wrapper)
// for code in lang. Unknown languages fall back to escaped plain text.
func Highlight(lang, code string) (template.HTML, error) {
	code = strings.TrimRight(code, "\n")
	lexer := lexers.Get(lang)
	if lexer == nil || lang == "" || lang == "text" {
		return template.HTML(template.HTMLEscapeString(code)), nil
	}
	lexer = chroma.Coalesce(lexer)
	it, err := lexer.Tokenise(nil, code)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := formatter.Format(&buf, styles.Fallback, it); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}

// CodeBlock is the data for the "code" component.
type CodeBlock struct {
	Lang    string
	Caption string // optional file name or label shown in the bar
	HTML    template.HTML
}

// NewCodeBlock highlights code and wraps it for the "code" partial.
func NewCodeBlock(lang, caption, code string) (CodeBlock, error) {
	h, err := Highlight(lang, code)
	if err != nil {
		return CodeBlock{}, err
	}
	return CodeBlock{Lang: lang, Caption: caption, HTML: h}, nil
}
