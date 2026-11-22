package build

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Token is one CSS custom property declared in tokens.css.
type Token struct {
	Name  string // without the leading "--"
	Value string
}

var (
	tokenDecl = regexp.MustCompile(`(?m)^\s*--([a-z0-9-]+)\s*:\s*([^;]+);`)
	tokenUse  = regexp.MustCompile(`var\(\s*--([a-z0-9-]+)`)
)

// LoadTokens reads the custom properties declared in styles/tokens.css,
// in declaration order, first declaration wins (media-query overrides are
// ignored for display).
func LoadTokens(srcDir string) ([]Token, error) {
	data, err := os.ReadFile(filepath.Join(srcDir, "styles", "tokens.css"))
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []Token
	for _, m := range tokenDecl.FindAllStringSubmatch(string(data), -1) {
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		out = append(out, Token{Name: m[1], Value: strings.TrimSpace(m[2])})
	}
	return out, nil
}

// TokensWithPrefix filters tokens by name prefix (e.g. "color-").
func TokensWithPrefix(tokens []Token, prefixes ...string) []Token {
	var out []Token
	for _, t := range tokens {
		for _, p := range prefixes {
			if strings.HasPrefix(t.Name, p) {
				out = append(out, t)
				break
			}
		}
	}
	return out
}

// UsedVars returns the sorted, de-duplicated custom property names
// referenced via var(--x) in css.
func UsedVars(css string) []string {
	set := map[string]bool{}
	for _, m := range tokenUse.FindAllStringSubmatch(css, -1) {
		set[m[1]] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
