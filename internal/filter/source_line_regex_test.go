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

func TestInvalidSourceLinePatterns_ReportsOnlyInvalidInOrder(t *testing.T) {
	invalid := InvalidSourceLinePatterns([]string{"ok", "return (5", "fine", "[bad"})
	require.Len(t, invalid, 2)
	assert.Equal(t, "return (5", invalid[0].Pattern)
	assert.Contains(t, invalid[0].Err.Error(), "missing closing )")
	assert.Equal(t, "[bad", invalid[1].Pattern)
	assert.Error(t, invalid[1].Err)
}

func TestInvalidSourceLinePatterns_AllValid(t *testing.T) {
	assert.Empty(t, InvalidSourceLinePatterns([]string{"assert\\.", "todo"}))
}

func TestNewSourceLineRegexFilter_ValidPatterns(t *testing.T) {
	f := NewSourceLineRegexFilter([]string{"assert\\.", "todo"})
	assert.Len(t, f.patterns, 2)
}

func TestSourceLineRegexFilter_ShouldSkipNilNode(t *testing.T) {
	f := NewSourceLineRegexFilter([]string{"assert\\."})
	assert.False(t, f.ShouldSkip(nil, "any"))
}

func TestSourceLineRegexFilter_ShouldSkipPositionBeforeCollect(t *testing.T) {
	f := NewSourceLineRegexFilter([]string{"match"})
	fset := token.NewFileSet()
	pos := fset.AddFile("test.go", -1, 1).Pos(0)

	assert.True(t, pos.IsValid())
	assert.False(t, f.ShouldSkipPosition(pos, "any"))
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
	assert.Empty(t, f.skippedLines)
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
	assert.Empty(t, f.skippedLines)
}

func TestSourceLineRegexFilter_CollectBadFileClearsPreviousMatches(t *testing.T) {
	f := NewSourceLineRegexFilter([]string{"match"})
	source := `package p
func f() {
	match := 1
}
`
	firstPath := filepath.Join(t.TempDir(), "first.go")
	require.NoError(t, os.WriteFile(firstPath, []byte(source), 0644))
	firstFset := token.NewFileSet()
	firstFile, err := parser.ParseFile(firstFset, firstPath, source, 0)
	require.NoError(t, err)
	f.Collect(firstFile, firstFset, firstPath)

	secondSource := `package p
func f() {
	safe := 1
}
`
	secondFset := token.NewFileSet()
	secondFile, err := parser.ParseFile(secondFset, "second.go", secondSource, 0)
	require.NoError(t, err)
	f.Collect(secondFile, secondFset, "/nonexistent/path/that/does/not/exist.go")

	assignment := secondFile.Decls[0].(*ast.FuncDecl).Body.List[0]
	assert.False(t, f.ShouldSkipPosition(assignment.Pos(), "any"))
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
	assert.NotEmpty(t, f.skippedLines)

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

	assert.Empty(t, f.skippedLines)
}

func TestSourceLineRegexFilter_CollectUsesPhysicalLinesWithLineDirective(t *testing.T) {
	for _, tt := range []struct {
		name      string
		directive string
	}{
		{name: "named", directive: "//line fake.go:100"},
		{name: "filename-less", directive: "//line :100"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			src := "package p\n" + tt.directive + "\nfunc f() int { return 5 }\n"
			tmp := filepath.Join(t.TempDir(), "sample.go")
			require.NoError(t, os.WriteFile(tmp, []byte(src), 0644))

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, tmp, src, parser.ParseComments)
			require.NoError(t, err)

			f := NewSourceLineRegexFilter([]string{"return"})
			f.Collect(file, fset, tmp)

			statement := file.Decls[0].(*ast.FuncDecl).Body.List[0]
			assert.True(t, f.ShouldSkip(statement, "statement/remove"), "physical line containing return should be skipped")
			ret := statement.(*ast.ReturnStmt)
			assert.True(t, f.ShouldSkipPosition(ret.Results[0].Pos(), "statement/return"), "mutation position on the physical return line should be skipped")
		})
	}
}

func TestSourceLineRegexFilter_DoesNotSkipAdjustedLineCollision(t *testing.T) {
	src := "package p\n//line fake.go:2\nfunc f() int { return 5 }\n"
	tmp := filepath.Join(t.TempDir(), "sample.go")
	require.NoError(t, os.WriteFile(tmp, []byte(src), 0644))

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, tmp, src, parser.ParseComments)
	require.NoError(t, err)

	f := NewSourceLineRegexFilter([]string{"^//line"})
	f.Collect(file, fset, tmp)

	statement := file.Decls[0].(*ast.FuncDecl).Body.List[0]
	assert.False(t, f.ShouldSkip(statement, "statement/remove"), "directive's physical line must not skip the return on another physical line")
	ret := statement.(*ast.ReturnStmt)
	assert.False(t, f.ShouldSkipPosition(ret.Results[0].Pos(), "statement/return"), "an adjusted-line collision must not skip a return on another physical line")
}
