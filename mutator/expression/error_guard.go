package expression

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("expression/error-guard", MutatorErrorGuard)
}

// MutatorErrorGuard replaces error-check conditions in if-statements:
//
//	if err != nil  →  if false
//	if err == nil  →  if true
//
// Inspired by gomu's error-handling mutations.
func MutatorErrorGuard(_ *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	ifStmt, ok := node.(*ast.IfStmt)
	if !ok || info == nil {
		return nil
	}

	bin, ok := ifStmt.Cond.(*ast.BinaryExpr)
	if !ok {
		return nil
	}
	if bin.Op != token.NEQ && bin.Op != token.EQL {
		return nil
	}
	if !isErrorNilComparison(info, bin) {
		return nil
	}

	replacement := boolIdentForOp(bin.Op)
	original := ifStmt.Cond
	return []mutator.Mutation{
		{
			Change: func() { ifStmt.Cond = replacement },
			Reset:  func() { ifStmt.Cond = original },
		},
	}
}

// isErrorNilComparison reports whether bin compares an error-typed expression
// against nil in either order, e.g. `err != nil` or `nil == err`. Requiring one
// side to be nil avoids triggering on `if err1 != err2` (two-error comparisons).
func isErrorNilComparison(info *types.Info, bin *ast.BinaryExpr) bool {
	xIsNil := isNilIdent(bin.X)
	yIsNil := isNilIdent(bin.Y)
	if !xIsNil && !yIsNil {
		return false
	}
	errExpr := bin.Y
	if yIsNil {
		errExpr = bin.X
	}
	return isErrorExpr(info, errExpr)
}

// boolIdentForOp returns the constant the guard collapses to: `!=` checks become
// false, `==` checks become true.
func boolIdentForOp(op token.Token) *ast.Ident {
	if op == token.NEQ {
		return ast.NewIdent("false")
	}
	return ast.NewIdent("true")
}

func isNilIdent(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "nil"
}

// isErrorExpr reports whether expr is of type error (the predeclared interface).
func isErrorExpr(info *types.Info, expr ast.Expr) bool {
	t := info.TypeOf(expr)
	if t == nil {
		return false
	}
	return t.String() == "error"
}
