package expression

import (
	"go/ast"
	"go/token"
	"go/types"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
	"github.com/stretchr/testify/assert"
)

func TestMutatorErrorGuard(t *testing.T) {
	test.Mutator(
		t,
		MutatorErrorGuard,
		"../../testdata/expression/error_guard.go",
		2,
	)
}

func TestMutatorErrorGuard_SkipsNonIfStmt(t *testing.T) {
	assert.Nil(t, MutatorErrorGuard(nil, nil, &ast.BasicLit{}))
}

func TestMutatorErrorGuard_SkipsNilInfo(t *testing.T) {
	ifStmt := &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			Op: token.NEQ,
			X:  ast.NewIdent("err"),
			Y:  ast.NewIdent("nil"),
		},
	}
	assert.Nil(t, MutatorErrorGuard(nil, nil, ifStmt))
}

func TestMutatorErrorGuard_SkipsNonBinaryCond(t *testing.T) {
	ifStmt := &ast.IfStmt{Cond: ast.NewIdent("ok")}
	assert.Nil(t, MutatorErrorGuard(nil, nil, ifStmt))
}

func TestMutatorErrorGuard_SkipsBothNonNil(t *testing.T) {
	// if err1 != err2 — neither side is the nil identifier → should return nil
	ifStmt := &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			Op: token.NEQ,
			X:  ast.NewIdent("err1"),
			Y:  ast.NewIdent("err2"),
		},
	}
	assert.Nil(t, MutatorErrorGuard(nil, &types.Info{}, ifStmt))
}

func TestMutatorErrorGuard_SkipsNonIdentNilSide(t *testing.T) {
	// if someCall() != nil — non-Ident on X, covers isNilIdent(!ok) and isErrorExpr(t==nil)
	ifStmt := &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			Op: token.NEQ,
			X:  &ast.CallExpr{Fun: ast.NewIdent("f")},
			Y:  ast.NewIdent("nil"),
		},
	}
	assert.Nil(t, MutatorErrorGuard(nil, &types.Info{}, ifStmt))
}

func TestMutatorErrorGuard_Registered(t *testing.T) {
	_, err := mutator.New("expression/error-guard")
	assert.Nil(t, err)
}
