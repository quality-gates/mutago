package numbers

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("numbers/float-negate", MutatorFloatNegate)
}

// MutatorFloatNegate replaces a non-zero float literal with 0.0.
// Using 0.0 (rather than prepending "-") avoids producing --x when the literal
// is already the operand of a unary minus expression.
func MutatorFloatNegate(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.BasicLit)
	if !ok || n.Kind != token.FLOAT {
		return nil
	}

	f, err := strconv.ParseFloat(n.Value, 64)
	if err != nil || f == 0 {
		return nil
	}

	original := n.Value
	return []mutator.Mutation{
		{
			Position: n.ValuePos,
			Change:   func() { n.Value = "0.0" },
			Reset:    func() { n.Value = original },
		},
	}
}
