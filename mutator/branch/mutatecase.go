package branch

import (
	"go/ast"
	"go/types"

	"github.com/quality-gates/mutago/v2/astutil"
	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("branch/case", MutatorCase)
}

// MutatorCase implements a mutator for case clauses.
func MutatorCase(pkg *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.CaseClause)
	if !ok {
		return nil
	}
	if len(n.Body) == 0 {
		return nil
	}

	old := n.Body
	position := statementPosition(n.Body[0])

	return []mutator.Mutation{
		{
			Position: position,
			Change: func() {
				n.Body = []ast.Stmt{
					astutil.CreateNoopOfStatements(pkg, info, n.Body),
				}
			},
			Reset: func() {
				n.Body = old
			},
		},
	}
}
