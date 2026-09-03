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

func TestMutatorArithmeticBase(t *testing.T) {
	test.Mutator(
		t,
		MutatorArithmeticBase,
		"../../testdata/arithmetic/base.go",
		5,
	)
}

func TestMutatorArithmeticBaseRegistered(t *testing.T) {
	if _, err := mutator.New("arithmetic/base"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorArithmeticBase_SkipsStrings(t *testing.T) {
	src := `package main
func concat(a, b string) string {
	return a + b
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{}
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
	}
	_, err = conf.Check("main", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}

	var count int
	ast.Inspect(file, func(n ast.Node) bool {
		muts := MutatorArithmeticBase(nil, info, n)
		count += len(muts)
		return true
	})
	if count != 0 {
		t.Fatalf("expected 0 mutations on string +, got %d", count)
	}
}

func TestMutatorArithmeticBase_StringLiteralWithoutInfo(t *testing.T) {
	strBin := &ast.BinaryExpr{
		Op: token.ADD,
		X:  &ast.BasicLit{Kind: token.STRING, Value: `"hello"`},
		Y:  &ast.BasicLit{Kind: token.STRING, Value: `"world"`},
	}
	assert.Empty(t, MutatorArithmeticBase(nil, nil, strBin))

	intBin := &ast.BinaryExpr{
		Op: token.ADD,
		X:  &ast.BasicLit{Kind: token.INT, Value: "1"},
		Y:  &ast.BasicLit{Kind: token.INT, Value: "2"},
	}
	assert.Len(t, MutatorArithmeticBase(nil, nil, intBin), 1)
}
