package gitdiff

import (
	"fmt"
	"testing"
)

// Regression for issue #73: IsLineChanged matched an absolute file path against
// the diff's relative paths with a path-suffix test while iterating a Go map
// (random order). When one relPath is a path-suffix of another (foo/bar.go vs
// bar.go), both match the same absFile and the winner was random per run:
// changed lines could be reported unchanged (mutations wrongly skipped) and
// unchanged lines could be reported changed (mutations wrongly kept).
//
// The fix chooses the longest (most specific) matching key deterministically.
// This test runs many fresh prefixes and asserts the correct resolution every
// time. Under the buggy code this fails with near-certainty.
func TestIsLineChangedSuffixCollision(t *testing.T) {
	cl := ChangedLines{
		"foo/bar.go": {{Start: 5, End: 5}},   // only line 5 of foo/bar.go changed
		"bar.go":     {{Start: 50, End: 50}}, // only line 50 of ROOT bar.go changed
	}
	cl.Coalesce()

	// For any absFile ending in /foo/bar.go: line 5 -> changed; line 50 -> not.
	var saw5false, saw50true bool
	for i := 0; i < 2000; i++ {
		absFile := fmt.Sprintf("/repo%d/foo/bar.go", i)
		if !IsLineChanged(cl, absFile, 5) {
			saw5false = true
		}
		if IsLineChanged(cl, absFile, 50) {
			saw50true = true
		}
	}
	if saw5false {
		t.Error("BUG: IsLineChanged(line 5) for foo/bar.go sometimes false (suffix collision picked bar.go)")
	}
	if saw50true {
		t.Error("BUG: IsLineChanged(line 50) for foo/bar.go sometimes true (suffix collision picked bar.go)")
	}

	// Sanity: the distinct root bar.go resolves to its own ranges.
	if !IsLineChanged(cl, "/repo/root/bar.go", 50) {
		t.Error("root bar.go line 50 should be changed")
	}
	if IsLineChanged(cl, "/repo/root/bar.go", 5) {
		t.Error("root bar.go line 5 should not be changed")
	}
}
