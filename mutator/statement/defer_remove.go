package statement

import (
	"go/ast"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("statement/defer-remove", MutatorDeferRemove)
}

// MutatorDeferRemove removes the defer keyword, turning deferred calls into
// immediate calls. This tests whether the timing of cleanup matters for
// correctness — e.g. mutex unlocks, file closes, and span finishes that must
// happen after (not during) the function body.
func MutatorDeferRemove(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
	var l []ast.Stmt

	switch n := node.(type) {
	case *ast.BlockStmt:
		l = n.List
	case *ast.CaseClause:
		l = n.Body
	case *ast.CommClause:
		l = n.Body
	default:
		return nil
	}

	var mutations []mutator.Mutation

	for i, stmt := range l {
		deferStmt, ok := stmt.(*ast.DeferStmt)
		if !ok {
			continue
		}

		li := i
		original := stmt

		mutations = append(mutations, mutator.Mutation{
			Position: deferStmt.Defer,
			Change: func() {
				l[li] = &ast.ExprStmt{X: deferStmt.Call}
			},
			Reset: func() {
				l[li] = original
			},
		})
	}

	return mutations
}
