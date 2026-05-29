package parser

import (
	"fmt"
	"regexp"
	"strconv"
)

const (
	fallbackLine     int64 = 0
	diffContextLines       = 3
)

var (
	diffRegex *regexp.Regexp
)

func init() {
	var err error
	diffRegex, err = regexp.Compile(`@@ -(\d+),?\d* \+(\d+),?\d* @@`)
	if err != nil {
		panic(fmt.Sprintf("failed to compile diff regex: %v", err))
	}
}

// ParseDiffOutput parses the unified diff (-u) output to extract the line numbers where changes occurred.
// The `-u` flag provides exactly 3 lines of context around changes, so the actual changed line
// can be derived by adjusting the reported line number from the diff header.
func ParseDiffOutput(diff string) []int64 {
	lines := make([]int64, 0)

	matches := diffRegex.FindAllStringSubmatch(diff, -1)
	for _, match := range matches {
		line, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			lines = append(lines, fallbackLine)
			continue
		}

		actualLine := line + diffContextLines
		lines = append(lines, actualLine)
	}

	return lines
}

// FindOriginalStartLine attempts to find the original line number where a mutation occurred.
// For multi-hunk diffs the first hunk's start line is returned — it is the earliest affected
// line in the original file and the most representative location for coverage and diff-filter checks.
func FindOriginalStartLine(diff []byte) int64 {
	changedLines := ParseDiffOutput(string(diff))

	if len(changedLines) == 0 {
		return fallbackLine
	}

	return changedLines[0]
}
