package selectmutator

import (
	"bytes"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorSelectDefaultRemoveRegistered(t *testing.T) {
	if _, err := mutator.New("select/default-remove"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorSelectDefaultRemove(t *testing.T) {
	test.Mutator(
		t,
		MutatorSelectDefaultRemove,
		"../../testdata/select/default_remove.go",
		1,
	)
}

func TestMutatorSelectDefaultRemoveSkipsUncompilableMutants(t *testing.T) {
	const source = `package example

func Send(ch chan int, x int) {
	v := x + 1
	select {
	case <-ch:
	default:
		ch <- v
	}
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "example.go", source, 0)
	if err != nil {
		t.Fatalf("parse source: %v", err)
	}

	info := &types.Info{
		Defs:   make(map[*ast.Ident]types.Object),
		Uses:   make(map[*ast.Ident]types.Object),
		Scopes: make(map[ast.Node]*types.Scope),
	}
	pkg, err := (&types.Config{Importer: importer.Default()}).Check("example", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatalf("type-check source: %v", err)
	}

	var selectStmt *ast.SelectStmt
	ast.Inspect(file, func(node ast.Node) bool {
		if stmt, ok := node.(*ast.SelectStmt); ok {
			selectStmt = stmt
			return false
		}
		return true
	})
	if selectStmt == nil {
		t.Fatal("source did not contain a select statement")
	}

	mutations := MutatorSelectDefaultRemove(pkg, info, selectStmt)
	for _, mutation := range mutations {
		mutation.Change()

		var mutated bytes.Buffer
		if err := printer.Fprint(&mutated, fset, file); err != nil {
			t.Fatalf("print mutated source: %v", err)
		}

		mutatedFset := token.NewFileSet()
		mutatedFile, err := parser.ParseFile(mutatedFset, "example.go", mutated.String(), 0)
		if err != nil {
			t.Fatalf("parse mutated source: %v", err)
		}
		mutatedInfo := &types.Info{
			Defs:   make(map[*ast.Ident]types.Object),
			Uses:   make(map[*ast.Ident]types.Object),
			Scopes: make(map[ast.Node]*types.Scope),
		}
		_, err = (&types.Config{Importer: importer.Default()}).Check("example", mutatedFset, []*ast.File{mutatedFile}, mutatedInfo)
		mutation.Reset()
		if err != nil {
			t.Fatalf("mutated source does not compile: %v", err)
		}
	}

	if len(mutations) != 0 {
		t.Fatalf("got %d mutations, want no mutation that leaves a local variable unused", len(mutations))
	}
}

func TestMutatorSelectDefaultRemoveSkipsSoleDefault(t *testing.T) {
	const source = `package example

func Run() {
	select {
	default:
		println("done")
	}
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "example.go", source, 0)
	if err != nil {
		t.Fatalf("parse source: %v", err)
	}

	var selectStmt *ast.SelectStmt
	ast.Inspect(file, func(node ast.Node) bool {
		if stmt, ok := node.(*ast.SelectStmt); ok {
			selectStmt = stmt
			return false
		}
		return true
	})
	if selectStmt == nil {
		t.Fatal("source did not contain a select statement")
	}

	if got := MutatorSelectDefaultRemove(nil, &types.Info{}, selectStmt); len(got) != 0 {
		t.Fatalf("got %d mutations, want none: removing the sole clause yields a blocking select {}", len(got))
	}
}
