package build

import (
	"errors"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	list := func(tags ...string) func() ([]string, error) {
		return func() ([]string, error) { return tags, nil }
	}
	tests := []struct {
		name     string
		explicit string
		lister   func() ([]string, error)
		want     string
	}{
		{"explicit wins", "v9.9.9", list("v2.2.0"), "v9.9.9"},
		{"explicit trimmed", "  v1.0.0 \n", nil, "v1.0.0"},
		{"highest tag", "", list("v2.2.0", "v2.1.0"), "v2.2.0"},
		{"skips non-release tags", "", list("v3.0.0-rc1", "vnext", "v2.1.0"), "v2.1.0"},
		{"no tags", "", list(""), ""},
		{"git error", "", func() ([]string, error) { return nil, errors.New("no git") }, ""},
		{"nil lister", "", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveVersion(tt.explicit, tt.lister); got != tt.want {
				t.Errorf("ResolveVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}
