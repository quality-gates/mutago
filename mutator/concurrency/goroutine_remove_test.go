package concurrency

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorGoroutineRemoveRegistered(t *testing.T) {
	if _, err := mutator.New("concurrency/goroutine-remove"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorGoroutineRemove(t *testing.T) {
	test.Mutator(
		t,
		MutatorGoroutineRemove,
		"../../testdata/concurrency/goroutine_remove.go",
		2,
	)
}

func TestMutatorGoroutineRemoveSwitch(t *testing.T) {
	src := `package main
func f(x int) {
	switch x {
	case 1:
		go func(){}()
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
		muts := MutatorGoroutineRemove(nil, nil, n)
		count += len(muts)
		return true
	})
	if count != 1 {
		t.Fatalf("expected 1 mutation for go inside switch CaseClause, got %d", count)
	}
}
