package expression

import (
	"go/ast"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
	"github.com/stretchr/testify/assert"
)

func TestMutatorContextNil(t *testing.T) {
	test.Mutator(
		t,
		MutatorContextNil,
		"../../testdata/expression/context_nil.go",
		2,
	)
}

func TestMutatorContextNil_SkipsNilInfo(t *testing.T) {
	call := &ast.CallExpr{Args: []ast.Expr{ast.NewIdent("ctx")}}
	assert.Nil(t, MutatorContextNil(nil, nil, call))
}

func TestMutatorContextNil_SkipsNonCallExpr(t *testing.T) {
	assert.Nil(t, MutatorContextNil(nil, nil, &ast.BasicLit{}))
}

func TestMutatorContextNil_Registered(t *testing.T) {
	_, err := mutator.New("expression/context-nil")
	assert.Nil(t, err)
}
