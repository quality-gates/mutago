package astutil

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseAndTypeCheckSource(t *testing.T, source string) (*token.FileSet, *ast.File, *types.Package, *types.Info) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "example.go", source, parser.ParseComments)
	require.NoError(t, err)

	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Scopes:     make(map[ast.Node]*types.Scope),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	pkg, err := (&types.Config{Importer: importer.Default()}).Check("example", fset, []*ast.File{file}, info)
	require.NoError(t, err)

	return fset, file, pkg, info
}

func TestUnsafeLocalVars_OnlyUse(t *testing.T) {
	src := `package example

func Foo() int {
	x := 42
	return x
}
`
	_, file, _, info := parseAndTypeCheckSource(t, src)
	fn := file.Decls[0].(*ast.FuncDecl)
	ret := fn.Body.List[1].(*ast.ReturnStmt)
	expr := ret.Results[0]

	unsafe := UnsafeLocalVars(info, expr)
	require.Len(t, unsafe, 1)
	assert.Equal(t, "x", unsafe[0].(*ast.Ident).Name)
	assert.False(t, IsSafeToRemove(info, expr))
}

func TestUnsafeLocalVars_AnotherUse(t *testing.T) {
	src := `package example

func Foo() int {
	x := 42
	println(x)
	return x
}
`
	_, file, _, info := parseAndTypeCheckSource(t, src)
	fn := file.Decls[0].(*ast.FuncDecl)
	ret := fn.Body.List[2].(*ast.ReturnStmt)
	expr := ret.Results[0]

	unsafe := UnsafeLocalVars(info, expr)
	assert.Empty(t, unsafe)
	assert.True(t, IsSafeToRemove(info, expr))
}

func TestUnsafeLocalVars_Param(t *testing.T) {
	src := `package example

func Foo(p int) int {
	return p
}
`
	_, file, _, info := parseAndTypeCheckSource(t, src)
	fn := file.Decls[0].(*ast.FuncDecl)
	ret := fn.Body.List[0].(*ast.ReturnStmt)
	expr := ret.Results[0]

	unsafe := UnsafeLocalVars(info, expr)
	assert.Empty(t, unsafe)
	assert.True(t, IsSafeToRemove(info, expr))
}

func TestUnsafeLocalVars_FieldSelector(t *testing.T) {
	src := `package example

type S struct {
	Field int
}

var Global S

func Foo() int {
	return Global.Field
}
`
	_, file, _, info := parseAndTypeCheckSource(t, src)
	fn := file.Decls[2].(*ast.FuncDecl)
	ret := fn.Body.List[0].(*ast.ReturnStmt)
	expr := ret.Results[0]

	unsafe := UnsafeLocalVars(info, expr)
	assert.Empty(t, unsafe)
	assert.True(t, IsSafeToRemove(info, expr))
}

func TestHasUnsafeImport_SoleUse(t *testing.T) {
	src := `package example

import "strings"

func Foo() string {
	return strings.TrimSpace("abc")
}
`
	_, file, _, info := parseAndTypeCheckSource(t, src)
	fn := file.Decls[1].(*ast.FuncDecl)
	ret := fn.Body.List[0].(*ast.ReturnStmt)
	expr := ret.Results[0]

	assert.True(t, HasUnsafeImport(info, expr))
	assert.False(t, IsSafeToRemove(info, expr))
}

func TestHasUnsafeImport_MultiUse(t *testing.T) {
	src := `package example

import "strings"

func Bar() {
	println(strings.ToLower("xyz"))
}

func Foo() string {
	return strings.TrimSpace("abc")
}
`
	_, file, _, info := parseAndTypeCheckSource(t, src)
	fn := file.Decls[2].(*ast.FuncDecl)
	ret := fn.Body.List[0].(*ast.ReturnStmt)
	expr := ret.Results[0]

	assert.False(t, HasUnsafeImport(info, expr))
	assert.True(t, IsSafeToRemove(info, expr))
}

func TestCreateNoopOfExpressions(t *testing.T) {
	pos := token.Pos(10)
	ids := []ast.Expr{
		&ast.Ident{Name: "a"},
		&ast.Ident{Name: "b"},
	}
	stmt := CreateNoopOfExpressions(ids, pos)
	assign, ok := stmt.(*ast.AssignStmt)
	require.True(t, ok)
	assert.Equal(t, token.ASSIGN, assign.Tok)
	assert.Equal(t, pos, assign.TokPos)
	require.Len(t, assign.Lhs, 2)
	assert.Equal(t, "_", assign.Lhs[0].(*ast.Ident).Name)
	assert.Equal(t, "_", assign.Lhs[1].(*ast.Ident).Name)
	require.Len(t, assign.Rhs, 2)
	assert.Equal(t, "a", assign.Rhs[0].(*ast.Ident).Name)
	assert.Equal(t, "b", assign.Rhs[1].(*ast.Ident).Name)
}
