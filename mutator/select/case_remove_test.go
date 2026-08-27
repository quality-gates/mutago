package selectmutator

import (
	"go/ast"
	"go/token"
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
		2,
	)
}
