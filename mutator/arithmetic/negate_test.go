package arithmetic

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
	"github.com/stretchr/testify/assert"
)

func TestMutatorArithmeticNegate(t *testing.T) {
	test.Mutator(
		t,
		MutatorArithmeticNegate,
		"../../testdata/arithmetic/negate.go",
		2,
	)
}

func TestMutatorArithmeticNegate_SkipsPlus(t *testing.T) {
	node := &ast.UnaryExpr{Op: token.ADD, X: ast.NewIdent("x")}
	assert.Nil(t, MutatorArithmeticNegate(nil, nil, node))
}

func TestMutatorArithmeticNegate_SkipsNonUnary(t *testing.T) {
	node := &ast.BasicLit{Kind: token.INT, Value: "5"}
	assert.Nil(t, MutatorArithmeticNegate(nil, nil, node))
}

func TestMutatorArithmeticNegate_MutatesMinus(t *testing.T) {
	node := &ast.UnaryExpr{Op: token.SUB, X: ast.NewIdent("x")}
	mutations := MutatorArithmeticNegate(nil, nil, node)
	assert.Len(t, mutations, 1)
	mutations[0].Change()
	assert.Equal(t, token.ADD, node.Op)
	mutations[0].Reset()
	assert.Equal(t, token.SUB, node.Op)
}

func TestMutatorArithmeticNegate_Registered(t *testing.T) {
	_, err := mutator.New("arithmetic/negate")
	assert.Nil(t, err)
}
