package filter

import (
	"bufio"
	"go/ast"
	"go/token"
	"os"
	"regexp"
	"strings"
)

// SourceLineRegexFilter skips mutations on any source line that matches one of
// the configured regular expressions. This lets teams suppress known-noisy
// patterns globally in the config file without modifying source files.
//
// Example config usage:
//
//	ignore_source_lines:
//	  - "assert\\."      # skip lines that call assertion helpers
//	  - "//\\s*nolint"   # skip lines with nolint directives
type SourceLineRegexFilter struct {
	patterns     []*regexp.Regexp
	skippedLines map[int]struct{}
	fset         *token.FileSet
}

// NewSourceLineRegexFilter compiles each pattern string and returns a filter.
// Patterns that fail to compile are silently skipped.
func NewSourceLineRegexFilter(patterns []string) *SourceLineRegexFilter {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}
	return &SourceLineRegexFilter{patterns: compiled}
}

// Collect reads the source file and records physical line numbers that match
// any configured pattern.
func (f *SourceLineRegexFilter) Collect(_ *ast.File, fset *token.FileSet, fileAbs string) {
	f.fset = fset
	f.skippedLines = nil
	if len(f.patterns) == 0 {
		return
	}

	content, err := os.ReadFile(fileAbs)
	if err != nil {
		return
	}

	// Build a set of 1-based line numbers whose content matches a pattern.
	skippedLines := make(map[int]struct{})
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	lineNum := 1
	for scanner.Scan() {
		line := scanner.Text()
		for _, re := range f.patterns {
			if re.MatchString(line) {
				skippedLines[lineNum] = struct{}{}
				break
			}
		}
		lineNum++
	}

	f.skippedLines = skippedLines
}

// ShouldSkip implements NodeFilter. Returns true when the node starts on a
// physical source line that matched a configured regex.
func (f *SourceLineRegexFilter) ShouldSkip(node ast.Node, mutatorName string) bool {
	return node != nil && f.ShouldSkipPosition(node.Pos(), mutatorName)
}

// ShouldSkipPosition returns true when pos is on a physical source line that
// matched a configured regex.
func (f *SourceLineRegexFilter) ShouldSkipPosition(pos token.Pos, _ string) bool {
	if f.fset == nil {
		return false
	}
	_, skip := f.skippedLines[f.fset.PositionFor(pos, false).Line]
	return skip
}
