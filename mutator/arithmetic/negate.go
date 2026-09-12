package arithmetic

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("arithmetic/negate", MutatorArithmeticNegate)
}

// MutatorArithmeticNegate inverts unary negation: -x becomes +x (effectively x).
// Mirrors gremlins' INVERT_NEGATIVES operator.
// Skips typed MinInt constants: flipping -MinInt to +MinInt overflows and will not compile.
func MutatorArithmeticNegate(_ *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.UnaryExpr)
	if !ok || n.Op != token.SUB {
		return nil
	}

	if negateInvertOverflows(info, n) {
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

// negateInvertOverflows is true when the unary minus is a typed signed MinInt
// constant; inverting to + yields a magnitude past MaxInt for that type.
func negateInvertOverflows(info *types.Info, n *ast.UnaryExpr) bool {
	if info == nil {
		return false
	}
	tv, ok := info.Types[n]
	if !ok || tv.Type == nil || tv.Value == nil {
		return false
	}
	basic, ok := types.Unalias(tv.Type).Underlying().(*types.Basic)
	if !ok || basic.Info()&types.IsInteger == 0 || basic.Info()&types.IsUntyped != 0 {
		return false
	}
	min, ok := minIntOfSigned(basic)
	if !ok {
		return false
	}
	val, exact := constant.Int64Val(constant.ToInt(tv.Value))
	return exact && val == min
}

func minIntOfSigned(basic *types.Basic) (int64, bool) {
	switch basic.Kind() {
	case types.Int8:
		return math.MinInt8, true
	case types.Int16:
		return math.MinInt16, true
	case types.Int32:
		return math.MinInt32, true
	case types.Int64:
		return math.MinInt64, true
	case types.Int:
		return int64(math.MinInt), true
	default:
		return 0, false
	}
}
