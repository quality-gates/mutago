package concurrency

import (
	"go/ast"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("concurrency/goroutine-remove", MutatorGoroutineRemove)
}

// MutatorGoroutineRemove removes the go keyword from goroutine launches,
// making them run synchronously. This tests whether concurrent scheduling
// is actually required for correctness rather than just being incidental.
func MutatorGoroutineRemove(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
	var l []ast.Stmt

	switch n := node.(type) {
	case *ast.BlockStmt:
		l = n.List
	case *ast.CommClause:
		l = n.Body
	default:
		return nil
	}

	var mutations []mutator.Mutation

	for i, stmt := range l {
		goStmt, ok := stmt.(*ast.GoStmt)
		if !ok {
			continue
		}

		li := i
		original := stmt

		mutations = append(mutations, mutator.Mutation{
			Position: goStmt.Go,
			Change: func() {
				l[li] = &ast.ExprStmt{X: goStmt.Call}
			},
			Reset: func() {
				l[li] = original
			},
		})
	}

	return mutations
}
