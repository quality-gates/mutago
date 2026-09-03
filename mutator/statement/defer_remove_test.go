package statement

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorDeferRemove(t *testing.T) {
	test.Mutator(
		t,
		MutatorDeferRemove,
		"../../testdata/statement/defer_remove.go",
		2,
	)
}

func TestMutatorDeferRemoveSelect(t *testing.T) {
	test.Mutator(
		t,
		MutatorDeferRemove,
		"../../testdata/statement/defer_remove_select.go",
		1,
	)
}

func TestMutatorDeferRemoveRegistered(t *testing.T) {
	if _, err := mutator.New("statement/defer-remove"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorDeferRemoveSwitch(t *testing.T) {
	src := `package main
func f(x int) {
	switch x {
	case 1:
		defer func(){}()
	}
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	ast.Inspect(file, func(n ast.Node) bool {
		muts := MutatorDeferRemove(nil, nil, n)
		count += len(muts)
		return true
	})
	if count != 1 {
		t.Fatalf("expected 1 mutation for defer inside switch CaseClause, got %d", count)
	}
}
