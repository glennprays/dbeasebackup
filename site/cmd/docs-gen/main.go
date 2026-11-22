// Command docs-gen builds the public DBEaseBackup site.
//
// Usage (run from the site/ directory):
//
//	docs-gen build [outDir]          render the site (default ../_site)
//	docs-gen serve [-addr :4000]     build, then serve outDir locally
//
// DOCS_VERSION overrides the header version badge; otherwise the highest
// v* git tag is used. DOCS_SITE_URL overrides the canonical origin.
package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/glennprays/dbeasebackup/site/internal/build"
)

const defaultOut = "../_site"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "docs-gen:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: docs-gen build [outDir] | serve [-addr :4000] [outDir]")
	}
	switch args[0] {
	case "build":
		out := defaultOut
		if len(args) > 1 {
			out = args[1]
		}
		return buildSite(out)
	case "serve":
		fs := flag.NewFlagSet("serve", flag.ContinueOnError)
		addr := fs.String("addr", "localhost:4000", "listen address")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		out := defaultOut
		if fs.NArg() > 0 {
			out = fs.Arg(0)
		}
		if err := buildSite(out); err != nil {
			return err
		}
		fmt.Printf("docs-gen: serving %s at http://%s\n", out, *addr)
		srv := &http.Server{
			Addr:              *addr,
			Handler:           notFoundFallback(out),
			ReadHeaderTimeout: 5 * time.Second,
		}
		return srv.ListenAndServe()
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func buildSite(out string) error {
	version := build.ResolveVersion(os.Getenv("DOCS_VERSION"), func() ([]string, error) {
		return build.GitTags(".")
	})
	start := time.Now()
	if err := build.Build(build.Options{
		SrcDir:  ".",
		OutDir:  out,
		Version: version,
		SiteURL: os.Getenv("DOCS_SITE_URL"),
	}); err != nil {
		return err
	}
	badge := version
	if badge == "" {
		badge = "(none)"
	}
	fmt.Printf("docs-gen: built %s in %s, version %s\n", out, time.Since(start).Round(time.Millisecond), badge)
	return nil
}

// notFoundFallback serves dir and answers missing paths with 404.html,
// mirroring GitHub Pages.
func notFoundFallback(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := http.Dir(dir).Open(r.URL.Path)
		if err == nil {
			f.Close()
			files.ServeHTTP(w, r)
			return
		}
		body, readErr := os.ReadFile(dir + "/404.html")
		if readErr != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(body)
	})
}
