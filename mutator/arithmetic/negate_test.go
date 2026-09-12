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

func TestMutatorArithmeticNegate_SkipsMinIntBoundaries(t *testing.T) {
	minIntLiterals := []string{
		"128",
		"32768",
		"2147483648",
		"9223372036854775808",
		"0x80",
		"0x8000",
		"0x80000000",
		"0x8000000000000000",
		"0o200",
		"0b10000000",
		"9_223_372_036_854_775_808",
	}
	for _, lit := range minIntLiterals {
		node := &ast.UnaryExpr{
			Op: token.SUB,
			X:  &ast.BasicLit{Kind: token.INT, Value: lit},
		}
		assert.Nil(t, MutatorArithmeticNegate(nil, nil, node), "expected mutation to be skipped for -%s", lit)
	}

	// Also test parenthesized boundary constant -(128)
	parenNode := &ast.UnaryExpr{
		Op: token.SUB,
		X: &ast.ParenExpr{
			X: &ast.BasicLit{Kind: token.INT, Value: "128"},
		},
	}
	assert.Nil(t, MutatorArithmeticNegate(nil, nil, parenNode), "expected mutation to be skipped for -(128)")
}

func TestMutatorArithmeticNegate_AllowsNonBoundaryNumbers(t *testing.T) {
	nonBoundaries := []string{
		"1",
		"0",
		"10",
		"127",
		"32767",
		"2147483647",
		"9223372036854775807",
		"9999999999999999999999999999999999999999",
	}
	for _, lit := range nonBoundaries {
		node := &ast.UnaryExpr{
			Op: token.SUB,
			X:  &ast.BasicLit{Kind: token.INT, Value: lit},
		}
		mutations := MutatorArithmeticNegate(nil, nil, node)
		assert.Len(t, mutations, 1, "expected mutation for -%s", lit)
	}
}

func TestMutatorArithmeticNegate_TypedContext(t *testing.T) {
	src := `package main
var a int8 = -128
var b int16 = -32768
var c int32 = -2147483648
var d int64 = -9223372036854775808
const MinInt8 = -128
var e int16 = -128
var f = -1
var ok8 int8 = -127
var ok16 int16 = -32767
var ok32 int32 = -2147483647
var ok64 int64 = -9223372036854775807
var okInt int = -100
var okFloat float64 = -2.5
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	assert.NoError(t, err)

	conf := types.Config{}
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
	}
	_, err = conf.Check("main", fset, []*ast.File{file}, info)
	assert.NoError(t, err)

	expected := map[string]bool{
		"a":       false, // int8: +128 overflows
		"b":       false, // int16: +32768 overflows
		"c":       false, // int32: +2147483648 overflows
		"d":       false, // int64: +9223372036854775808 overflows
		"MinInt8": false, // untyped boundary constant: +128
		"e":       true,  // int16: +128 is representable in int16
		"f":       true,  // int: +1 is representable in int
		"ok8":     true,  // int8: +127 is representable in int8
		"ok16":    true,  // int16: +32767 is representable in int16
		"ok32":    true,  // int32: +2147483647 is representable in int32
		"ok64":    true,  // int64: +9223372036854775807 is representable in int64
		"okInt":   true,  // int: +100 is representable in int
		"okFloat": true,  // float64: -2.5 is not an integer, mutates
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.ValueSpec:
			for i, name := range decl.Names {
				shouldMutate, ok := expected[name.Name]
				if !ok {
					continue
				}
				val := decl.Values[i]
				muts := MutatorArithmeticNegate(nil, info, val)
				if shouldMutate {
					assert.Len(t, muts, 1, "expected mutation for %s", name.Name)
				} else {
					assert.Nil(t, muts, "expected no mutation for %s", name.Name)
				}
			}
		}
		return true
	})
}
