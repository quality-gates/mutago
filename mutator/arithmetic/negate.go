package arithmetic

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("arithmetic/negate", MutatorArithmeticNegate)
}

// MutatorArithmeticNegate inverts unary negation: -x becomes +x (effectively x).
// Mirrors gremlins' INVERT_NEGATIVES operator.
func MutatorArithmeticNegate(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.UnaryExpr)
	if !ok || n.Op != token.SUB {
		return nil
	}

	return []mutator.Mutation{
		{
			Change: func() { n.Op = token.ADD },
			Reset:  func() { n.Op = token.SUB },
		},
	}
}
