package parser

import (
	"testing"
)

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
				"@@ -14,7 +14,7 @@\n" +
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
				"@@ -5,3 +5,3 @@\n" +
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
			// Multiple hunks: the first hunk's first changed line wins.
			name: "multiple hunks returns first hunk change",
			input: "--- Original\n" +
				"+++ New\n" +
				"@@ -14,4 +14,4 @@\n" +
				" func foo() {\n" +
				" \tjjj := 6\n" +
				"-\tslog.Info(strconv.Itoa(jjj))\n" +
				"+\t_ = jjj\n" +
				"@@ -20,4 +20,4 @@\n" +
				" func doo() {\n" +
				"-\tddd := 6\n" +
				"+\tddd := 5\n",
			expected: 16,
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
