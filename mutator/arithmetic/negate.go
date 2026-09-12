package arithmetic

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("arithmetic/negate", MutatorArithmeticNegate)
}

const (
	minInt8Magnitude  = 1 << 7  // 128
	minInt16Magnitude = 1 << 15 // 32768
	minInt32Magnitude = 1 << 31 // 2147483648
	minInt64Magnitude = 1 << 63 // 9223372036854775808
)

func unwrapParen(expr ast.Expr) ast.Expr {
	for {
		p, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = p.X
	}
}

// isSignedMinBoundary checks if a unary minus expression represents a signed integer
// minimum boundary constant (e.g. -128, -32768, -2147483648, -9223372036854775808).
// Mutating -MinInt to +MinInt produces integer constant overflow (+MinInt > MaxInt).
func isSignedMinBoundary(n *ast.UnaryExpr) bool {
	expr := unwrapParen(n.X)
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return false
	}

	cleaned := strings.ReplaceAll(lit.Value, "_", "")
	val, err := strconv.ParseUint(cleaned, 0, 64)
	if err != nil {
		return false
	}

	switch val {
	case minInt8Magnitude, minInt16Magnitude, minInt32Magnitude:
		return true
	default:
		return val >= minInt64Magnitude
	}
}

// MutatorArithmeticNegate inverts unary negation: -x becomes +x (effectively x).
// Mirrors gremlins' INVERT_NEGATIVES operator.
func MutatorArithmeticNegate(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.UnaryExpr)
	if !ok || n.Op != token.SUB {
		return nil
	}

	if isSignedMinBoundary(n) {
		return nil
	}

	return []mutator.Mutation{
		{
			Position: n.OpPos,
			Change:   func() { n.Op = token.ADD },
			Reset:    func() { n.Op = token.SUB },
		},
	}
}
