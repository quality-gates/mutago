package branch

import (
	"go/ast"
	"go/types"

	"github.com/quality-gates/mutago/v2/astutil"
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
	// We ignore else ifs, nil blocks, and already-empty else bodies.
	block, ok := n.Else.(*ast.BlockStmt)
	if !ok || len(block.List) == 0 {
		return nil
	}

	old := n.Else

	return []mutator.Mutation{
		{
			Position: statementPosition(old),
			Change: func() {
				n.Else = astutil.CreateNoopOfStatement(pkg, info, old)
			},
			Reset: func() {
				n.Else = old
			},
		},
	}
}
