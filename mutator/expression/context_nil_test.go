package expression

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/quality-gates/mutago/v2/astutil"
	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
	"github.com/stretchr/testify/assert"
)

func TestMutatorContextNil(t *testing.T) {
	test.Mutator(
		t,
		MutatorContextNil,
		"../../testdata/expression/context_nil.go",
		1,
	)
}

func TestMutatorContextNil_SkipsNilInfo(t *testing.T) {
	call := &ast.CallExpr{Args: []ast.Expr{ast.NewIdent("ctx")}}
	assert.Nil(t, MutatorContextNil(nil, nil, call))
}

func TestMutatorContextNil_SkipsOnlyContextVariableUse(t *testing.T) {
	src := `package example

import "context"

func foo() {
	var ctx context.Context
	bar(ctx)
}

func bar(any) {}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "example.go", src, 0)
	assert.NoError(t, err)

	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Scopes:     make(map[ast.Node]*types.Scope),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	pkg, err := (&types.Config{Importer: importer.Default()}).Check("example", fset, []*ast.File{file}, info)
	assert.NoError(t, err)

	fn := file.Decls[1].(*ast.FuncDecl)
	call := fn.Body.List[1].(*ast.ExprStmt).X.(*ast.CallExpr)

	assert.False(t, astutil.IsSafeToRemove(info, call.Args[0]))
	assert.Empty(t, MutatorContextNil(pkg, info, call))
}

func TestMutatorContextNil_SkipsOnlyContextImportUse(t *testing.T) {
	src := `package example

import "context"

func foo() {
	bar(context.Background())
}

func bar(any) {}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "example.go", src, 0)
	assert.NoError(t, err)

	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Scopes:     make(map[ast.Node]*types.Scope),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	pkg, err := (&types.Config{Importer: importer.Default()}).Check("example", fset, []*ast.File{file}, info)
	assert.NoError(t, err)

	fn := file.Decls[1].(*ast.FuncDecl)
	call := fn.Body.List[0].(*ast.ExprStmt).X.(*ast.CallExpr)

	assert.False(t, astutil.IsSafeToRemove(info, call.Args[0]))
	assert.Empty(t, MutatorContextNil(pkg, info, call))
}

func TestMutatorContextNil_SkipsNonCallExpr(t *testing.T) {
	assert.Nil(t, MutatorContextNil(nil, nil, &ast.BasicLit{}))
}

func TestMutatorContextNil_Registered(t *testing.T) {
	_, err := mutator.New("expression/context-nil")
	assert.Nil(t, err)
}
