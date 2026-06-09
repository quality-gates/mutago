package astutil

import (
	"go/ast"
	"go/token"
	"go/types"
)

// CreateNoopOfStatement creates a syntactically safe noop statement out of a given statement.
func CreateNoopOfStatement(pkg *types.Package, info *types.Info, stmt ast.Stmt) ast.Stmt {
	return CreateNoopOfStatements(pkg, info, []ast.Stmt{stmt})
}

// CreateNoopOfStatements creates a syntactically safe noop statement out of a given statement.
//
// The replacement is anchored at the position of the first original line of code.
// Without a position the synthesized tokens (the `_` identifiers and the `=`)
// default to token.NoPos, which sorts before everything; go/printer then floats a
// leading comment into the middle of the assignment (e.g. `_, _ =\n// comment\ntotal, x`)
// and the diff's first hunk line points at the comment rather than the code.
// Anchoring at the original code position keeps a leading comment above the
// replacement and makes the diff report the correct original line.
func CreateNoopOfStatements(pkg *types.Package, info *types.Info, stmts []ast.Stmt) ast.Stmt {
	var pos token.Pos
	if len(stmts) > 0 {
		pos = anchorPos(stmts[0])
	}

	ids := identifiersInStatements(pkg, info, stmts)
	for _, id := range ids {
		anchorExpression(id, pos)
	}

	if len(ids) == 0 {
		return &ast.EmptyStmt{
			Semicolon: pos,
		}
	}

	lhs := make([]ast.Expr, len(ids))
	for i := range ids {
		lhs[i] = &ast.Ident{Name: "_", NamePos: pos}
	}

	return &ast.AssignStmt{
		Lhs:    lhs,
		Rhs:    ids,
		Tok:    token.ASSIGN,
		TokPos: pos,
	}
}

func anchorExpression(expr ast.Expr, pos token.Pos) {
	switch n := expr.(type) {
	case *ast.Ident:
		n.NamePos = pos
	case *ast.SelectorExpr:
		anchorExpression(n.X, pos)
		anchorExpression(n.Sel, pos)
	case *ast.CompositeLit:
		anchorExpression(n.Type, pos)
	}
}

// anchorPos returns the position of the first real line of code in stmt.
// For a block statement that is stmt.Pos() of its first inner statement (skipping
// the opening brace), so a noop replacing a whole branch body sits on the code line
// rather than at the brace. For any other statement it is just stmt.Pos().
func anchorPos(stmt ast.Stmt) token.Pos {
	if block, ok := stmt.(*ast.BlockStmt); ok {
		if len(block.List) > 0 {
			return anchorPos(block.List[0])
		}
		return token.NoPos
	}

	return stmt.Pos()
}
