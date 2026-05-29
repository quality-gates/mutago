package filter

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSourceLineRegexFilter_EmptyPatterns(t *testing.T) {
	f := NewSourceLineRegexFilter(nil)
	assert.NotNil(t, f)
	assert.Empty(t, f.patterns)
}

func TestNewSourceLineRegexFilter_InvalidRegex(t *testing.T) {
	f := NewSourceLineRegexFilter([]string{"[invalid"})
	assert.Empty(t, f.patterns)
}

func TestNewSourceLineRegexFilter_ValidPatterns(t *testing.T) {
	f := NewSourceLineRegexFilter([]string{"assert\\.", "todo"})
	assert.Len(t, f.patterns, 2)
}

func TestSourceLineRegexFilter_ShouldSkipNilNode(t *testing.T) {
	f := NewSourceLineRegexFilter([]string{"assert\\."})
	assert.False(t, f.ShouldSkip(nil, "any"))
}

func TestSourceLineRegexFilter_CollectNoPatterns(t *testing.T) {
	f := NewSourceLineRegexFilter(nil)
	fset := token.NewFileSet()
	src := `package main
func main() { _ = 1 + 2 }
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	require.NoError(t, err)
	f.Collect(file, fset, "nonexistent.go")
	assert.Empty(t, f.skippedPositions)
}

func TestSourceLineRegexFilter_CollectBadFile(t *testing.T) {
	f := NewSourceLineRegexFilter([]string{"match"})
	fset := token.NewFileSet()
	src := `package main
func main() { _ = 1 + 2 }
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	require.NoError(t, err)
	f.Collect(file, fset, "/nonexistent/path/that/does/not/exist.go")
	assert.Empty(t, f.skippedPositions)
}

func TestSourceLineRegexFilter_CollectAndShouldSkip(t *testing.T) {
	// Line 4 (skipMe := 99) matches "skipMe", line 5 (safe := 1) does not.
	src := `package main

func main() {
	skipMe := 99
	safe := 1
	_ = safe
}
`
	tmp := filepath.Join(t.TempDir(), "sample.go")
	require.NoError(t, os.WriteFile(tmp, []byte(src), 0644))

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, tmp, src, 0)
	require.NoError(t, err)

	f := NewSourceLineRegexFilter([]string{"skipMe"})
	f.Collect(file, fset, tmp)

	// At least one node on line 4 must be recorded.
	assert.NotEmpty(t, f.skippedPositions)

	// Verify: nodes on the skipped line return true; nodes on safe lines return false.
	var seenSkipped, seenSafe bool
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return true
		}
		pos := fset.Position(n.Pos())
		switch pos.Line {
		case 4:
			if f.ShouldSkip(n, "any") {
				seenSkipped = true
			}
		case 5:
			if !f.ShouldSkip(n, "any") {
				seenSafe = true
			}
		}
		return true
	})
	assert.True(t, seenSkipped, "expected at least one node on line 4 to be skipped")
	assert.True(t, seenSafe, "expected at least one node on line 5 to be not-skipped")
}

func TestSourceLineRegexFilter_NoMatchingLines(t *testing.T) {
	src := `package main

func main() {
	x := 1 + 2
	_ = x
}
`
	tmp := filepath.Join(t.TempDir(), "sample.go")
	require.NoError(t, os.WriteFile(tmp, []byte(src), 0644))

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, tmp, src, 0)
	require.NoError(t, err)

	f := NewSourceLineRegexFilter([]string{"nolint"})
	f.Collect(file, fset, tmp)

	assert.Empty(t, f.skippedPositions)
}
