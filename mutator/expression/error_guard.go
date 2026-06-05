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
func isErrorComparison(bin *ast.BinaryExpr, info *types.Info) bool {
	if bin.Op != token.NEQ && bin.Op != token.EQL {
		return false
	}
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
	if !isErrorComparison(bin, info) {
		return nil
	}

	replacement := ast.NewIdent("true")
	if bin.Op == token.NEQ {
		replacement = ast.NewIdent("false")
	}

	original := ifStmt.Cond
	return []mutator.Mutation{
		{
			Change: func() { ifStmt.Cond = replacement },
			Reset:  func() { ifStmt.Cond = original },
		},
	}
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
