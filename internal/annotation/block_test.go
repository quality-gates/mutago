package annotation

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
