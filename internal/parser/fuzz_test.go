package parser

import (
	"fmt"
	"strings"
	"testing"
)

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

func FuzzFindOriginalStartLineMatchesModel(f *testing.F) {
	f.Add(uint16(1), uint8(0), uint8(0), uint8(0), false, false)
	f.Add(uint16(10), uint8(4), uint8(3), uint8(1), true, true)
	f.Add(uint16(100), uint8(2), uint8(0), uint8(2), true, false)

	f.Fuzz(func(
		t *testing.T,
		startSeed uint16,
		leadingSeed uint8,
		trailingSeed uint8,
		changeSeed uint8,
		gitStyle bool,
		finalNewline bool,
	) {
		start := int64(startSeed%1000) + 1
		leading := int(leadingSeed % 5)
		trailing := int(trailingSeed % 5)
		change := changeSeed % 3

		diff, expected := modelDiff(start, leading, trailing, change, gitStyle, finalNewline)
		if got := FindOriginalStartLine([]byte(diff)); got != expected {
			t.Fatalf("FindOriginalStartLine() = %d, want %d\ndiff:\n%s", got, expected, diff)
		}
	})
}

// modelDiff constructs a valid hunk from source operations and derives the
// expected line directly from those operations, independently of the parser.
func modelDiff(
	start int64,
	leading int,
	trailing int,
	change uint8,
	gitStyle bool,
	finalNewline bool,
) (string, int64) {
	oldChanged, newChanged := 0, 0
	switch change {
	case 0: // addition
		newChanged = 1
	case 1: // deletion
		oldChanged = 1
	case 2: // replacement
		oldChanged = 1
		newChanged = 1
	}

	originalCount := leading + oldChanged + trailing
	newCount := leading + newChanged + trailing
	originalStart := start
	newStart := start
	if originalCount == 0 {
		originalStart--
	}
	if newCount == 0 {
		newStart--
	}

	var diff strings.Builder
	if gitStyle {
		diff.WriteString("diff --git a/model.go b/model.go\n")
		diff.WriteString("index 1111111..2222222 100644\n")
		diff.WriteString("--- a/model.go\n")
		diff.WriteString("+++ b/model.go\n")
	}
	fmt.Fprintf(&diff, "@@ -%d,%d +%d,%d @@ model\n", originalStart, originalCount, newStart, newCount)
	for i := 0; i < leading; i++ {
		fmt.Fprintf(&diff, " context before %d\n", i)
	}
	if oldChanged == 1 {
		diff.WriteString("-old\n")
	}
	if newChanged == 1 {
		diff.WriteString("+new\n")
	}
	for i := 0; i < trailing; i++ {
		fmt.Fprintf(&diff, " context after %d\n", i)
	}

	result := diff.String()
	if !finalNewline {
		result = strings.TrimSuffix(result, "\n")
	}
	return result, start + int64(leading)
}
