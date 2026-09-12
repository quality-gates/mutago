package arithmetic

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestMutatorArithmeticNegate_SkipsTypedMinInt(t *testing.T) {
	src := `package main
func f() {
	var a int64 = -9223372036854775808
	var b int8 = -128
	var c int16 = -32768
	var d int32 = -2147483648
	var e int64 = -0x8000000000000000
	var f int8 = -127
	var g int = -1
	_ = a
	_ = b
	_ = c
	_ = d
	_ = e
	_ = f
	_ = g
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	require.NoError(t, err)
	info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
	_, err = (&types.Config{}).Check("main", fset, []*ast.File{file}, info)
	require.NoError(t, err)

	skipped, mutated := 0, 0
	ast.Inspect(file, func(n ast.Node) bool {
		u, ok := n.(*ast.UnaryExpr)
		if !ok || u.Op != token.SUB {
			return true
		}
		muts := MutatorArithmeticNegate(nil, info, u)
		tv := info.Types[u]
		switch tv.Type.String() {
		case "int64", "int8", "int16", "int32":
			if tv.Value != nil && (tv.Value.ExactString() == "-9223372036854775808" ||
				tv.Value.ExactString() == "-128" ||
				tv.Value.ExactString() == "-32768" ||
				tv.Value.ExactString() == "-2147483648") {
				require.Empty(t, muts, "MinInt %s should skip", tv.Type)
				skipped++
				return true
			}
		}
		require.Len(t, muts, 1, "non-MinInt %s=%s should mutate", tv.Type, tv.Value)
		mutated++
		return true
	})
	require.Equal(t, 5, skipped)
	require.Equal(t, 2, mutated)
}
