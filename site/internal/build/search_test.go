package build

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const maxIndexBytes = 150 << 10

func TestSearchIndex(t *testing.T) {
	out := buildForTest(t)
	raw := readOut(t, out, strings.TrimPrefix(SearchIndexPath, "/"))
	if len(raw) > maxIndexBytes {
		t.Errorf("search index is %d bytes, over the %d byte budget", len(raw), maxIndexBytes)
	}
	var entries []SearchEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		t.Fatal(err)
	}

	var sawRetention, sawHeading bool
	for _, e := range entries {
		if e.Title == "" || e.Section == "" || e.URL == "" {
			t.Errorf("incomplete entry %+v", e)
		}
		if len(e.Text) > maxSnippet+len("…") {
			t.Errorf("entry text too long (%d) for %s", len(e.Text), e.URL)
		}
		if e.URL == "/docs/retention/" && e.Heading == "" && strings.Contains(e.Text, "retention_days") {
			sawRetention = true
		}
		if e.URL == "/docs/webhooks/#verify-the-signature" && e.Heading == "Verify the signature" {
			sawHeading = true
		}
		// Every URL (and fragment) must exist in the built site.
		page, frag, _ := strings.Cut(e.URL, "#")
		html := readOut(t, out, strings.TrimPrefix(page, "/")+"index.html")
		if frag != "" && !strings.Contains(html, `id="`+frag+`"`) {
			t.Errorf("index entry %s points at a missing anchor", e.URL)
		}
	}
	if !sawRetention {
		t.Error("index has no intro entry for /docs/retention/")
	}
	if !sawHeading {
		t.Error("index has no heading entry for webhooks#verify-the-signature")
	}

	doc := readOut(t, out, "docs/retention/index.html")
	for _, s := range []string{"data-search-dialog", `data-index="/docs/search-index.json?v=`, `role="combobox"`, `role="listbox"`, "js/search.js"} {
		if !strings.Contains(doc, s) {
			t.Errorf("doc page missing %q", s)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate short = %q", got)
	}
	got := truncate("alpha beta gamma delta", 12)
	if got != "alpha beta…" {
		t.Errorf("truncate = %q", got)
	}
	if got := truncate("ééééé", 3); got != "é…" {
		t.Errorf("truncate multibyte = %q", got)
	}
}

// TestSearchRankingJS runs the client ranking tests with node when present.
func TestSearchRankingJS(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not installed; Go index tests still gate the build")
	}
	out := buildForTest(t)
	cmd := exec.Command(node, "--test", filepath.Join("testdata", "search_rank.test.js"))
	cmd.Env = append(os.Environ(), "SEARCH_INDEX="+filepath.Join(out, "docs", "search-index.json"))
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("node --test failed: %v\n%s", err, b)
	}
}
