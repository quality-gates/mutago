package statement

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("statement/remove-self-assign", MutatorRemoveSelfAssign)
}

// MutatorRemoveSelfAssign removes self-assignment statements (a = a).
// Mirrors gremlins' REMOVE_SELF_ASSIGNMENTS operator.
func MutatorRemoveSelfAssign(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
	var l []ast.Stmt
	switch n := node.(type) {
	case *ast.BlockStmt:
		l = n.List
	case *ast.CaseClause:
		l = n.Body
	}
	if l == nil {
		return nil
	}

	var mutations []mutator.Mutation
	for i, stmt := range l {
		if !isSelfAssign(stmt) {
			continue
		}

		li := i
		old := l[li]
		mutations = append(mutations, mutator.Mutation{
			Change: func() { l[li] = &ast.EmptyStmt{Semicolon: token.NoPos} },
			Reset:  func() { l[li] = old },
		})
	}
	return mutations
}

// isSelfAssign reports whether stmt is a plain assignment whose left- and
// right-hand sides are pairwise identical (e.g. `a = a` or `a, b = a, b`).
func isSelfAssign(stmt ast.Stmt) bool {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN || len(assign.Lhs) != len(assign.Rhs) {
		return false
	}
	for j := range assign.Lhs {
		if !stmtExprEqual(assign.Lhs[j], assign.Rhs[j]) {
			return false
		}
	}
	return true
}

// stmtExprEqual reports whether two expressions are syntactically identical.
func stmtExprEqual(a, b ast.Expr) bool {
	switch x := a.(type) {
	case *ast.Ident:
		return identExprEqual(x, b)
	case *ast.BasicLit:
		return basicLitEqual(x, b)
	case *ast.BinaryExpr:
		return binaryExprEqual(x, b)
	case *ast.UnaryExpr:
		return unaryExprEqual(x, b)
	case *ast.SelectorExpr:
		return selectorExprEqual(x, b)
	case *ast.IndexExpr:
		return indexExprEqual(x, b)
	case *ast.StarExpr:
		return starExprEqual(x, b)
	}
	return false
}

func identExprEqual(x *ast.Ident, b ast.Expr) bool {
	y, ok := b.(*ast.Ident)
	return ok && x.Name == y.Name
}

func basicLitEqual(x *ast.BasicLit, b ast.Expr) bool {
	y, ok := b.(*ast.BasicLit)
	return ok && x.Kind == y.Kind && x.Value == y.Value
}

func starExprEqual(x *ast.StarExpr, b ast.Expr) bool {
	y, ok := b.(*ast.StarExpr)
	return ok && stmtExprEqual(x.X, y.X)
}

func binaryExprEqual(x *ast.BinaryExpr, b ast.Expr) bool {
	y, ok := b.(*ast.BinaryExpr)
	return ok && x.Op == y.Op && stmtExprEqual(x.X, y.X) && stmtExprEqual(x.Y, y.Y)
}

func unaryExprEqual(x *ast.UnaryExpr, b ast.Expr) bool {
	y, ok := b.(*ast.UnaryExpr)
	return ok && x.Op == y.Op && stmtExprEqual(x.X, y.X)
}

func selectorExprEqual(x *ast.SelectorExpr, b ast.Expr) bool {
	y, ok := b.(*ast.SelectorExpr)
	return ok && stmtExprEqual(x.X, y.X) && x.Sel.Name == y.Sel.Name
}

func indexExprEqual(x *ast.IndexExpr, b ast.Expr) bool {
	y, ok := b.(*ast.IndexExpr)
	return ok && stmtExprEqual(x.X, y.X) && stmtExprEqual(x.Index, y.Index)
}
