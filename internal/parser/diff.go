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

// hunkScanner walks a unified-diff hunk body, validating each line against the
// hunk's declared ranges while tracking the first changed original-file line.
type hunkScanner struct {
	header           hunkHeader
	originalConsumed int64
	newConsumed      int64
	firstChange      int64
	foundChange      bool
}

// firstChangedLine validates the first hunk body against its declared ranges and
// returns its first changed original-file line.
func firstChangedLine(body []string, header hunkHeader) (int64, bool) {
	s := hunkScanner{header: header}
	for _, rawLine := range body {
		if s.done() {
			break
		}
		line := strings.TrimSuffix(rawLine, "\r")
		if line == `\ No newline at end of file` {
			continue
		}
		if !s.consume(line) {
			return 0, false
		}
	}
	return s.result()
}

// done reports whether both declared ranges have been fully consumed.
func (s *hunkScanner) done() bool {
	return s.originalConsumed == s.header.originalCount && s.newConsumed == s.header.newCount
}

// consume processes one body line, returning false if the line is malformed or
// overruns either declared range.
func (s *hunkScanner) consume(line string) bool {
	if line == "" {
		return false
	}
	switch line[0] {
	case ' ':
		return s.consumeContext()
	case '-':
		return s.consumeRemoval()
	case '+':
		return s.consumeAddition()
	default:
		return false
	}
}

func (s *hunkScanner) consumeContext() bool {
	if s.originalConsumed >= s.header.originalCount || s.newConsumed >= s.header.newCount {
		return false
	}
	s.originalConsumed++
	s.newConsumed++
	return true
}

func (s *hunkScanner) consumeRemoval() bool {
	if s.originalConsumed >= s.header.originalCount {
		return false
	}
	if !s.markChange(s.originalConsumed) {
		return false
	}
	s.originalConsumed++
	return true
}

func (s *hunkScanner) consumeAddition() bool {
	if s.newConsumed >= s.header.newCount {
		return false
	}
	offset := s.originalConsumed
	if s.header.originalCount == 0 {
		offset = 1
	}
	if !s.markChange(offset) {
		return false
	}
	s.newConsumed++
	return true
}

// markChange records the first changed line once, at originalStart+offset.
func (s *hunkScanner) markChange(offset int64) bool {
	if s.foundChange {
		return true
	}
	change, ok := addLine(s.header.originalStart, offset)
	if !ok {
		return false
	}
	s.firstChange = change
	s.foundChange = true
	return true
}

// result returns the located line once both ranges are fully consumed.
func (s *hunkScanner) result() (int64, bool) {
	if s.originalConsumed != s.header.originalCount || s.newConsumed != s.header.newCount {
		return 0, false
	}
	return s.firstChange, s.foundChange
}

func addLine(start, offset int64) (int64, bool) {
	if offset > math.MaxInt64-start {
		return 0, false
	}
	return start + offset, true
}
