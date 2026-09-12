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

func TestMutatorArithmeticBase_SkipsZeroMultiplication(t *testing.T) {
	tests := []struct {
		name      string
		node      *ast.BinaryExpr
		wantCount int
	}{
		{
			name: "int 0 literal",
			node: &ast.BinaryExpr{
				Op: token.MUL,
				X:  ast.NewIdent("x"),
				Y:  &ast.BasicLit{Kind: token.INT, Value: "0"},
			},
			wantCount: 0,
		},
		{
			name: "float 0.0 literal",
			node: &ast.BinaryExpr{
				Op: token.MUL,
				X:  ast.NewIdent("x"),
				Y:  &ast.BasicLit{Kind: token.FLOAT, Value: "0.0"},
			},
			wantCount: 0,
		},
		{
			name: "hex 0x0 literal",
			node: &ast.BinaryExpr{
				Op: token.MUL,
				X:  ast.NewIdent("x"),
				Y:  &ast.BasicLit{Kind: token.INT, Value: "0x0"},
			},
			wantCount: 0,
		},
		{
			name: "parenthesized 0 literal",
			node: &ast.BinaryExpr{
				Op: token.MUL,
				X:  ast.NewIdent("x"),
				Y:  &ast.ParenExpr{X: &ast.BasicLit{Kind: token.INT, Value: "0"}},
			},
			wantCount: 0,
		},
		{
			name: "unary plus 0 literal",
			node: &ast.BinaryExpr{
				Op: token.MUL,
				X:  ast.NewIdent("x"),
				Y:  &ast.UnaryExpr{Op: token.ADD, X: &ast.BasicLit{Kind: token.INT, Value: "0"}},
			},
			wantCount: 0,
		},
		{
			name: "unary minus 0 literal",
			node: &ast.BinaryExpr{
				Op: token.MUL,
				X:  ast.NewIdent("x"),
				Y:  &ast.UnaryExpr{Op: token.SUB, X: &ast.BasicLit{Kind: token.INT, Value: "0"}},
			},
			wantCount: 0,
		},
		{
			name: "unary plus 1 non-zero literal",
			node: &ast.BinaryExpr{
				Op: token.MUL,
				X:  ast.NewIdent("x"),
				Y:  &ast.UnaryExpr{Op: token.ADD, X: &ast.BasicLit{Kind: token.INT, Value: "1"}},
			},
			wantCount: 1,
		},
		{
			name: "unary minus 1 non-zero literal",
			node: &ast.BinaryExpr{
				Op: token.MUL,
				X:  ast.NewIdent("x"),
				Y:  &ast.UnaryExpr{Op: token.SUB, X: &ast.BasicLit{Kind: token.INT, Value: "1"}},
			},
			wantCount: 1,
		},
		{
			name: "unary bitwise not 0 literal",
			node: &ast.BinaryExpr{
				Op: token.MUL,
				X:  ast.NewIdent("x"),
				Y:  &ast.UnaryExpr{Op: token.XOR, X: &ast.BasicLit{Kind: token.INT, Value: "0"}},
			},
			wantCount: 1,
		},
		{
			name: "int 1 non-zero literal",
			node: &ast.BinaryExpr{
				Op: token.MUL,
				X:  ast.NewIdent("x"),
				Y:  &ast.BasicLit{Kind: token.INT, Value: "1"},
			},
			wantCount: 1,
		},
		{
			name: "variable y operand",
			node: &ast.BinaryExpr{
				Op: token.MUL,
				X:  ast.NewIdent("x"),
				Y:  ast.NewIdent("y"),
			},
			wantCount: 1,
		},
		{
			name: "left operand 0, right operand non-zero",
			node: &ast.BinaryExpr{
				Op: token.MUL,
				X:  &ast.BasicLit{Kind: token.INT, Value: "0"},
				Y:  ast.NewIdent("x"),
			},
			wantCount: 1,
		},
		{
			name: "addition with zero operand not skipped",
			node: &ast.BinaryExpr{
				Op: token.ADD,
				X:  ast.NewIdent("x"),
				Y:  &ast.BasicLit{Kind: token.INT, Value: "0"},
			},
			wantCount: 1,
		},
		{
			name: "subtraction with zero operand not skipped",
			node: &ast.BinaryExpr{
				Op: token.SUB,
				X:  ast.NewIdent("x"),
				Y:  &ast.BasicLit{Kind: token.INT, Value: "0"},
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			muts := MutatorArithmeticBase(nil, nil, tt.node)
			assert.Len(t, muts, tt.wantCount)
		})
	}
}

func TestMutatorArithmeticBase_SkipsZeroMultiplication_WithTypeInfo(t *testing.T) {
	src := `package main

const zeroConst = 0
const nonZeroConst = 5

func calc(x int) int {
	a := x * 0
	b := x * 0.0
	c := x * zeroConst
	d := x * 1
	e := x * a
	f := x + 0
	g := x * nonZeroConst
	return a + b + c + d + e + f + g
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

	var zeroMutations, nonZeroMutations int
	ast.Inspect(file, func(n ast.Node) bool {
		bin, ok := n.(*ast.BinaryExpr)
		if !ok || bin.Op != token.MUL {
			return true
		}
		muts := MutatorArithmeticBase(nil, info, bin)
		switch y := bin.Y.(type) {
		case *ast.BasicLit:
			if y.Value == "0" || y.Value == "0.0" {
				zeroMutations += len(muts)
			} else {
				nonZeroMutations += len(muts)
			}
		case *ast.Ident:
			if y.Name == "zeroConst" {
				zeroMutations += len(muts)
			} else {
				nonZeroMutations += len(muts)
			}
		}
		return true
	})

	assert.Equal(t, 0, zeroMutations, "expected 0 mutations on * 0, * 0.0, * zeroConst")
	assert.Equal(t, 3, nonZeroMutations, "expected 3 mutations on * 1, * a, and * nonZeroConst")
}

func TestMutatorArithmeticBase_MutantsCompileCleanly(t *testing.T) {
	src := `package main

const zeroConst = 0

func calc(x int) int {
	a := x * 0
	b := x * 0.0
	c := x * zeroConst
	d := x * 1
	e := x * a
	return a + int(b) + c + d + e
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

	var allMutations []mutator.Mutation
	ast.Inspect(file, func(n ast.Node) bool {
		muts := MutatorArithmeticBase(nil, info, n)
		allMutations = append(allMutations, muts...)
		return true
	})

	for i, m := range allMutations {
		m.Change()
		checkInfo := &types.Info{
			Types: make(map[ast.Expr]types.TypeAndValue),
		}
		_, compileErr := conf.Check("main", fset, []*ast.File{file}, checkInfo)
		m.Reset()
		assert.NoError(t, compileErr, "mutant %d failed to compile: %v", i, compileErr)
	}
}
