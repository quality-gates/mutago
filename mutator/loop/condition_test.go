package loop

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorLoopCondition(t *testing.T) {
	test.Mutator(
		t,
		MutatorLoopCondition,
		"../../testdata/loop/condition.go",
		2,
	)
}

func TestMutatorLoopConditionNeq(t *testing.T) {
	test.Mutator(
		t,
		MutatorLoopCondition,
		"../../testdata/loop/condition_neq.go",
		1,
	)
}

func TestMutatorLoopConditionRegistered(t *testing.T) {
	if _, err := mutator.New("loop/condition"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorLoopCondition_SkipsSoleUseLocal(t *testing.T) {
	src := `package example

func foo() {
	limit := 10
	for 0 < limit {
		doSomething()
	}
}

func bar() {
	k := 0
	for k < 100 {
		k = k + 1
	}
}

func doSomething() {}
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
		total += len(MutatorLoopCondition(pkg, info, n))
		return true
	})
	// limit sole-use skipped; k reused in body mutated
	if total != 1 {
		t.Fatalf("expected 1 mutation, got %d", total)
	}
}
