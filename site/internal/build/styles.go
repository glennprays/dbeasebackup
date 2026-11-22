package build

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// StyleOrder is the cascade order of site/styles/*.css in the bundle.
// Every file in site/styles must be listed; a test enforces it.
var StyleOrder = []string{
	"tokens.css",
	"base.css",
	"components.css",
	"landing.css",
	"docs.css",
	"styleguide.css",
}

// bundleCSS concatenates site/styles in StyleOrder (skipping files that do
// not exist yet) and appends generated code-highlighting rules, written to
// /assets/css/site.css.
func (b *Builder) bundleCSS() error {
	var buf bytes.Buffer
	for _, name := range StyleOrder {
		data, err := os.ReadFile(filepath.Join(b.opts.SrcDir, "styles", name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(&buf, "/* --- %s --- */\n", name)
		buf.Write(data)
		buf.WriteByte('\n')
	}
	if extra := b.extraCSS(); extra != "" {
		buf.WriteString("/* --- generated --- */\n")
		buf.WriteString(extra)
	}
	return b.writeAsset("/assets/css/site.css", buf.Bytes())
}

// extraCSS returns generated CSS appended to the bundle (code highlighting).
func (b *Builder) extraCSS() string { return "" }
