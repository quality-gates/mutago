package selectmutator

import (
	"go/ast"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("select/default-remove", MutatorSelectDefaultRemove)
}

// MutatorSelectDefaultRemove removes the default clause from a select statement.
// This turns a non-blocking select into a blocking one, catching tests that
// verify the program does not stall when no channel is ready.
func MutatorSelectDefaultRemove(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.SelectStmt)
	if !ok {
		return nil
	}

	for i, stmt := range n.Body.List {
		comm, ok := stmt.(*ast.CommClause)
		if !ok || comm.Comm != nil {
			continue // skip non-default cases
		}

		li := i
		original := make([]ast.Stmt, len(n.Body.List))
		copy(original, n.Body.List)

		return []mutator.Mutation{
			{
				Change: func() {
					n.Body.List = append(n.Body.List[:li], n.Body.List[li+1:]...)
				},
				Reset: func() {
					n.Body.List = original
				},
			},
		}
	}

	return nil
}
