package branch

import (
	"go/ast"
	"go/types"

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
	return mutateBranchBody(pkg, info, n, n.Body, func(stmts []ast.Stmt) {
		n.Body = stmts
	}, statementPosition(n.Body[0]))
}
