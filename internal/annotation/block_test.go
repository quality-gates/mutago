package annotation

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func TestHandleBlockStmt_HonorsMutatorName(t *testing.T) {
	src := `package main
func Inc(x int) int {
	// mutator-disable-next-line statement/return
	return x + 1
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "inc.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	p := NewProcessor()
	p.Collect(file, fset, "inc.go")

	var ret *ast.ReturnStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if r, ok := n.(*ast.ReturnStmt); ok {
			ret = r
			return false
		}
		return true
	})
	if ret == nil {
		t.Fatal("no return stmt")
	}

	if !HandleBlockStmt(ret, "statement/return") {
		t.Fatal("expected statement/return to skip annotated return")
	}
	if HandleBlockStmt(ret, "statement/remove") {
		t.Fatal("statement/remove should not skip when only statement/return is annotated")
	}
}

func TestHandleBlockStmt_StarSkipsBoth(t *testing.T) {
	src := `package main
func Inc(x int) int {
	// mutator-disable-next-line *
	return x + 1
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "inc.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	p := NewProcessor()
	p.Collect(file, fset, "inc.go")

	var ret *ast.ReturnStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if r, ok := n.(*ast.ReturnStmt); ok {
			ret = r
			return false
		}
		return true
	})
	if ret == nil {
		t.Fatal("no return stmt")
	}

	if !HandleBlockStmt(ret, "statement/return") {
		t.Fatal("expected * to skip statement/return")
	}
	if !HandleBlockStmt(ret, "statement/remove") {
		t.Fatal("expected * to skip statement/remove")
	}
}

func TestHandleBlockStmt_RegexpReturn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inc.go")
	src := `package main
// mutator-disable-regexp return *
func Inc(x int) int {
	return x + 1
}
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	p := NewProcessor()
	p.Collect(file, fset, path)

	var ret *ast.ReturnStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if r, ok := n.(*ast.ReturnStmt); ok {
			ret = r
			return false
		}
		return true
	})
	if ret == nil {
		t.Fatal("no return stmt")
	}
	if !HandleBlockStmt(ret, "statement/return") {
		t.Fatal("expected regexp * to skip statement/return")
	}
}
