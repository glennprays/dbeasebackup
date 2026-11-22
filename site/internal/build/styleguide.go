package build

// StyleguideData feeds templates/styleguide.html. Values come straight from
// tokens.css so the page cannot drift from the real tokens.
type StyleguideData struct {
	Surfaces []Token
	Text     []Token
	Accent   []Token
	Status   []Token
	Code     []Token
	Type     []Token
	Space    []Token
}

func (b *Builder) renderStyleguide() error {
	tokens, err := LoadTokens(b.opts.SrcDir)
	if err != nil {
		return err
	}
	data := StyleguideData{
		Surfaces: TokensWithPrefix(tokens, "surface-", "color-rule"),
		Text:     TokensWithPrefix(tokens, "color-ink", "color-secondary", "color-muted"),
		Accent:   TokensWithPrefix(tokens, "color-accent", "color-on-accent"),
		Status:   TokensWithPrefix(tokens, "color-ok", "color-warn", "color-fail"),
		Code:     TokensWithPrefix(tokens, "code-"),
		Type:     TokensWithPrefix(tokens, "text-"),
		Space:    TokensWithPrefix(tokens, "space-"),
	}
	return b.renderPage("styleguide.html", &Page{
		Site:        b.site,
		Title:       "Style guide · DBEaseBackup",
		Description: "Design tokens and components of the DBEaseBackup site.",
		Path:        "/_styleguide/",
		NoIndex:     true,
		Data:        data,
	})
}
