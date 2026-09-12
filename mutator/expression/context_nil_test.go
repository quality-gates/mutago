package expression

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

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

func TestMutatorContextNil_SkipsNonCallExpr(t *testing.T) {
	assert.Nil(t, MutatorContextNil(nil, nil, &ast.BasicLit{}))
}

func TestMutatorContextNil_Registered(t *testing.T) {
	_, err := mutator.New("expression/context-nil")
	assert.Nil(t, err)
}

func TestMutatorContextNil_SkipsSoleUseLocal(t *testing.T) {
	src := `package example

import "context"

func bar(c context.Context) {}

func foo() {
	ctx := context.Background()
	bar(ctx)
}

func baz(ctx context.Context) {
	bar(ctx)
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Scopes:     make(map[ast.Node]*types.Scope),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	pkg, err := (&types.Config{Importer: importer.Default()}).Check("example", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}

	var total int
	ast.Inspect(file, func(n ast.Node) bool {
		total += len(MutatorContextNil(pkg, info, n))
		return true
	})
	// foo's sole-use ctx skipped; baz's param ctx mutated
	if total != 1 {
		t.Fatalf("expected 1 mutation, got %d", total)
	}
}
