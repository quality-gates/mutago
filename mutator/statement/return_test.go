package statement

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

func TestMutatorReturnValue_Safety(t *testing.T) {
	src := `package example

import (
	"strings"
)

type S struct {
	Field int
}

func Cases(param int) int {
	onlyLocal := 10
	if param > 0 {
		return onlyLocal // 1. local with only use -> preceded by _ = onlyLocal
	}

	multiLocal := 20
	println(multiLocal)
	if param < 0 {
		return multiLocal // 2. local with another use -> plain return 0
	}

	var s S
	s.Field = 30
	if param == 0 {
		return s.Field // 3. field selector, receiver used elsewhere -> plain return 0
	}

	return len(strings.TrimSpace("abc")) // 4. strings is sole use in file -> skipped
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
	muts := MutatorReturnValue(pkg, info, fn.Body)

	// In fn.Body:
	// - Case 1 (onlyLocal) is inside fn.Body.List[1] (ifStmt). Body of ifStmt has the return.
	// When MutatorReturnValue is called on fn.Body, it inspects statements directly in fn.Body.
	// The return statements are inside if-statement bodies!
	// So calling MutatorReturnValue on each block statement:
	ifStmt1 := fn.Body.List[1].(*ast.IfStmt)
	muts1 := MutatorReturnValue(pkg, info, ifStmt1.Body)
	if len(muts1) != 1 {
		t.Fatalf("expected 1 mutation for onlyLocal, got %d", len(muts1))
	}
	// Verify that applying muts1 inserts `_ = onlyLocal` before `return 0`
	muts1[0].Change()
	if len(ifStmt1.Body.List) != 2 {
		t.Fatalf("expected 2 statements after mutation (noop + return), got %d", len(ifStmt1.Body.List))
	}
	noop, ok := ifStmt1.Body.List[0].(*ast.AssignStmt)
	if !ok || len(noop.Lhs) != 1 || noop.Lhs[0].(*ast.Ident).Name != "_" {
		t.Fatalf("expected blank assign as first statement, got %T", ifStmt1.Body.List[0])
	}
	muts1[0].Reset()
	if len(ifStmt1.Body.List) != 1 {
		t.Fatalf("expected 1 statement after reset, got %d", len(ifStmt1.Body.List))
	}

	// Case 2 (multiLocal): inside ifStmt2.Body
	ifStmt2 := fn.Body.List[4].(*ast.IfStmt)
	muts2 := MutatorReturnValue(pkg, info, ifStmt2.Body)
	if len(muts2) != 1 {
		t.Fatalf("expected 1 mutation for multiLocal, got %d", len(muts2))
	}
	// Applying muts2 should NOT insert a noop statement (list length stays 1)
	muts2[0].Change()
	if len(ifStmt2.Body.List) != 1 {
		t.Fatalf("expected 1 statement for multiLocal (no noop), got %d", len(ifStmt2.Body.List))
	}
	muts2[0].Reset()

	// Case 3 (s.Field): inside ifStmt3.Body
	ifStmt3 := fn.Body.List[7].(*ast.IfStmt)
	muts3 := MutatorReturnValue(pkg, info, ifStmt3.Body)
	if len(muts3) != 1 {
		t.Fatalf("expected 1 mutation for s.Field, got %d", len(muts3))
	}
	muts3[0].Change()
	if len(ifStmt3.Body.List) != 1 {
		t.Fatalf("expected 1 statement for s.Field (no noop), got %d", len(ifStmt3.Body.List))
	}
	muts3[0].Reset()

	// Case 4 (strings sole use in file): the final return in fn.Body
	// muts was collected on fn.Body directly, which has the final return
	if len(muts) != 0 {
		t.Fatalf("expected final return to be skipped due to sole import, got %d mutations", len(muts))
	}
}
