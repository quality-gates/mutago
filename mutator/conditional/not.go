package conditional

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("conditional/not", MutatorConditionalNot)
}

// MutatorConditionalNot removes the logical-NOT operator from a negated
// boolean expression: !x becomes x. This tests whether the negation is
// actually required — a test that only exercises one branch will miss this
// mutation.
//
// Works on if/for conditions and the operands of && / || expressions.
func MutatorConditionalNot(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
	switch n := node.(type) {
	case *ast.IfStmt:
		return notMutations(&n.Cond)
	case *ast.ForStmt:
		if n.Cond != nil {
			return notMutations(&n.Cond)
		}
	case *ast.BinaryExpr:
		if n.Op == token.LAND || n.Op == token.LOR {
			var ms []mutator.Mutation
			ms = append(ms, notMutations(&n.X)...)
			ms = append(ms, notMutations(&n.Y)...)
			return ms
		}
	}
	return nil
}

func notMutations(exprPtr *ast.Expr) []mutator.Mutation {
	unary, ok := (*exprPtr).(*ast.UnaryExpr)
	if !ok || unary.Op != token.NOT {
		return nil
	}
	original := *exprPtr
	inner := unary.X
	return []mutator.Mutation{
		{
			Change: func() { *exprPtr = inner },
			Reset:  func() { *exprPtr = original },
		},
	}
}
