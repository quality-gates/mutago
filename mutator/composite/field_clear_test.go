package composite

import (
	"go/ast"
	"testing"

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
