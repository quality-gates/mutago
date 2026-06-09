package parser

import "testing"

// FuzzFindOriginalStartLine exercises the unified-diff line extractor with arbitrary
// diff text. It must never panic and the returned line number must be non-negative.
func FuzzFindOriginalStartLine(f *testing.F) {
	seeds := []string{
		"",
		"@@ -1,2 +3,4 @@",
		"@@ -1 +1 @@",
		"@@ -0,0 +0,0 @@",
		"@@ -1 +99999999999999999999 @@",
		"@@ -99999999999999999999 +1 @@",
		"@@ -1 +1 @@\n@@ -2 +2 @@",
		"@@ -3,6 +3,6 @@\n func f() {\n \tx := 0\n \t// note\n-\tx++\n+\t_ = x\n \treturn x\n }",
		"not a diff",
		"@@ -1, +1, @@",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, diff string) {
		// Must not panic on arbitrary input and must never return a negative line.
		got := FindOriginalStartLine([]byte(diff))
		if got < 0 {
			t.Fatalf("negative start line %d from input %q", got, diff)
		}
	})
}
