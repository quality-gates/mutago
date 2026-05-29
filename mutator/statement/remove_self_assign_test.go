package statement

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
	"github.com/stretchr/testify/assert"
)

func TestMutatorRemoveSelfAssign(t *testing.T) {
	test.Mutator(
		t,
		MutatorRemoveSelfAssign,
		"../../testdata/statement/remove_self_assign.go",
		2,
	)
}

func TestMutatorRemoveSelfAssign_SkipsNonBlock(t *testing.T) {
	assert.Nil(t, MutatorRemoveSelfAssign(nil, nil, &ast.BasicLit{}))
}

func TestMutatorRemoveSelfAssign_SkipsShortAssign(t *testing.T) {
	// := is not a self-assignment
	stmt := &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent("x")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{ast.NewIdent("x")},
	}
	block := &ast.BlockStmt{List: []ast.Stmt{stmt}}
	assert.Nil(t, MutatorRemoveSelfAssign(nil, nil, block))
}

func TestMutatorRemoveSelfAssign_SkipsNonSelf(t *testing.T) {
	// x = y is not a self-assignment
	stmt := &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent("x")},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{ast.NewIdent("y")},
	}
	block := &ast.BlockStmt{List: []ast.Stmt{stmt}}
	assert.Nil(t, MutatorRemoveSelfAssign(nil, nil, block))
}

func TestMutatorRemoveSelfAssign_MutatesSelf(t *testing.T) {
	// x = x should produce one mutation
	ident := ast.NewIdent("x")
	stmt := &ast.AssignStmt{
		Lhs: []ast.Expr{ident},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{ast.NewIdent("x")},
	}
	block := &ast.BlockStmt{List: []ast.Stmt{stmt}}
	mutations := MutatorRemoveSelfAssign(nil, nil, block)
	assert.Len(t, mutations, 1)
	mutations[0].Change()
	_, isEmpty := block.List[0].(*ast.EmptyStmt)
	assert.True(t, isEmpty)
	mutations[0].Reset()
	assert.Equal(t, stmt, block.List[0])
}

func TestMutatorRemoveSelfAssign_NonSelfBeforeSelf(t *testing.T) {
	// a = b (non-self) followed by x = x (self): count must be 1, not 0
	nonSelf := &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent("a")},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{ast.NewIdent("b")},
	}
	self := &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent("x")},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{ast.NewIdent("x")},
	}
	block := &ast.BlockStmt{List: []ast.Stmt{nonSelf, self}}
	assert.Len(t, MutatorRemoveSelfAssign(nil, nil, block), 1)
}

func TestMutatorRemoveSelfAssign_SkipsLengthMismatch(t *testing.T) {
	// a, b = a — Lhs len 2 != Rhs len 1, must not produce a mutation
	stmt := &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent("a"), ast.NewIdent("b")},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{ast.NewIdent("a")},
	}
	block := &ast.BlockStmt{List: []ast.Stmt{stmt}}
	assert.Nil(t, MutatorRemoveSelfAssign(nil, nil, block))
}

func TestMutatorRemoveSelfAssign_Registered(t *testing.T) {
	_, err := mutator.New("statement/remove-self-assign")
	assert.Nil(t, err)
}

func TestStmtExprEqual_BasicLit(t *testing.T) {
	same := &ast.BasicLit{Kind: token.INT, Value: "1"}
	diff := &ast.BasicLit{Kind: token.INT, Value: "2"}
	assert.True(t, stmtExprEqual(same, same))
	assert.False(t, stmtExprEqual(same, diff))
	assert.False(t, stmtExprEqual(same, ast.NewIdent("x")))
}

func TestStmtExprEqual_BinaryExpr(t *testing.T) {
	a, b, c := ast.NewIdent("a"), ast.NewIdent("b"), ast.NewIdent("c")
	addAB := &ast.BinaryExpr{Op: token.ADD, X: a, Y: b}
	subAB := &ast.BinaryExpr{Op: token.SUB, X: a, Y: b}
	addCB := &ast.BinaryExpr{Op: token.ADD, X: c, Y: b}
	addAC := &ast.BinaryExpr{Op: token.ADD, X: a, Y: c}
	assert.True(t, stmtExprEqual(addAB, addAB))
	assert.False(t, stmtExprEqual(addAB, subAB))
	assert.False(t, stmtExprEqual(addAB, addCB))
	assert.False(t, stmtExprEqual(addAB, addAC))
	assert.False(t, stmtExprEqual(addAB, ast.NewIdent("x")))
}

func TestStmtExprEqual_UnaryExpr(t *testing.T) {
	neg := &ast.UnaryExpr{Op: token.SUB, X: ast.NewIdent("x")}
	pos := &ast.UnaryExpr{Op: token.ADD, X: ast.NewIdent("x")}
	diffX := &ast.UnaryExpr{Op: token.SUB, X: ast.NewIdent("y")}
	assert.True(t, stmtExprEqual(neg, neg))
	assert.False(t, stmtExprEqual(neg, pos))
	assert.False(t, stmtExprEqual(neg, diffX))
	assert.False(t, stmtExprEqual(neg, ast.NewIdent("x")))
}

func TestStmtExprEqual_SelectorExpr(t *testing.T) {
	s, t2 := ast.NewIdent("s"), ast.NewIdent("t")
	field, other := ast.NewIdent("Field"), ast.NewIdent("Other")
	s1 := &ast.SelectorExpr{X: s, Sel: field}
	s2 := &ast.SelectorExpr{X: s, Sel: other}
	s3 := &ast.SelectorExpr{X: t2, Sel: field}
	assert.True(t, stmtExprEqual(s1, s1))
	assert.False(t, stmtExprEqual(s1, s2))
	assert.False(t, stmtExprEqual(s1, s3))
	assert.False(t, stmtExprEqual(s1, ast.NewIdent("x")))
}

func TestStmtExprEqual_IndexExpr(t *testing.T) {
	a, b := ast.NewIdent("a"), ast.NewIdent("b")
	i, j := ast.NewIdent("i"), ast.NewIdent("j")
	i1 := &ast.IndexExpr{X: a, Index: i}
	i2 := &ast.IndexExpr{X: a, Index: j}
	i3 := &ast.IndexExpr{X: b, Index: i}
	assert.True(t, stmtExprEqual(i1, i1))
	assert.False(t, stmtExprEqual(i1, i2))
	assert.False(t, stmtExprEqual(i1, i3))
	assert.False(t, stmtExprEqual(i1, ast.NewIdent("x")))
}

func TestStmtExprEqual_StarExpr(t *testing.T) {
	p1 := &ast.StarExpr{X: ast.NewIdent("p")}
	p2 := &ast.StarExpr{X: ast.NewIdent("q")}
	assert.True(t, stmtExprEqual(p1, p1))
	assert.False(t, stmtExprEqual(p1, p2))
	assert.False(t, stmtExprEqual(p1, ast.NewIdent("x")))
}

func TestStmtExprEqual_Unknown(t *testing.T) {
	call := &ast.CallExpr{}
	assert.False(t, stmtExprEqual(call, call))
}
