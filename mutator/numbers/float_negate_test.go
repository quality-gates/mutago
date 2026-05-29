package numbers

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
	"github.com/stretchr/testify/assert"
)

func TestMutatorFloatNegate(t *testing.T) {
	test.Mutator(
		t,
		MutatorFloatNegate,
		"../../testdata/numbers/float_negate.go",
		2,
	)
}

func TestMutatorFloatNegate_SkipsZero(t *testing.T) {
	node := &ast.BasicLit{Kind: token.FLOAT, Value: "0.0"}
	assert.Nil(t, MutatorFloatNegate(nil, nil, node))
}

func TestMutatorFloatNegate_SkipsIntLit(t *testing.T) {
	node := &ast.BasicLit{Kind: token.INT, Value: "42"}
	assert.Nil(t, MutatorFloatNegate(nil, nil, node))
}

func TestMutatorFloatNegate_MutatesNonZero(t *testing.T) {
	node := &ast.BasicLit{Kind: token.FLOAT, Value: "3.14"}
	mutations := MutatorFloatNegate(nil, nil, node)
	assert.Len(t, mutations, 1)
	mutations[0].Change()
	assert.Equal(t, "0.0", node.Value)
	mutations[0].Reset()
	assert.Equal(t, "3.14", node.Value)
}

func TestMutatorFloatNegate_Registered(t *testing.T) {
	_, err := mutator.New("numbers/float-negate")
	assert.Nil(t, err)
}
