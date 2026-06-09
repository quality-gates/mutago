package parser

import (
	"regexp"
	"strconv"
	"strings"
)

const fallbackLine int64 = 0

// hunkHeaderRegex matches a unified-diff hunk header and captures the original
// start line, e.g. "@@ -12,7 +12,6 @@" or "@@ -1 +1 @@".
var hunkHeaderRegex = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+\d+(?:,\d+)? @@`)

// FindOriginalStartLine returns the original-file line number of the first changed
// line in a unified (`diff -u`) diff. It walks the body of the first hunk rather than
// assuming a fixed amount of leading context, so it stays correct when the change is
// near the top of the file (fewer than three context lines) or when a leading comment
// sits above the changed code — in both cases a fixed "header + 3" offset would point
// at the wrong line (often the comment). Returns 0 when no hunk header is found.
func FindOriginalStartLine(diff []byte) int64 {
	lines := strings.Split(string(diff), "\n")

	for i, line := range lines {
		match := hunkHeaderRegex.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		start, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return fallbackLine
		}

		return firstChangedLine(lines[i+1:], start)
	}

	return fallbackLine
}

// firstChangedLine walks a hunk body, tracking the original-file line number, and
// returns the line number of the first added or removed line. body holds the diff
// lines following the hunk header; start is the hunk's original start line. Context
// and removed lines advance the original line counter; added lines do not. If the
// hunk contains no change (which should not happen for a real mutation) the running
// line number is returned as a safe fallback.
func firstChangedLine(body []string, start int64) int64 {
	cur := start

	for _, line := range body {
		switch {
		case strings.HasPrefix(line, "@@"):
			// Reached the next hunk without finding a change in this one.
			return cur
		case strings.HasPrefix(line, "+"), strings.HasPrefix(line, "-"):
			// First changed line: an addition sits at the current original line
			// (the insertion point); a removal is that original line itself.
			return cur
		case strings.HasPrefix(line, `\`):
			// "\ No newline at end of file" is metadata, not a source line.
		default:
			// Context line (prefixed with a space, or empty for a blank line).
			cur++
		}
	}

	return cur
}
