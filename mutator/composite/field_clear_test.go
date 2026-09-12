package composite

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

func BenchmarkMutatorFieldClearWideLiteral(b *testing.B) {
	elements := make([]ast.Expr, 1000)
	for i := range elements {
		elements[i] = &ast.KeyValueExpr{Key: ast.NewIdent("Field"), Value: ast.NewIdent("value")}
	}
	literal := &ast.CompositeLit{Elts: elements}
	mutations := MutatorFieldClear(nil, nil, literal)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for _, mutation := range mutations {
			mutation.Change()
			mutation.Reset()
		}
	}
}

func TestMutatorFieldClearRegistered(t *testing.T) {
	if _, err := mutator.New("composite/field-clear"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorFieldClear(t *testing.T) {
	test.Mutator(
		t,
		MutatorFieldClear,
		"../../testdata/composite/field_clear.go",
		4,
	)
}

func TestMutatorFieldClear_Safety(t *testing.T) {
	src := `package example

import "strings"

type Config struct {
	A string
	B string
	C string
	D string
}

func Build() Config {
	onlyLocal := "hello"
	multiLocal := "world"
	println(multiLocal)

	var other Config
	other.A = "existing"

	return Config{
		A: onlyLocal,                      // local with only use -> skipped
		B: multiLocal,                     // local with another use -> mutated
		C: other.A,                        // field selector, receiver has another use -> mutated
		D: strings.TrimSpace("sole import"), // strings is sole use in file -> skipped
	}
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
	ret := fn.Body.List[5].(*ast.ReturnStmt)
	lit := ret.Results[0].(*ast.CompositeLit)

	muts := MutatorFieldClear(pkg, info, lit)
	// Out of 4 fields (A, B, C, D):
	// A should be skipped (onlyLocal)
	// B should be mutated (multiLocal)
	// C should be mutated (other.A)
	// D should be skipped (sole import strings)
	if len(muts) != 2 {
		t.Fatalf("expected 2 mutations, got %d", len(muts))
	}
	// Verify positions correspond to B and C
	kvB := lit.Elts[1].(*ast.KeyValueExpr)
	kvC := lit.Elts[2].(*ast.KeyValueExpr)
	if muts[0].Position != kvB.Pos() {
		t.Errorf("expected mut 0 at B (%v), got %v", kvB.Pos(), muts[0].Position)
	}
	if muts[1].Position != kvC.Pos() {
		t.Errorf("expected mut 1 at C (%v), got %v", kvC.Pos(), muts[1].Position)
	}
}

func TestMutatorFieldClear_SkipsOnlyKeyUses(t *testing.T) {
	src := `package example

import "net/http"

func build() map[string]int {
	return map[string]int{
		http.MethodGet: 100,
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
	ret := fn.Body.List[0].(*ast.ReturnStmt)
	lit := ret.Results[0].(*ast.CompositeLit)
	kv := lit.Elts[0].(*ast.KeyValueExpr)
	if astutil.IsSafeToRemove(info, kv.Value) == false {
		t.Fatal("value should be safe to remove")
	}
	if astutil.IsSafeToRemove(info, kv) {
		t.Fatal("key-value expression should not be safe to remove")
	}

	if mutations := MutatorFieldClear(pkg, info, lit); len(mutations) != 0 {
		t.Fatalf("expected no mutations, got %d", len(mutations))
	}
}

func TestMutatorFieldClear_SkipsOnlyLocalKeyUse(t *testing.T) {
	src := `package example

func build() map[string]int {
	key := "hello"
	return map[string]int{
		key: 42,
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
	ret := fn.Body.List[1].(*ast.ReturnStmt)
	lit := ret.Results[0].(*ast.CompositeLit)

	if mutations := MutatorFieldClear(pkg, info, lit); len(mutations) != 0 {
		t.Fatalf("expected no mutations, got %d", len(mutations))
	}
}
