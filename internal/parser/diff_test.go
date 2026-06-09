package parser

import (
	"fmt"
	"strings"
	"testing"
)

func TestFindOriginalStartLineContextLines(t *testing.T) {
	for contextLines := 0; contextLines <= 4; contextLines++ {
		context := strings.Repeat(" unchanged\n", contextLines)
		oldStart := int64(10)
		expected := oldStart + int64(contextLines)

		t.Run(fmt.Sprintf("deletion/%d", contextLines), func(t *testing.T) {
			diff := fmt.Sprintf("@@ -%d,%d +%d,%d @@\n%s-removed\n",
				oldStart, contextLines+1, oldStart, contextLines, context)
			if got := FindOriginalStartLine([]byte(diff)); got != expected {
				t.Fatalf("FindOriginalStartLine() = %d, want %d\ndiff:\n%s", got, expected, diff)
			}
		})

		t.Run(fmt.Sprintf("replacement/%d", contextLines), func(t *testing.T) {
			diff := fmt.Sprintf("@@ -%d,%d +%d,%d @@\n%s-removed\n+added\n",
				oldStart, contextLines+1, oldStart, contextLines+1, context)
			if got := FindOriginalStartLine([]byte(diff)); got != expected {
				t.Fatalf("FindOriginalStartLine() = %d, want %d\ndiff:\n%s", got, expected, diff)
			}
		})

		t.Run(fmt.Sprintf("addition/%d", contextLines), func(t *testing.T) {
			headerStart := oldStart
			if contextLines == 0 {
				headerStart--
			}
			diff := fmt.Sprintf("@@ -%d,%d +%d,%d @@\n%s+added\n",
				headerStart, contextLines, oldStart, contextLines+1, context)
			if got := FindOriginalStartLine([]byte(diff)); got != expected {
				t.Fatalf("FindOriginalStartLine() = %d, want %d\ndiff:\n%s", got, expected, diff)
			}
		})
	}
}

func TestFindOriginalStartLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			// Three context lines above the change: removed line is start+3.
			name: "removal with three context lines",
			input: "--- Original\n" +
				"+++ New\n" +
				"@@ -20,7 +20,7 @@\n" +
				" }\n" +
				" \n" +
				" func doo() {\n" +
				"-\tddd := 6\n" +
				"+\tddd := 5\n" +
				" \tslog.Info(strconv.Itoa(ddd))\n" +
				" \tfmt.Println(\"doo\")\n" +
				" }\n",
			expected: 23,
		},
		{
			// Only two context lines above the change (near the top of a function).
			// The fixed "header + 3" heuristic returned 17 here; the real line is 16.
			name: "removal with two context lines",
			input: "--- Original\n" +
				"+++ New\n" +
				"@@ -14,6 +14,6 @@\n" +
				" func foo() {\n" +
				" \tjjj := 6\n" +
				"-\tslog.Info(strconv.Itoa(jjj))\n" +
				"+\t_, _, _ = slog.Info, strconv.Itoa, jjj\n" +
				" \n" +
				" \tfmt.Println(\"foo\")\n" +
				" }\n",
			expected: 16,
		},
		{
			// A leading comment sits directly above the changed code. The comment is
			// an unchanged context line (line 5); the changed code is line 6. The
			// reported line must be the code, not the comment.
			name: "leading comment above changed code",
			input: "--- Original\n" +
				"+++ New\n" +
				"@@ -3,6 +3,6 @@\n" +
				" func Calc(x int) int {\n" +
				" \ttotal := 0\n" +
				" \t// increment total by x for the running sum\n" +
				"-\ttotal += x\n" +
				"+\t_, _ = total, x\n" +
				" \treturn total\n" +
				" }\n",
			expected: 6,
		},
		{
			// Change on the very first line of the hunk (no leading context).
			name: "change on first hunk line",
			input: "--- Original\n" +
				"+++ New\n" +
				"@@ -5,2 +5,2 @@\n" +
				"-\told := 1\n" +
				"+\told := 2\n" +
				" \tkeep := 3\n",
			expected: 5,
		},
		{
			// Pure insertion: the change is at the insertion point (original line 4).
			name: "pure addition",
			input: "--- Original\n" +
				"+++ New\n" +
				"@@ -3,2 +3,3 @@\n" +
				" \ta := 1\n" +
				"+\tinserted := 2\n" +
				" \tb := 3\n",
			expected: 4,
		},
		{
			name:     "pure addition at top of empty file",
			input:    "@@ -0,0 +1 @@\n+first\n",
			expected: 1,
		},
		{
			name: "blank context line",
			input: "@@ -7,2 +7,2 @@\n" +
				" \n" +
				"-old\n" +
				"+new\n",
			expected: 8,
		},
		{
			name: "no newline at end of file markers",
			input: "@@ -2 +2 @@\n" +
				"-old\n" +
				"\\ No newline at end of file\n" +
				"+new\n" +
				"\\ No newline at end of file\n",
			expected: 2,
		},
		{
			// Multiple hunks: the first hunk's first changed line wins.
			name: "multiple hunks returns first hunk change",
			input: "--- Original\n" +
				"+++ New\n" +
				"@@ -14,3 +14,3 @@\n" +
				" func foo() {\n" +
				" \tjjj := 6\n" +
				"-\tslog.Info(strconv.Itoa(jjj))\n" +
				"+\t_ = jjj\n" +
				"@@ -20,2 +20,2 @@\n" +
				" func doo() {\n" +
				"-\tddd := 6\n" +
				"+\tddd := 5\n",
			expected: 16,
		},
		{
			name: "git diff metadata and hunk section",
			input: "diff --git a/example.go b/example.go\n" +
				"index 257cc56..3bd1f0e 100644\n" +
				"--- a/example.go\n" +
				"+++ b/example.go\n" +
				"@@ -40,2 +40,2 @@ func example() {\n" +
				" unchanged\n" +
				"-old\n" +
				"+new\n",
			expected: 41,
		},
		{
			name:     "CRLF diff",
			input:    "@@ -3 +3 @@\r\n-old\r\n+new\r\n",
			expected: 3,
		},
		{
			name:     "empty input",
			input:    "",
			expected: 0,
		},
		{
			name:     "no hunk header",
			input:    "--- Original\n+++ New\nnot a diff\n",
			expected: 0,
		},
		{
			name:     "invalid line numbers",
			input:    "@@ -abc +def @@\n-garbage\n+garbage\n",
			expected: 0,
		},
		{
			name:     "malformed header suffix",
			input:    "@@ -1 +1 @@garbage\n-old\n+new\n",
			expected: 0,
		},
		{
			name:     "unprefixed hunk body line",
			input:    "@@ -1,2 +1,2 @@\nunchanged\n-old\n+new\n",
			expected: 0,
		},
		{
			name:     "empty hunk body line is not context",
			input:    "@@ -1,2 +1,2 @@\n\n-old\n+new\n",
			expected: 0,
		},
		{
			name:     "hunk body exceeds declared range",
			input:    "@@ -1 +1 @@\n unchanged\n-old\n+new\n",
			expected: 0,
		},
		{
			name:     "hunk without a change",
			input:    "@@ -1 +1 @@\n unchanged\n",
			expected: 0,
		},
		{
			name:     "original start overflows int64",
			input:    "@@ -99999999999999999999 +1 @@\n-old\n+new\n",
			expected: 0,
		},
		{
			name:     "original count overflows int64",
			input:    "@@ -1,99999999999999999999 +1 @@\n-old\n+new\n",
			expected: 0,
		},
		{
			name:     "new start overflows int64",
			input:    "@@ -1 +99999999999999999999 @@\n-old\n+new\n",
			expected: 0,
		},
		{
			name:     "new count overflows int64",
			input:    "@@ -1 +1,99999999999999999999 @@\n-old\n+new\n",
			expected: 0,
		},
		{
			name:     "original range overflows int64",
			input:    "@@ -9223372036854775807,2 +1,2 @@\n-old\n+new\n",
			expected: 0,
		},
		{
			name: "addition after maximum original line overflows int64",
			input: "@@ -9223372036854775807 +1,2 @@\n" +
				" unchanged\n" +
				"+added\n",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindOriginalStartLine([]byte(tt.input))
			if got != tt.expected {
				t.Errorf("FindOriginalStartLine() = %v, want %v", got, tt.expected)
			}
		})
	}
}
