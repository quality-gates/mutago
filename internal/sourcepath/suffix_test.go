package sourcepath_test

import (
	"testing"

	"github.com/quality-gates/mutago/v2/internal/sourcepath"
)

func TestLongestSuffixKey(t *testing.T) {
	paths := map[string]int{"": 1, "bar.go": 2, "foo/bar.go": 3}
	for _, tt := range []struct {
		path  string
		key   string
		found bool
	}{
		{"/repo/foo/bar.go", "foo/bar.go", true},
		{"/repo/bar.go", "bar.go", true},
		{"foo/bar.go", "foo/bar.go", true},
		{"/repo/notbar.go", "", false},
		{"/repo/", "", false},
		{"", "", true},
	} {
		key, found := sourcepath.LongestSuffixKey(tt.path, paths)
		if key != tt.key || found != tt.found {
			t.Errorf("LongestSuffixKey(%q) = %q, %v; want %q, %v", tt.path, key, found, tt.key, tt.found)
		}
	}
}
