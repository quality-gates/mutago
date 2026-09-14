package statement

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/quality-gates/mutago/v2/astutil"
	"github.com/quality-gates/mutago/v2/internal/annotation"
	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("statement/return", MutatorReturnValue)
}

// MutatorReturnValue replaces each non-zero return value with the zero value
// for its type (false, 0, "", nil, TypeName{}).
func MutatorReturnValue(pkg *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	if info == nil {
		return nil
	}

	l, setStmts := stmtListAndSetter(node)
	if l == nil {
		return nil
	}

	var mutations []mutator.Mutation
	for stmtIdx := range l {
		mutations = append(mutations, mutateReturnStmt(pkg, info, l, stmtIdx, setStmts)...)
	}
	return mutations
}

func stmtListAndSetter(node ast.Node) ([]ast.Stmt, func([]ast.Stmt)) {
	switch n := node.(type) {
	case *ast.BlockStmt:
		return n.List, func(stmts []ast.Stmt) { n.List = stmts }
	case *ast.CaseClause:
		return n.Body, func(stmts []ast.Stmt) { n.Body = stmts }
	case *ast.CommClause:
		return n.Body, func(stmts []ast.Stmt) { n.Body = stmts }
	default:
		return nil, nil
	}
}

func mutateReturnStmt(pkg *types.Package, info *types.Info, l []ast.Stmt, stmtIdx int, setStmts func([]ast.Stmt)) []mutator.Mutation {
	ret, ok := l[stmtIdx].(*ast.ReturnStmt)
	if !ok || len(ret.Results) == 0 {
		return nil
	}
	if annotation.HandleBlockStmt(ret, "statement/return") {
		return nil
	}

	var mutations []mutator.Mutation
	for resIdx := range ret.Results {
		if m, ok := mutateReturnResult(pkg, info, l, stmtIdx, ret, resIdx, setStmts); ok {
			mutations = append(mutations, m)
		}
	}
	return mutations
}

func mutateReturnResult(pkg *types.Package, info *types.Info, l []ast.Stmt, stmtIdx int, ret *ast.ReturnStmt, resIdx int, setStmts func([]ast.Stmt)) (mutator.Mutation, bool) {
	result := ret.Results[resIdx]
	t := info.TypeOf(result)
	if t == nil {
		return mutator.Mutation{}, false
	}

	zero := astutil.ZeroExprForTypeAt(t, pkg, info, result.Pos())
	if zero == nil || isAlreadyZero(result) || astutil.HasUnsafeImport(info, result) {
		return mutator.Mutation{}, false
	}

	unsafeVars := astutil.UnsafeLocalVars(info, result)
	if len(unsafeVars) == 0 {
		orig := result
		idx := resIdx
		return mutator.Mutation{
			Position: orig.Pos(),
			Change:   func() { ret.Results[idx] = zero },
			Reset:    func() { ret.Results[idx] = orig },
		}, true
	}

	noop := astutil.CreateNoopOfExpressions(unsafeVars, ret.Pos())
	newRet := cloneReturnWithZero(ret, resIdx, zero)
	mutatedList := make([]ast.Stmt, len(l)+1)
	copy(mutatedList[:stmtIdx], l[:stmtIdx])
	mutatedList[stmtIdx] = noop
	mutatedList[stmtIdx+1] = newRet
	copy(mutatedList[stmtIdx+2:], l[stmtIdx+1:])

	return mutator.Mutation{
		Position: result.Pos(),
		Change:   func() { setStmts(mutatedList) },
		Reset:    func() { setStmts(l) },
	}, true
}

func cloneReturnWithZero(ret *ast.ReturnStmt, zeroIdx int, zero ast.Expr) *ast.ReturnStmt {
	newRet := &ast.ReturnStmt{
		Return:  ret.Return,
		Results: make([]ast.Expr, len(ret.Results)),
	}
	copy(newRet.Results, ret.Results)
	newRet.Results[zeroIdx] = zero
	return newRet
}

// isAlreadyZero reports whether expr is already a zero-value literal,
// avoiding no-op mutations.
func isAlreadyZero(expr ast.Expr) bool {
	switch n := expr.(type) {
	case *ast.Ident:
		switch n.Name {
		case "nil", "false":
			return true
		}
	case *ast.BasicLit:
		switch n.Kind {
		case token.INT, token.FLOAT:
			switch n.Value {
			case "0", "0.0":
				return true
			}
		case token.STRING:
			switch n.Value {
			case `""`, "``":
				return true
			}
		}
	case *ast.CompositeLit:
		// Already a zero-value struct literal if it has no field initializers.
		return len(n.Elts) == 0
	}
	return false
}
