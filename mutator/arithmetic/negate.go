package arithmetic

import (
	"go/ast"
	"go/token"
	"go/types"
	"math"
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

func parseIntLitVal(expr ast.Expr) (uint64, bool) {
	lit, ok := unwrapParen(expr).(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return 0, false
	}
	cleaned := strings.ReplaceAll(lit.Value, "_", "")
	val, err := strconv.ParseUint(cleaned, 0, 64)
	if err != nil {
		return 0, false
	}
	return val, true
}

func maxIntForKind(kind types.BasicKind) (uint64, bool) {
	switch kind {
	case types.Int8:
		return math.MaxInt8, true
	case types.Int16:
		return math.MaxInt16, true
	case types.Int32:
		return math.MaxInt32, true
	case types.Int64:
		return math.MaxInt64, true
	case types.Int:
		return math.MaxInt, true
	default:
		return 0, false
	}
}

func isTypedOverflow(info *types.Info, n *ast.UnaryExpr, val uint64) (bool, bool) {
	if info == nil {
		return false, false
	}
	tv, ok := info.Types[n]
	if !ok || tv.Type == nil {
		return false, false
	}
	basic, ok := tv.Type.Underlying().(*types.Basic)
	if !ok || basic.Info()&types.IsUntyped != 0 {
		return false, false
	}
	maxVal, ok := maxIntForKind(basic.Kind())
	if !ok {
		return false, false
	}
	return val > maxVal, true
}

func isMinIntMagnitude(val uint64) bool {
	switch val {
	case minInt8Magnitude, minInt16Magnitude, minInt32Magnitude:
		return true
	default:
		return val >= minInt64Magnitude
	}
}

// isSignedMinBoundary checks if a unary minus expression represents a signed integer
// minimum boundary constant (e.g. -128, -32768, -2147483648, -9223372036854775808).
// Mutating -MinInt to +MinInt produces integer constant overflow (+MinInt > MaxInt).
func isSignedMinBoundary(info *types.Info, n *ast.UnaryExpr) bool {
	val, ok := parseIntLitVal(n.X)
	if !ok {
		return false
	}

	if overflows, isTyped := isTypedOverflow(info, n, val); isTyped {
		return overflows
	}

	return isMinIntMagnitude(val)
}

// MutatorArithmeticNegate inverts unary negation: -x becomes +x (effectively x).
// Mirrors gremlins' INVERT_NEGATIVES operator.
func MutatorArithmeticNegate(_ *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.UnaryExpr)
	if !ok || n.Op != token.SUB {
		return nil
	}

	if isSignedMinBoundary(info, n) {
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
