package selectmutator

import (
	"go/ast"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("select/case-remove", MutatorSelectCaseRemove)
}

// MutatorSelectCaseRemove removes one non-default case at a time from a select statement.
// Each mutation tests whether the program still behaves correctly when a particular
// channel event can never fire.
func MutatorSelectCaseRemove(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.SelectStmt)
	if !ok {
		return nil
	}

	// Removing the only case produces an empty select{} that blocks forever;
	// only generate mutations when at least one case will remain.
	if len(n.Body.List) < 2 {
		return nil
	}

	var mutations []mutator.Mutation

	for i, stmt := range n.Body.List {
		comm, ok := stmt.(*ast.CommClause)
		if !ok || comm.Comm == nil {
			continue // skip default
		}

		li := i
		original := make([]ast.Stmt, len(n.Body.List))
		copy(original, n.Body.List)

		mutations = append(mutations, mutator.Mutation{
			Change: func() {
				n.Body.List = append(n.Body.List[:li], n.Body.List[li+1:]...)
			},
			Reset: func() {
				n.Body.List = original
			},
		})
	}

	return mutations
}
