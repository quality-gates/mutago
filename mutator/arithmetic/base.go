package arithmetic

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("arithmetic/base", MutatorArithmeticBase)
}

var arithmeticMutations = map[token.Token]token.Token{
	token.ADD: token.SUB,
	token.SUB: token.ADD,
	token.MUL: token.QUO,
	token.QUO: token.MUL,
	token.REM: token.MUL,
}

// MutatorArithmeticBase implements a mutator to change base arithmetic.
func MutatorArithmeticBase(_ *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.BinaryExpr)
	if !ok {
		return nil
	}

	if n.Op == token.ADD && isStringExpr(info, n) {
		return nil
	}

	if n.Op == token.MUL && isZeroExpr(info, n.Y) {
		return nil
	}

	original := n.Op
	mutated, ok := arithmeticMutations[n.Op]
	if !ok {
		return nil
	}

	return []mutator.Mutation{
		{
			Position: n.OpPos,
			Change: func() {
				n.Op = mutated
			},
			Reset: func() {
				n.Op = original
			},
		},
	}
}

func isStringExpr(info *types.Info, n *ast.BinaryExpr) bool {
	if isStringLit(n.X) || isStringLit(n.Y) {
		return true
	}
	if info != nil {
		if t := info.TypeOf(n); t != nil {
			basic, ok := t.Underlying().(*types.Basic)
			return ok && basic.Info()&types.IsString != 0
		}
	}
	return false
}

func isStringLit(expr ast.Expr) bool {
	lit, ok := expr.(*ast.BasicLit)
	return ok && lit.Kind == token.STRING
}

func isZeroExpr(info *types.Info, expr ast.Expr) bool {
	expr = unwrapParen(expr)
	if info != nil {
		if tv, ok := info.Types[expr]; ok && isZeroValue(tv.Value) {
			return true
		}
	}
	switch e := expr.(type) {
	case *ast.UnaryExpr:
		if e.Op == token.ADD || e.Op == token.SUB {
			return isZeroExpr(info, e.X)
		}
	case *ast.BasicLit:
		return isZeroValue(constant.MakeFromLiteral(e.Value, e.Kind, 0))
	}
	return false
}

func isZeroValue(val constant.Value) bool {
	if val == nil {
		return false
	}
	switch val.Kind() {
	case constant.Int, constant.Float:
		return constant.Sign(val) == 0
	}
	return false
}
