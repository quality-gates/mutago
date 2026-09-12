package loop

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

func TestMutatorLoopCondition_SkipsOnlyConditionUse(t *testing.T) {
	src := `package example

func run() {
	limit := 10
	for 0 < limit {
	}
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "example.go", src, 0)
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

	fn := file.Decls[0].(*ast.FuncDecl)
	loop := fn.Body.List[1].(*ast.ForStmt)

	if astutil.IsSafeToRemove(info, loop.Cond) {
		t.Fatal("condition should not be safe to remove")
	}
	if mutations := MutatorLoopCondition(pkg, info, loop); len(mutations) != 0 {
		t.Fatalf("expected no mutations, got %d", len(mutations))
	}
}

func TestMutatorLoopCondition_SkipsOnlyConditionImportUse(t *testing.T) {
	src := `package example

import "strings"

func run() {
	for 0 < len(strings.Fields("")) {
	}
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "example.go", src, 0)
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

	fn := file.Decls[1].(*ast.FuncDecl)
	loop := fn.Body.List[0].(*ast.ForStmt)

	if astutil.IsSafeToRemove(info, loop.Cond) {
		t.Fatal("condition should not be safe to remove")
	}
	if mutations := MutatorLoopCondition(pkg, info, loop); len(mutations) != 0 {
		t.Fatalf("expected no mutations, got %d", len(mutations))
	}
}
