package expression

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("expression/string-literal", MutatorStringLiteral)
}

// MutatorStringLiteral zeros non-empty string literals in equality and
// inequality comparisons (== and !=). This tests whether the exact string
// value matters — e.g. if err.Error() == "not found" where "" would pass
// when no error occurs but fail when the specific message is expected.
//
// Scoped to comparisons only to avoid high-noise mutations in log messages,
// metric names, and other strings whose values are not asserted by tests.
func MutatorStringLiteral(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.BinaryExpr)
	if !ok {
		return nil
	}
	if n.Op != token.EQL && n.Op != token.NEQ {
		return nil
	}
	var ms []mutator.Mutation
	ms = append(ms, stringLiteralMutation(&n.X)...)
	ms = append(ms, stringLiteralMutation(&n.Y)...)
	return ms
}

func stringLiteralMutation(exprPtr *ast.Expr) []mutator.Mutation {
	lit, ok := (*exprPtr).(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return nil
	}
	switch lit.Value {
	case `""`:
		return nil
	}
	original := *exprPtr
	mutated := &ast.BasicLit{Kind: token.STRING, Value: `""`}
	return []mutator.Mutation{
		{
			Change: func() { *exprPtr = mutated },
			Reset:  func() { *exprPtr = original },
		},
	}
}
