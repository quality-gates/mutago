package numbers

import (
	"go/ast"
	"go/types"
	"math"
	"strconv"
	"strings"
)

type intLiteralInfo struct {
	val    int64
	base   int
	prefix string
}

func parseIntLiteral(s string) (intLiteralInfo, bool) {
	cleaned := strings.ReplaceAll(s, "_", "")
	base := 10
	prefix := ""

	if len(cleaned) >= 2 && cleaned[0] == '0' {
		switch cleaned[1] {
		case 'x', 'X':
			base = 16
			prefix = s[:2]
		case 'b', 'B':
			base = 2
			prefix = s[:2]
		case 'o', 'O':
			base = 8
			prefix = s[:2]
		}
	}

	val, err := strconv.ParseInt(cleaned, 0, 64)
	if err != nil {
		return intLiteralInfo{}, false
	}

	return intLiteralInfo{val: val, base: base, prefix: prefix}, true
}

func formatIntLiteral(val int64, info intLiteralInfo) string {
	if val < 0 {
		if info.base == 10 {
			return "(" + strconv.FormatInt(val, 10) + ")"
		}
		magnitude := uint64(-(val + 1)) + 1
		return "(-" + info.prefix + strconv.FormatUint(magnitude, info.base) + ")"
	}

	return info.prefix + strconv.FormatInt(val, info.base)
}

func shouldSkipDecrement(info *types.Info, expr ast.Expr, val int64) bool {
	if info == nil {
		return false
	}
	tv, ok := info.Types[expr]
	if !ok {
		return false
	}
	basic, ok := tv.Type.Underlying().(*types.Basic)
	if !ok {
		return false
	}
	return basic.Info()&types.IsUnsigned != 0 && val <= 0
}

var maxIntBounds = map[types.BasicKind]int64{
	types.Int8:    math.MaxInt8,
	types.Int16:   math.MaxInt16,
	types.Int32:   math.MaxInt32,
	types.Int64:   math.MaxInt64,
	types.Int:     math.MaxInt,
	types.Uint8:   math.MaxUint8,
	types.Uint16:  math.MaxUint16,
	types.Uint32:  math.MaxUint32,
	types.Uint64:  math.MaxInt64,
	types.Uint:    math.MaxInt,
	types.Uintptr: math.MaxInt,
}

func shouldSkipIncrement(info *types.Info, expr ast.Expr, val int64) bool {
	if info == nil {
		return false
	}
	tv, ok := info.Types[expr]
	if !ok {
		return false
	}
	basic, ok := tv.Type.Underlying().(*types.Basic)
	if !ok {
		return false
	}

	maxBound, bounded := maxIntBounds[basic.Kind()]
	return bounded && val >= maxBound
}
