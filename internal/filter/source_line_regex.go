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
	patterns         []*regexp.Regexp
	skippedPositions map[token.Pos]struct{}
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
	return &SourceLineRegexFilter{
		patterns:         compiled,
		skippedPositions: make(map[token.Pos]struct{}),
	}
}

// Collect reads the source file and records the positions of all AST nodes
// that sit on lines matching any configured pattern.
func (f *SourceLineRegexFilter) Collect(file *ast.File, fset *token.FileSet, fileAbs string) {
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

	if len(skippedLines) == 0 {
		return
	}

	// Walk the AST and record positions of nodes on skipped lines.
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return true
		}
		pos := fset.Position(n.Pos())
		if _, skip := skippedLines[pos.Line]; skip {
			f.skippedPositions[n.Pos()] = struct{}{}
		}
		return true
	})
}

// ShouldSkip implements NodeFilter. Returns true when the node's start
// position falls on a line that matched a configured regex.
func (f *SourceLineRegexFilter) ShouldSkip(node ast.Node, _ string) bool {
	if node == nil {
		return false
	}
	_, skip := f.skippedPositions[node.Pos()]
	return skip
}
