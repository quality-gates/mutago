package branch

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/quality-gates/mutago/v2/astutil"
	"github.com/quality-gates/mutago/v2/mutator"
)

func mutateBranchBody(pkg *types.Package, info *types.Info, node ast.Node, old []ast.Stmt, setBody func([]ast.Stmt), pos token.Pos) []mutator.Mutation {
	if len(old) == 0 {
		return nil
	}
	ret, skip := terminatingReturn(pkg, info, node, func() {
		setBody([]ast.Stmt{astutil.CreateNoopOfStatements(pkg, info, old)})
	}, func() {
		setBody(old)
	})
	if skip {
		return nil
	}
	return []mutator.Mutation{
		{
			Position: pos,
			Change: func() {
				setBody(noopWithReturn(pkg, info, old, ret, pos))
			},
			Reset: func() {
				setBody(old)
			},
		},
	}
}

func terminatingReturn(pkg *types.Package, info *types.Info, node ast.Node, empty, restore func()) (ast.Stmt, bool) {
	body, sig := astutil.EnclosingFunc(info, node.Pos())
	if body == nil || sig == nil || sig.Results().Len() == 0 {
		return nil, false
	}
	empty()
	still := astutil.IsTerminatingList(body.List)
	restore()
	if still {
		return nil, false
	}
	ret := astutil.ZeroReturnForSignature(pkg, sig)
	if ret == nil {
		return nil, true
	}
	return ret, false
}

func noopWithReturn(pkg *types.Package, info *types.Info, old []ast.Stmt, ret ast.Stmt, pos token.Pos) []ast.Stmt {
	noop := astutil.CreateNoopOfStatements(pkg, info, old)
	if ret == nil {
		return []ast.Stmt{noop}
	}
	anchorReturn(ret, pos)
	if _, empty := noop.(*ast.EmptyStmt); empty {
		return []ast.Stmt{ret}
	}
	return []ast.Stmt{noop, ret}
}

func anchorReturn(ret ast.Stmt, pos token.Pos) {
	rs, ok := ret.(*ast.ReturnStmt)
	if !ok || !pos.IsValid() {
		return
	}
	rs.Return = pos
}
