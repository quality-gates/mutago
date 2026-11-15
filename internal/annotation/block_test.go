package annotation

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockExclusionsReplacePreviousCollection(t *testing.T) {
	parse := func(src string) (*ast.File, *token.FileSet) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
		require.NoError(t, err)
		return file, fset
	}
	first, firstSet := parse("package p\nfunc f() {\n// mutator-disable-next-line *\nreturn\n}\n")
	statement := first.Decls[0].(*ast.FuncDecl).Body.List[0]
	NewProcessor().Collect(first, firstSet, "test.go")
	require.True(t, HandleBlockStmt(statement))
	second, secondSet := parse("package p\nfunc f() {\nreturn\n}\n")
	NewProcessor().Collect(second, secondSet, "test.go")
	require.False(t, HandleBlockStmt(statement))
}

func TestBlockExclusionsCanBeReadWhileCollected(t *testing.T) {
	src := "package p\nfunc f() {\n// mutator-disable-next-line *\nreturn\n}\n"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	require.NoError(t, err)
	statement := file.Decls[0].(*ast.FuncDecl).Body.List[0]
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for range 50 {
			NewProcessor().Collect(file, fset, "test.go")
		}
	}()
	go func() {
		defer workers.Done()
		for range 50 {
			HandleBlockStmt(statement)
		}
	}()
	workers.Wait()
	require.True(t, HandleBlockStmt(statement))
}

func TestHandleBlockStmt_LineAnnotation(t *testing.T) {
	src := `package main

func foo() int {
	// mutator-disable-next-line *
	return 1
	// mutator-disable-next-line statement/return
	return 2
	// mutator-disable-next-line statement/remove
	return 3
	return 4
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	require.NoError(t, err)

	p := NewProcessor()
	p.Collect(file, fset, "test.go")

	fn := file.Decls[0].(*ast.FuncDecl)
	require.Len(t, fn.Body.List, 4)

	ret1 := fn.Body.List[0]
	ret2 := fn.Body.List[1]
	ret3 := fn.Body.List[2]
	ret4 := fn.Body.List[3]

	// ret1: disabled for * (all mutators)
	assert.True(t, HandleBlockStmt(ret1, "statement/return"))
	assert.True(t, HandleBlockStmt(ret1, "statement/remove"))
	assert.True(t, HandleBlockStmt(ret1))

	// ret2: disabled for statement/return only
	assert.True(t, HandleBlockStmt(ret2, "statement/return"))
	assert.False(t, HandleBlockStmt(ret2, "statement/remove"))
	assert.False(t, HandleBlockStmt(ret2))

	// ret3: disabled for statement/remove only
	assert.False(t, HandleBlockStmt(ret3, "statement/return"))
	assert.True(t, HandleBlockStmt(ret3, "statement/remove"))
	assert.True(t, HandleBlockStmt(ret3))

	// ret4: unannotated return in the same block is NOT disabled
	assert.False(t, HandleBlockStmt(ret4, "statement/return"))
	assert.False(t, HandleBlockStmt(ret4, "statement/remove"))
	assert.False(t, HandleBlockStmt(ret4))
}

func TestHandleBlockStmt_RegexAnnotation(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "regex_block_test.go")
	src := `package main

// mutator-disable-regexp return.*special statement/return
func foo() int {
	return 1 // special
	return 2 // normal
}
`
	err := os.WriteFile(filePath, []byte(src), 0644)
	require.NoError(t, err)

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	require.NoError(t, err)

	p := NewProcessor()
	p.Collect(file, fset, filePath)

	fn := file.Decls[0].(*ast.FuncDecl)
	require.Len(t, fn.Body.List, 2)

	ret1 := fn.Body.List[0]
	ret2 := fn.Body.List[1]

	// ret1 matches regex for statement/return
	assert.True(t, HandleBlockStmt(ret1, "statement/return"))
	assert.False(t, HandleBlockStmt(ret1, "statement/remove"))

	// ret2 does not match regex
	assert.False(t, HandleBlockStmt(ret2, "statement/return"))
	assert.False(t, HandleBlockStmt(ret2, "statement/remove"))
}

func TestHandleBlockStmt_RegexAnnotation_Spaces(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "regex_spaces_block_test.go")
	src := `package main

// mutator-disable-regexp return true *
// mutator-disable-regexp return false branch/if
func foo(b bool) bool {
	if b {
		return true
	}
	return false
}
`
	err := os.WriteFile(filePath, []byte(src), 0644)
	require.NoError(t, err)

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	require.NoError(t, err)

	p := NewProcessor()
	p.Collect(file, fset, filePath)

	fn := file.Decls[0].(*ast.FuncDecl)
	require.Len(t, fn.Body.List, 2)

	ifStmt := fn.Body.List[0].(*ast.IfStmt)
	retTrue := ifStmt.Body.List[0]
	retFalse := fn.Body.List[1]

	// return true has wildcard mutator "*" -> suppresses all mutators
	assert.True(t, HandleBlockStmt(retTrue, "statement/return"))
	assert.True(t, HandleBlockStmt(retTrue, "branch/if"))
	assert.True(t, HandleBlockStmt(retTrue, "numbers/incrementer"))

	// return false has explicit mutator "branch/if" -> only suppresses branch/if
	assert.True(t, HandleBlockStmt(retFalse, "branch/if"))
	assert.False(t, HandleBlockStmt(retFalse, "statement/return"))
	assert.False(t, HandleBlockStmt(retFalse, "numbers/incrementer"))
}
