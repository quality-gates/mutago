package statement

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorRemoveStatement(t *testing.T) {
	test.Mutator(
		t,
		MutatorRemoveStatement,
		"../../testdata/statement/remove.go",
		17,
	)
}

func TestMutatorRemoveStatementRegistered(t *testing.T) {
	if _, err := mutator.New("statement/remove"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorRemoveStatement_SelectCommClause(t *testing.T) {
	src := `package main
func sel(ch chan int) {
	select {
	case <-ch:
		println("received")
	}
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{}
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	pkg, err := conf.Check("main", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}

	var count int
	ast.Inspect(file, func(n ast.Node) bool {
		muts := MutatorRemoveStatement(pkg, info, n)
		count += len(muts)
		return true
	})
	if count != 1 {
		t.Fatalf("expected 1 mutation for statement inside select CommClause, got %d", count)
	}
}
