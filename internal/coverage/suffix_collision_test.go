package coverage

import (
	"fmt"
	"testing"
)

// Regression for issue #74: Profile.IsCovered matched an absolute file path
// against module-relative keys with a path-suffix test while iterating a Go
// map (random order). When one relPath is a path-suffix of another
// (foo/bar.go vs bar.go), both match and the winner was random — and the
// wrong match was cached in resolved, poisoning later lookups.
//
// The fix chooses the longest (most specific) matching key deterministically.
// This test runs many fresh prefixes (defeating the cache) and asserts the
// correct resolution every time. Under the buggy code this fails with
// near-certainty (the wrong key is chosen on ~50% of iterations).
func TestProfileIsCoveredSuffixCollision(t *testing.T) {
	p := &Profile{coveredLines: make(map[string]map[int]bool)}
	p.coveredLines["foo/bar.go"] = map[int]bool{5: true}
	p.coveredLines["bar.go"] = map[int]bool{50: true}

	// For any absFile ending in /foo/bar.go: line 5 must be covered, line 50
	// must NOT (line 50 belongs to the distinct root bar.go).
	var saw5false, saw50true bool
	for i := 0; i < 2000; i++ {
		absFile := fmt.Sprintf("/repo%d/foo/bar.go", i)
		if !p.IsCovered(absFile, 5) {
			saw5false = true
		}
		if p.IsCovered(absFile, 50) {
			saw50true = true
		}
	}
	if saw5false {
		t.Error("BUG: IsCovered(line 5) for foo/bar.go sometimes false (wrong key chosen by suffix collision)")
	}
	if saw50true {
		t.Error("BUG: IsCovered(line 50) for foo/bar.go sometimes true (wrong key chosen by suffix collision)")
	}

	// Sanity: the distinct root bar.go resolves to its own coverage.
	if !p.IsCovered("/repo/root/bar.go", 50) {
		t.Error("root bar.go line 50 should be covered")
	}
	if p.IsCovered("/repo/root/bar.go", 5) {
		t.Error("root bar.go line 5 should not be covered")
	}
}

// Same class of bug for PerTestProfile.CoveringTests.
func TestPerTestProfileCoveringTestsSuffixCollision(t *testing.T) {
	p := &PerTestProfile{data: make(map[string]map[int][]string)}
	p.data["foo/bar.go"] = map[int][]string{5: {"TestA"}}
	p.data["bar.go"] = map[int][]string{50: {"TestB"}}

	var sawLine5MissingA, sawLine50HasB bool
	for i := 0; i < 2000; i++ {
		absFile := fmt.Sprintf("/repo%d/foo/bar.go", i)
		if names := p.CoveringTests(absFile, 5); len(names) != 1 || names[0] != "TestA" {
			sawLine5MissingA = true
		}
		if names := p.CoveringTests(absFile, 50); len(names) != 0 {
			sawLine50HasB = true
		}
	}
	if sawLine5MissingA {
		t.Error("BUG: CoveringTests(line 5) for foo/bar.go sometimes wrong (suffix collision)")
	}
	if sawLine50HasB {
		t.Error("BUG: CoveringTests(line 50) for foo/bar.go sometimes returned TestB (suffix collision)")
	}
}
