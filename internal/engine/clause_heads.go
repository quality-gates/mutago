package engine

import (
	"go/ast"
	"go/token"
)

// clauseHead is the span of one case or communication clause, from its case or
// default keyword to its colon, and the switch, type switch or select that
// holds it.
type clauseHead struct {
	from, colon token.Pos
	stmt        token.Pos
}

// clauseHeads lists the clause heads of a file, read before any mutation.
// Mutators change the tree before the coverage anchor is looked up: they remove
// select clauses and put position-less operands into case conditions (#326).
type clauseHeads []clauseHead

// collectClauseHeads reads every clause head of root's switch, type switch and
// select statements. The walk is pre-order, so an enclosing clause head comes
// before any head nested inside it.
func collectClauseHeads(root ast.Node) clauseHeads {
	var heads clauseHeads
	ast.Inspect(root, func(n ast.Node) bool {
		for _, c := range clauseList(n) {
			heads = append(heads, clauseHead{from: c.Pos(), colon: clauseColon(c), stmt: n.Pos()})
		}
		return true
	})
	return heads
}

func clauseList(n ast.Node) []ast.Stmt {
	switch s := n.(type) {
	case *ast.SwitchStmt:
		return s.Body.List
	case *ast.TypeSwitchStmt:
		return s.Body.List
	case *ast.SelectStmt:
		return s.Body.List
	}
	return nil
}

func clauseColon(c ast.Stmt) token.Pos {
	if cc, ok := c.(*ast.CaseClause); ok {
		return cc.Colon
	}
	return c.(*ast.CommClause).Colon
}

// anchor returns the switch, type switch or select of the innermost clause head
// that holds pos, or token.NoPos. Go coverage profiles record execution for
// that statement, but not a separate range for each clause head.
func (h clauseHeads) anchor(pos token.Pos) token.Pos {
	anchor := token.NoPos
	for _, head := range h {
		if pos >= head.from && pos <= head.colon {
			anchor = head.stmt
		}
	}
	return anchor
}
