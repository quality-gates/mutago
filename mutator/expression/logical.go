package expression

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("expression/logical", MutatorLogical)
}

// MutatorLogical swaps && and || operators.
func MutatorLogical(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.BinaryExpr)
	if !ok {
		return nil
	}

	var mutated token.Token
	switch n.Op {
	case token.LAND:
		mutated = token.LOR
	case token.LOR:
		mutated = token.LAND
	default:
		return nil
	}

	original := n.Op
	return []mutator.Mutation{
		{
			Change: func() { n.Op = mutated },
			Reset:  func() { n.Op = original },
		},
	}
}
