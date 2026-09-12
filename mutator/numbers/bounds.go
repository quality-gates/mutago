package numbers

import (
	"go/ast"
	"go/types"
	"math"
)

// integerBasicOf returns the typed integer basic type of expr, if any.
// Untyped integer constants and non-integers yield nil.
func integerBasicOf(info *types.Info, expr ast.Expr) *types.Basic {
	if info == nil {
		return nil
	}
	t := info.TypeOf(expr)
	if t == nil {
		return nil
	}
	basic, ok := t.Underlying().(*types.Basic)
	if !ok {
		return nil
	}
	if basic.Info()&types.IsInteger == 0 || basic.Info()&types.IsUntyped != 0 {
		return nil
	}
	return basic
}

// maxIntOf returns the maximum representable value of basic as an int64.
// ok is false when the maximum does not fit in int64 (e.g. uint64 on 64-bit).
func maxIntOf(basic *types.Basic) (int64, bool) {
	switch basic.Kind() {
	case types.Int8:
		return math.MaxInt8, true
	case types.Int16:
		return math.MaxInt16, true
	case types.Int32:
		return math.MaxInt32, true
	case types.Int64:
		return math.MaxInt64, true
	case types.Int:
		return int64(math.MaxInt), true
	case types.Uint8:
		return math.MaxUint8, true
	case types.Uint16:
		return math.MaxUint16, true
	case types.Uint32:
		return math.MaxUint32, true
	case types.Uint64:
		return 0, false
	case types.Uint, types.Uintptr:
		max := uint64(^uint(0))
		if max > math.MaxInt64 {
			return 0, false
		}
		return int64(max), true
	default:
		return 0, false
	}
}
