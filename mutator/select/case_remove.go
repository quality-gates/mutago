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
	original := append([]ast.Stmt(nil), n.Body.List...)

	for i, stmt := range original {
		comm, ok := stmt.(*ast.CommClause)
		if !ok || comm.Comm == nil {
			continue // skip default
		}

		li := i

		mutations = append(mutations, mutator.Mutation{
			Position: comm.Case,
			Change: func() {
				changed := make([]ast.Stmt, 0, len(original)-1)
				changed = append(changed, original[:li]...)
				changed = append(changed, original[li+1:]...)
				n.Body.List = changed
			},
			Reset: func() {
				n.Body.List = original
			},
		})
	}

	return mutations
}
