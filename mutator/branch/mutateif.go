package branch

import (
	"go/ast"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("branch/if", MutatorIf)
}

// MutatorIf implements a mutator for if and else if branches.
func MutatorIf(pkg *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.IfStmt)
	if !ok {
		return nil
	}
	return mutateBranchBody(pkg, info, n, n.Body.List, func(stmts []ast.Stmt) {
		n.Body.List = stmts
	}, statementPosition(n.Body))
}
