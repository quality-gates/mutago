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

func TestMutatorRemoveTerm_Safety(t *testing.T) {
	src := `package example

import "strings"

type S struct {
	Active bool
}

func Cases(flag bool) bool {
	onlyLocal := flag
	if onlyLocal && flag { // onlyLocal's only use is on LHS; RHS is flag (param)
		println("a")
	}

	multiLocal := flag
	if multiLocal && flag { // multiLocal also used on next line
		println(multiLocal)
	}

	var s S
	s.Active = true
	if s.Active && flag { // field selector on variable with another use
		println("c")
	}

	if strings.Contains("a", "b") && flag { // strings only use in file
		println("d")
	}

	return false
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

	fn := file.Decls[2].(*ast.FuncDecl)

	// 1. onlyLocal && flag: onlyLocal has only use removed -> LHS should be skipped, RHS should be mutated
	ifStmt1 := fn.Body.List[1].(*ast.IfStmt)
	muts1 := MutatorRemoveTerm(pkg, info, ifStmt1.Cond)
	if len(muts1) != 1 {
		t.Fatalf("expected 1 mutation for onlyLocal && flag, got %d", len(muts1))
	}
	// The 1 mutation should be for Y (flag), not X (onlyLocal)
	bin1 := ifStmt1.Cond.(*ast.BinaryExpr)
	if muts1[0].Position != bin1.Y.Pos() {
		t.Fatalf("expected mutation on Y (flag), got position of %v", muts1[0].Position)
	}

	// 2. multiLocal && flag: multiLocal has another use -> both X and Y mutated
	ifStmt2 := fn.Body.List[3].(*ast.IfStmt)
	muts2 := MutatorRemoveTerm(pkg, info, ifStmt2.Cond)
	if len(muts2) != 2 {
		t.Fatalf("expected 2 mutations for multiLocal && flag, got %d", len(muts2))
	}

	// 3. s.Active && flag: field selector with receiver used elsewhere -> both X and Y mutated
	ifStmt3 := fn.Body.List[6].(*ast.IfStmt)
	muts3 := MutatorRemoveTerm(pkg, info, ifStmt3.Cond)
	if len(muts3) != 2 {
		t.Fatalf("expected 2 mutations for s.Active && flag, got %d", len(muts3))
	}

	// 4. strings.Contains(...) && flag: strings is sole use in file -> LHS skipped, RHS mutated
	ifStmt4 := fn.Body.List[7].(*ast.IfStmt)
	muts4 := MutatorRemoveTerm(pkg, info, ifStmt4.Cond)
	if len(muts4) != 1 {
		t.Fatalf("expected 1 mutation for strings.Contains && flag, got %d", len(muts4))
	}
	bin4 := ifStmt4.Cond.(*ast.BinaryExpr)
	if muts4[0].Position != bin4.Y.Pos() {
		t.Fatalf("expected mutation on Y (flag), got position of %v", muts4[0].Position)
	}
}
