package numbers

import (
	"go/ast"
	"go/token"
	"go/types"
	"math"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
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
	if val > 0 {
		return false
	}
	if basic, ok := tv.Type.Underlying().(*types.Basic); ok && basic.Info()&types.IsUnsigned != 0 {
		return true
	}
	return inNonNegativeContext(info, expr)
}

// inNonNegativeContext reports whether expr is used where Go rejects negative
// constants: indexes, slice bounds, array lengths, make sizes, shift counts and
// array or slice literal keys.
func inNonNegativeContext(info *types.Info, expr ast.Expr) bool {
	path := enclosingPath(info, expr)
	child := ast.Node(expr)
	for i := 1; i < len(path); i++ {
		if _, ok := path[i].(*ast.ParenExpr); ok {
			child = path[i]
			continue
		}
		return requiresNonNegative(info, path[i], child, path[i+1:])
	}
	return false
}

func enclosingPath(info *types.Info, expr ast.Expr) []ast.Node {
	for node := range info.Scopes {
		file, ok := node.(*ast.File)
		if !ok || expr.Pos() < file.FileStart || expr.End() > file.FileEnd {
			continue
		}
		path, exact := astutil.PathEnclosingInterval(file, expr.Pos(), expr.End())
		if exact && len(path) > 0 && path[0] == expr {
			return path
		}
	}
	return nil
}

func requiresNonNegative(info *types.Info, parent, child ast.Node, ancestors []ast.Node) bool {
	return isIndexOrLength(info, parent, child) ||
		isShiftCount(parent, child) ||
		isSizeOrLiteralIndex(info, parent, child, ancestors)
}

// isIndexOrLength reports whether child is an index, a slice bound or an
// array length.
func isIndexOrLength(info *types.Info, parent, child ast.Node) bool {
	switch p := parent.(type) {
	case *ast.IndexExpr:
		return p.Index == child && !isMap(info, p.X)
	case *ast.SliceExpr:
		return p.Low == child || p.High == child || p.Max == child
	case *ast.ArrayType:
		return p.Len == child
	}
	return false
}

func isShiftCount(parent, child ast.Node) bool {
	switch p := parent.(type) {
	case *ast.BinaryExpr:
		return p.Y == child && isShift(p.Op)
	case *ast.AssignStmt:
		return isShift(p.Tok)
	}
	return false
}

// isSizeOrLiteralIndex reports whether child is a make size argument or the
// key of an array or slice literal element.
func isSizeOrLiteralIndex(info *types.Info, parent, child ast.Node, ancestors []ast.Node) bool {
	switch p := parent.(type) {
	case *ast.CallExpr:
		return isBuiltinMake(info, p.Fun) && len(p.Args) > 0 && p.Args[0] != child
	case *ast.KeyValueExpr:
		return p.Key == child && len(ancestors) > 0 && isIndexedLiteral(info, ancestors[0])
	}
	return false
}

func isShift(op token.Token) bool {
	switch op {
	case token.SHL, token.SHR, token.SHL_ASSIGN, token.SHR_ASSIGN:
		return true
	}
	return false
}

func isMap(info *types.Info, expr ast.Expr) bool {
	tv, ok := info.Types[expr]
	if !ok {
		return false
	}
	_, ok = tv.Type.Underlying().(*types.Map)
	return ok
}

func isBuiltinMake(info *types.Info, fun ast.Expr) bool {
	ident, ok := unwrapParen(fun).(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := info.Uses[ident].(*types.Builtin)
	return ok && builtin.Name() == "make"
}

func isIndexedLiteral(info *types.Info, node ast.Node) bool {
	lit, ok := node.(*ast.CompositeLit)
	if !ok {
		return false
	}
	tv, ok := info.Types[lit]
	if !ok {
		return false
	}
	switch tv.Type.Underlying().(type) {
	case *types.Array, *types.Slice:
		return true
	}
	return false
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
	if skipIncrementForType(info, expr, val) {
		return true
	}
	return skipIncrementUnderUnaryMinus(info, expr, val)
}

func skipIncrementUnderUnaryMinus(info *types.Info, expr ast.Expr, val int64) bool {
	for e := range info.Types {
		unary, ok := e.(*ast.UnaryExpr)
		if !ok || unary.Op != token.SUB {
			continue
		}
		if unwrapParen(unary.X) == expr && skipIncrementForType(info, unary, val) {
			return true
		}
	}
	return false
}

func unwrapParen(expr ast.Expr) ast.Expr {
	for {
		paren, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = paren.X
	}
}

func skipIncrementForType(info *types.Info, expr ast.Expr, val int64) bool {
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
