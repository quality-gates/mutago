package branch

import (
	"go/ast"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("branch/else", MutatorElse)
}

// MutatorElse implements a mutator for else branches.
func MutatorElse(pkg *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.IfStmt)
	if !ok {
		return nil
	}
	block, ok := n.Else.(*ast.BlockStmt)
	if !ok {
		return nil
	}
	return mutateBranchBody(pkg, info, n, block.List, func(stmts []ast.Stmt) {
		block.List = stmts
	}, statementPosition(block))
}
