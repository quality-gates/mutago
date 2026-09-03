package arithmetic

import (
	"go/ast"
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
