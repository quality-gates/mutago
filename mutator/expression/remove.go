package expression

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/quality-gates/mutago/v2/astutil"
	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("expression/remove", MutatorRemoveTerm)
}

func MutatorRemoveTerm(_ *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.BinaryExpr)
	if !ok {
		return nil
	}
	r := replacementForOp(n.Op)
	if r == nil {
		return nil
	}

	var mutations []mutator.Mutation
	if m, ok := tryMutateOperand(info, &n.X, r); ok {
		mutations = append(mutations, m)
	}
	if m, ok := tryMutateOperand(info, &n.Y, r); ok {
		mutations = append(mutations, m)
	}
	return mutations
}

func replacementForOp(op token.Token) *ast.Ident {
	switch op {
	case token.LAND:
		return ast.NewIdent("true")
	case token.LOR:
		return ast.NewIdent("false")
	default:
		return nil
	}
}

func tryMutateOperand(info *types.Info, target *ast.Expr, replacement *ast.Ident) (mutator.Mutation, bool) {
	orig := *target
	if isIdent(orig, replacement.Name) || !astutil.IsSafeToRemove(info, orig) {
		return mutator.Mutation{}, false
	}
	return mutator.Mutation{
		Position: orig.Pos(),
		Change: func() {
			*target = replacement
		},
		Reset: func() {
			*target = orig
		},
	}, true
}

func isIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}
