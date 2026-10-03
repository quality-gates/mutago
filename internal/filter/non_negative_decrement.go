package filter

import (
	"go/ast"
	"go/constant"
	"go/token"
)

const numbersDecrementer = "numbers/decrementer"

// NonNegativeDecrementFilter skips decrementing zero when it is used in an
// expression context that rejects negative constants.
type NonNegativeDecrementFilter struct {
	ignoredNodes map[token.Pos]struct{}
}

// NewNonNegativeDecrementFilter creates an initialized filter.
func NewNonNegativeDecrementFilter() *NonNegativeDecrementFilter {
	return &NonNegativeDecrementFilter{ignoredNodes: make(map[token.Pos]struct{})}
}

// Collect finds zero literals used as indexes, bounds, lengths, or shift counts.
func (f *NonNegativeDecrementFilter) Collect(file *ast.File, _ *token.FileSet, _ string) {
	ast.Inspect(file, func(node ast.Node) bool {
		f.collectNonNegativeContext(node)
		return true
	})
}

func (f *NonNegativeDecrementFilter) collectNonNegativeContext(node ast.Node) {
	switch n := node.(type) {
	case *ast.IndexExpr:
		f.collectZeroLiteral(n.Index)
	case *ast.SliceExpr:
		f.collectZeroLiteral(n.Low)
		f.collectZeroLiteral(n.High)
		f.collectZeroLiteral(n.Max)
	case *ast.ArrayType:
		f.collectZeroLiteral(n.Len)
	case *ast.BinaryExpr:
		f.collectShiftCount(n)
	case *ast.AssignStmt:
		f.collectShiftAssignment(n)
	}
}

func (f *NonNegativeDecrementFilter) collectShiftCount(expr *ast.BinaryExpr) {
	if expr.Op == token.SHL || expr.Op == token.SHR {
		f.collectZeroLiteral(expr.Y)
	}
}

func (f *NonNegativeDecrementFilter) collectShiftAssignment(stmt *ast.AssignStmt) {
	if stmt.Tok != token.SHL_ASSIGN && stmt.Tok != token.SHR_ASSIGN {
		return
	}
	for _, rhs := range stmt.Rhs {
		f.collectZeroLiteral(rhs)
	}
}

// ShouldSkip excludes the collected literals only from numbers/decrementer.
func (f *NonNegativeDecrementFilter) ShouldSkip(node ast.Node, mutatorName string) bool {
	if node == nil || mutatorName != numbersDecrementer {
		return false
	}
	_, exists := f.ignoredNodes[node.Pos()]
	return exists
}

func (f *NonNegativeDecrementFilter) collectZeroLiteral(expr ast.Expr) {
	for {
		paren, ok := expr.(*ast.ParenExpr)
		if !ok {
			break
		}
		expr = paren.X
	}

	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return
	}
	value := constant.MakeFromLiteral(lit.Value, token.INT, 0)
	if value.Kind() == constant.Int && constant.Sign(value) == 0 {
		f.ignoredNodes[lit.Pos()] = struct{}{}
	}
}
