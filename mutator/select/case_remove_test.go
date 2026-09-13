package selectmutator

import (
	"bytes"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"runtime"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func BenchmarkMutatorSelectCaseRemoveWideSelect(b *testing.B) {
	clauses := make([]ast.Stmt, 1000)
	for i := range clauses {
		clauses[i] = &ast.CommClause{Case: token.Pos(i + 1), Comm: &ast.SendStmt{}}
	}
	node := &ast.SelectStmt{Body: &ast.BlockStmt{List: clauses}}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		runtime.KeepAlive(MutatorSelectCaseRemove(nil, nil, node))
	}
}

func TestMutatorSelectCaseRemoveRegistered(t *testing.T) {
	if _, err := mutator.New("select/case-remove"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorSelectCaseRemove(t *testing.T) {
	test.Mutator(
		t,
		MutatorSelectCaseRemove,
		"../../testdata/select/case_remove.go",
		1,
	)
}

func TestMutatorSelectCaseRemoveSkipsUncompilableMutants(t *testing.T) {
	const source = `package example

import "time"

func Wait(ch chan int) {
	select {
	case <-time.After(time.Second):
	case <-ch:
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

	mutations := MutatorSelectCaseRemove(pkg, info, selectStmt)
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

	if len(mutations) != 1 {
		t.Fatalf("got %d mutations, want only the compilable case mutation", len(mutations))
	}
}
