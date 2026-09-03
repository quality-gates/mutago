package expression

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("expression/remove", MutatorRemoveTerm)
}

// MutatorRemoveTerm implements a mutator to remove expression terms.
func MutatorRemoveTerm(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.BinaryExpr)
	if !ok {
		return nil
	}
	if n.Op != token.LAND && n.Op != token.LOR {
		return nil
	}

	var r *ast.Ident

	switch n.Op {
	case token.LAND:
		r = ast.NewIdent("true")
	case token.LOR:
		r = ast.NewIdent("false")
	}

	x := n.X
	y := n.Y

	var mutations []mutator.Mutation
	if !isIdent(x, r.Name) {
		mutations = append(mutations, mutator.Mutation{
			Position: x.Pos(),
			Change: func() {
				n.X = r
			},
			Reset: func() {
				n.X = x
			},
		})
	}
	if !isIdent(y, r.Name) {
		mutations = append(mutations, mutator.Mutation{
			Position: y.Pos(),
			Change: func() {
				n.Y = r
			},
			Reset: func() {
				n.Y = y
			},
		})
	}

	return mutations
}

func isIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}
