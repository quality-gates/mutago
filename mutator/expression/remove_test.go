package expression

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorRemoveTermRegistered(t *testing.T) {
	if _, err := mutator.New("expression/remove"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorRemoveTerm(t *testing.T) {
	test.Mutator(
		t,
		MutatorRemoveTerm,
		"../../testdata/expression/remove.go",
		6,
	)
}

func TestMutatorRemoveTerm_SkipsEquivalent(t *testing.T) {
	// true && x should only mutate y (x is already true)
	exprAnd := &ast.BinaryExpr{
		X:  ast.NewIdent("true"),
		Op: token.LAND,
		Y:  ast.NewIdent("x"),
	}
	mutsAnd := MutatorRemoveTerm(nil, nil, exprAnd)
	if len(mutsAnd) != 1 {
		t.Fatalf("expected 1 mutation for true && x, got %d", len(mutsAnd))
	}

	// false || x should only mutate y (x is already false)
	exprOr := &ast.BinaryExpr{
		X:  ast.NewIdent("false"),
		Op: token.LOR,
		Y:  ast.NewIdent("x"),
	}
	mutsOr := MutatorRemoveTerm(nil, nil, exprOr)
	if len(mutsOr) != 1 {
		t.Fatalf("expected 1 mutation for false || x, got %d", len(mutsOr))
	}

	// x && true should only mutate x
	exprAndY := &ast.BinaryExpr{
		X:  ast.NewIdent("x"),
		Op: token.LAND,
		Y:  ast.NewIdent("true"),
	}
	mutsAndY := MutatorRemoveTerm(nil, nil, exprAndY)
	if len(mutsAndY) != 1 {
		t.Fatalf("expected 1 mutation for x && true, got %d", len(mutsAndY))
	}

	// x || false should only mutate x
	exprOrY := &ast.BinaryExpr{
		X:  ast.NewIdent("x"),
		Op: token.LOR,
		Y:  ast.NewIdent("false"),
	}
	mutsOrY := MutatorRemoveTerm(nil, nil, exprOrY)
	if len(mutsOrY) != 1 {
		t.Fatalf("expected 1 mutation for x || false, got %d", len(mutsOrY))
	}
}

func TestIsIdent(t *testing.T) {
	if isIdent(&ast.BasicLit{}, "true") {
		t.Fatal("expected false for non-ident")
	}
	if isIdent(ast.NewIdent("false"), "true") {
		t.Fatal("expected false for mismatched ident")
	}
	if !isIdent(ast.NewIdent("true"), "true") {
		t.Fatal("expected true for matching ident")
	}
}
