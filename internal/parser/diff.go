package parser

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

const fallbackLine int64 = 0

// hunkHeaderRegex captures both ranges in a unified-diff hunk header.
var hunkHeaderRegex = regexp.MustCompile(
	`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@(?: .*)?$`,
)

type hunkHeader struct {
	originalStart int64
	originalCount int64
	newStart      int64
	newCount      int64
}

// FindOriginalStartLine returns the original-file line number of the first changed
// line in a unified (`diff -u`) diff. It walks the body of the first hunk rather than
// assuming a fixed amount of leading context, so it stays correct when the change is
// near the top of the file (fewer than three context lines) or when a leading comment
// sits above the changed code — in both cases a fixed "header + 3" offset would point
// at the wrong line (often the comment). Returns 0 when no hunk header is found.
func FindOriginalStartLine(diff []byte) int64 {
	lines := strings.Split(string(diff), "\n")

	for i, line := range lines {
		match := hunkHeaderRegex.FindStringSubmatch(strings.TrimSuffix(line, "\r"))
		if match == nil {
			continue
		}

		header, ok := parseHunkHeader(match)
		if !ok {
			return fallbackLine
		}

		line, ok := firstChangedLine(lines[i+1:], header)
		if !ok {
			return fallbackLine
		}
		return line
	}

	return fallbackLine
}

func parseHunkHeader(match []string) (hunkHeader, bool) {
	originalStart, originalCount, ok := parseHunkRange(match[1], match[2])
	if !ok {
		return hunkHeader{}, false
	}
	newStart, newCount, ok := parseHunkRange(match[3], match[4])
	if !ok {
		return hunkHeader{}, false
	}

	return hunkHeader{
		originalStart: originalStart,
		originalCount: originalCount,
		newStart:      newStart,
		newCount:      newCount,
	}, true
}

func parseHunkRange(startText, countText string) (int64, int64, bool) {
	start, err := strconv.ParseInt(startText, 10, 64)
	if err != nil {
		return 0, 0, false
	}

	count := int64(1)
	if countText != "" {
		count, err = strconv.ParseInt(countText, 10, 64)
		if err != nil {
			return 0, 0, false
		}
	}

	if count == 0 {
		return start, count, start < math.MaxInt64
	}
	if start == 0 || count-1 > math.MaxInt64-start {
		return 0, 0, false
	}
	return start, count, true
}

// firstChangedLine validates the first hunk body against its declared ranges and
// returns its first changed original-file line.
func firstChangedLine(body []string, header hunkHeader) (int64, bool) {
	var originalConsumed, newConsumed int64
	var firstChange int64

	for _, rawLine := range body {
		if originalConsumed == header.originalCount && newConsumed == header.newCount {
			break
		}

		action, ok := classifyBodyLine(rawLine, header, originalConsumed, newConsumed)
		if !ok {
			return 0, false
		}
		if action.skip {
			continue
		}

		if firstChange == 0 && action.changedLine > 0 {
			firstChange = action.changedLine
		}
		originalConsumed += action.origDelta
		newConsumed += action.newDelta
	}

	return validateHunkCounts(header, originalConsumed, newConsumed, firstChange)
}

func validateHunkCounts(header hunkHeader, originalConsumed, newConsumed, firstChange int64) (int64, bool) {
	if originalConsumed != header.originalCount || newConsumed != header.newCount {
		return 0, false
	}
	return firstChange, firstChange > 0
}

// lineAction describes how a single body line affects the hunk counters.
type lineAction struct {
	skip        bool  // true for lines like "\ No newline at end of file"
	origDelta   int64 // how much to advance originalConsumed
	newDelta    int64 // how much to advance newConsumed
	changedLine int64 // non-zero when this line is the first change
}

func classifyBodyLine(rawLine string, header hunkHeader, origConsumed, newConsumed int64) (lineAction, bool) {
	line := strings.TrimSuffix(rawLine, "\r")
	if line == `\ No newline at end of file` {
		return lineAction{skip: true}, true
	}
	if line == "" {
		return lineAction{}, false
	}

	switch line[0] {
	case ' ':
		return processContext(header, origConsumed, newConsumed)
	case '-':
		return processDeletion(header, origConsumed, newConsumed)
	case '+':
		return processAddition(header, origConsumed, newConsumed)
	default:
		return lineAction{}, false
	}
}

func processContext(header hunkHeader, origConsumed, newConsumed int64) (lineAction, bool) {
	if origConsumed >= header.originalCount || newConsumed >= header.newCount {
		return lineAction{}, false
	}
	return lineAction{origDelta: 1, newDelta: 1}, true
}

func processDeletion(header hunkHeader, origConsumed, newConsumed int64) (lineAction, bool) {
	if origConsumed >= header.originalCount {
		return lineAction{}, false
	}
	changed, ok := addLine(header.originalStart, origConsumed)
	if !ok {
		return lineAction{}, false
	}
	return lineAction{origDelta: 1, changedLine: changed}, true
}

func processAddition(header hunkHeader, origConsumed, newConsumed int64) (lineAction, bool) {
	if newConsumed >= header.newCount {
		return lineAction{}, false
	}
	offset := origConsumed
	if header.originalCount == 0 {
		offset = 1
	}
	changed, ok := addLine(header.originalStart, offset)
	if !ok {
		return lineAction{}, false
	}
	return lineAction{newDelta: 1, changedLine: changed}, true
}

func addLine(start, offset int64) (int64, bool) {
	if offset > math.MaxInt64-start {
		return 0, false
	}
	return start + offset, true
}
