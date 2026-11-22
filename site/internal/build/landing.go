package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// lastReleaseWithoutTargets is the newest release that predates targets
// files, basebackup mode and retries. While it is still the latest tag, the
// landing page says so; the note disappears on the first redeploy after the
// next release.
const lastReleaseWithoutTargets = "v2.2.0"

// LandingData feeds templates/landing.html.
type LandingData struct {
	// Targets is a highlighted excerpt of the repository's
	// targets.example.yaml, so the landing page cannot drift from it.
	Targets CodeBlock
	// PreRelease is true while the published image lacks targets files.
	PreRelease bool
	// DockerRun is the one-line command in the hero.
	DockerRun string
}

// targetsExcerpt returns the first target of targets.example.yaml (from its
// "- name:" line through "backup_dir:"), under a "targets:" root.
func targetsExcerpt(srcDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(srcDir, "..", "targets.example.yaml"))
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	start, end := -1, -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if start < 0 && strings.HasPrefix(t, "- name:") {
			start = i
		}
		if start >= 0 && strings.HasPrefix(t, "backup_dir:") {
			end = i
			break
		}
	}
	if start < 0 || end < 0 {
		return "", fmt.Errorf("targets.example.yaml: could not find the first target's name..backup_dir block")
	}
	return "targets:\n" + strings.Join(lines[start:end+1], "\n") + "\n", nil
}

func (b *Builder) renderLanding() error {
	excerpt, err := targetsExcerpt(b.opts.SrcDir)
	if err != nil {
		return err
	}
	cb, err := NewCodeBlock("yaml", "targets.example.yaml (excerpt)", excerpt)
	if err != nil {
		return err
	}
	data := LandingData{
		Targets:    cb,
		PreRelease: b.site.Version == "" || b.site.Version == lastReleaseWithoutTargets,
		DockerRun:  `docker run -d --env-file .env -v "$(pwd)/targets.yaml:/config/targets.yaml:ro" -p 8080:8080 glennprays/dbeasebackup`,
	}
	return b.renderPage("landing.html", &Page{
		Site:        b.site,
		Title:       "DBEaseBackup · Scheduled PostgreSQL backups to Google Drive or S3",
		Description: "DBEaseBackup runs pg_dump or pg_basebackup on cron for every target, verifies the result, uploads it to Google Drive or S3 and enforces retention. One Go process, MIT licensed.",
		Path:        "/",
		Section:     "home",
		Data:        data,
	})
}
