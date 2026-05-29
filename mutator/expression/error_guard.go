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
	// Require one side to be nil and the other to be of type error.
	// This avoids triggering on `if err1 != err2` (two-error comparisons).
	xIsNil := isNilIdent(bin.X)
	yIsNil := isNilIdent(bin.Y)
	if !xIsNil && !yIsNil {
		return nil
	}
	errExpr := bin.Y
	if yIsNil {
		errExpr = bin.X
	}
	if !isErrorExpr(info, errExpr) {
		return nil
	}

	var replacement ast.Expr
	if bin.Op == token.NEQ {
		replacement = ast.NewIdent("false")
	} else {
		replacement = ast.NewIdent("true")
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
