package conditional

import (
	"go/ast"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("conditional/bool-literal", MutatorBoolLiteral)
}

// MutatorBoolLiteral swaps true↔false in assignment right-hand sides and
// function call arguments. It tests whether the hardcoded boolean value
// matters — e.g. a config flag defaulting to true that no test ever flips.
//
// Return statements are intentionally excluded: statement/return already
// handles bool return values by zeroing them. If-conditions are excluded
// because conditional/not covers that case more precisely.
func MutatorBoolLiteral(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
	switch n := node.(type) {
	case *ast.AssignStmt:
		var ms []mutator.Mutation
		for i := range n.Rhs {
			ms = append(ms, boolLiteralMutation(&n.Rhs[i])...)
		}
		return ms
	case *ast.CallExpr:
		var ms []mutator.Mutation
		for i := range n.Args {
			ms = append(ms, boolLiteralMutation(&n.Args[i])...)
		}
		return ms
	}
	return nil
}

func boolLiteralMutation(exprPtr *ast.Expr) []mutator.Mutation {
	ident, ok := (*exprPtr).(*ast.Ident)
	if !ok {
		return nil
	}
	var replacement string
	switch ident.Name {
	case "true":
		replacement = "false"
	case "false":
		replacement = "true"
	default:
		return nil
	}
	original := *exprPtr
	mutated := ast.NewIdent(replacement)
	return []mutator.Mutation{
		{
			Change: func() { *exprPtr = mutated },
			Reset:  func() { *exprPtr = original },
		},
	}
}
