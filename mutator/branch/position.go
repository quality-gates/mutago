package branch

import (
	"go/ast"
	"go/token"
)

func statementPosition(stmt ast.Stmt) token.Pos {
	if block, ok := stmt.(*ast.BlockStmt); ok {
		if len(block.List) > 0 {
			return statementPosition(block.List[0])
		}
		return block.Lbrace
	}
	return stmt.Pos()
}
