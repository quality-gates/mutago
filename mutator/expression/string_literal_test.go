package expression

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorStringLiteral(t *testing.T) {
	test.Mutator(
		t,
		MutatorStringLiteral,
		"../../testdata/expression/string_literal.go",
		2,
	)
}

func TestMutatorStringLiteralRegistered(t *testing.T) {
	if _, err := mutator.New("expression/string-literal"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorStringLiteral_SkipsEmptyRawString(t *testing.T) {
	src := "package main\nfunc check(s string) bool { return s == `` }\n"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	var count int
	ast.Inspect(file, func(n ast.Node) bool {
		muts := MutatorStringLiteral(nil, nil, n)
		count += len(muts)
		return true
	})
	if count != 0 {
		t.Fatalf("expected 0 mutations for empty raw string literal, got %d", count)
	}
}
