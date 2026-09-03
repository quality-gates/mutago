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

func TestMutatorReturnValue(t *testing.T) {
	test.Mutator(
		t,
		MutatorReturnValue,
		"../../testdata/statement/return.go",
		3,
	)
}

func TestMutatorReturnValuePointer(t *testing.T) {
	test.Mutator(
		t,
		MutatorReturnValue,
		"../../testdata/statement/return_pointer.go",
		2,
	)
}

func TestMutatorReturnValueRegistered(t *testing.T) {
	if _, err := mutator.New("statement/return"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorReturnValue_SkipsEmptyRawString(t *testing.T) {
	src := "package main\nfunc empty() string { return `` }\n"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{}
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
	}
	_, err = conf.Check("main", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}

	var count int
	ast.Inspect(file, func(n ast.Node) bool {
		muts := MutatorReturnValue(nil, info, n)
		count += len(muts)
		return true
	})
	if count != 0 {
		t.Fatalf("expected 0 mutations for empty raw string, got %d", count)
	}
}
