package build

import (
	"os/exec"
	"regexp"
	"strings"
)

// releaseTag matches the clean release tags the release workflow creates
// (v<major>.<minor>.<patch>). Anything else is ignored for the badge.
var releaseTag = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// ResolveVersion picks the version shown in the header badge.
// An explicit value (DOCS_VERSION) wins; otherwise the highest release tag
// from tagLister is used. An empty result hides the badge.
func ResolveVersion(explicit string, tagLister func() ([]string, error)) string {
	if v := strings.TrimSpace(explicit); v != "" {
		return v
	}
	if tagLister == nil {
		return ""
	}
	tags, err := tagLister()
	if err != nil {
		return ""
	}
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if releaseTag.MatchString(t) {
			return t
		}
	}
	return ""
}

// GitTags lists v* tags, highest version first. It returns an error when
// git is unavailable or the directory is not a repository.
func GitTags(dir string) ([]string, error) {
	cmd := exec.Command("git", "tag", "-l", "v*", "--sort=-v:refname")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSpace(string(out)), "\n"), nil
}
