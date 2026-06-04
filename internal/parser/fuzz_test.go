package parser

import "testing"

// FuzzParseDiffOutput exercises the unified-diff line extractor with arbitrary
// diff text. It must never panic and every returned line must be non-negative.
func FuzzParseDiffOutput(f *testing.F) {
	seeds := []string{
		"",
		"@@ -1,2 +3,4 @@",
		"@@ -1 +1 @@",
		"@@ -0,0 +0,0 @@",
		"@@ -1 +99999999999999999999 @@",
		"@@ -1 +1 @@\n@@ -2 +2 @@",
		"not a diff",
		"@@ -1, +1, @@",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, diff string) {
		lines := ParseDiffOutput(diff)
		for _, l := range lines {
			if l < 0 {
				t.Fatalf("negative line number %d from input %q", l, diff)
			}
		}
		// FindOriginalStartLine wraps ParseDiffOutput; must not panic.
		got := FindOriginalStartLine([]byte(diff))
		if got < 0 {
			t.Fatalf("negative start line %d from input %q", got, diff)
		}
	})
}
