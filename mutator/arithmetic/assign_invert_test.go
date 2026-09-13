package arithmetic

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorArithmeticAssignInvert(t *testing.T) {
	test.Mutator(
		t,
		MutatorArithmeticAssignInvert,
		"../../testdata/arithmetic/assign_invert.go",
		5,
	)
}

func TestMutatorArithmeticAssignInvertRegistered(t *testing.T) {
	if _, err := mutator.New("arithmetic/assign_invert"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorArithmeticAssignInvert_SkipsStrings(t *testing.T) {
	src := `package main
func appendStr(a, b string) {
	a += b
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
		muts := MutatorArithmeticAssignInvert(nil, info, n)
		count += len(muts)
		return true
	})
	if count != 0 {
		t.Fatalf("expected 0 mutations on string +=, got %d", count)
	}
}

func TestMutatorArithmeticAssignInvert_SkipsZeroMultiplication(t *testing.T) {
	tests := []struct {
		name      string
		node      *ast.AssignStmt
		wantCount int
	}{
		{
			name: "int 0 literal",
			node: &ast.AssignStmt{
				Tok: token.MUL_ASSIGN,
				Lhs: []ast.Expr{ast.NewIdent("x")},
				Rhs: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "0"}},
			},
			wantCount: 0,
		},
		{
			name: "float 0.0 literal",
			node: &ast.AssignStmt{
				Tok: token.MUL_ASSIGN,
				Lhs: []ast.Expr{ast.NewIdent("x")},
				Rhs: []ast.Expr{&ast.BasicLit{Kind: token.FLOAT, Value: "0.0"}},
			},
			wantCount: 0,
		},
		{
			name: "hex 0x0 literal",
			node: &ast.AssignStmt{
				Tok: token.MUL_ASSIGN,
				Lhs: []ast.Expr{ast.NewIdent("x")},
				Rhs: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "0x0"}},
			},
			wantCount: 0,
		},
		{
			name: "parenthesized 0 literal",
			node: &ast.AssignStmt{
				Tok: token.MUL_ASSIGN,
				Lhs: []ast.Expr{ast.NewIdent("x")},
				Rhs: []ast.Expr{&ast.ParenExpr{X: &ast.BasicLit{Kind: token.INT, Value: "0"}}},
			},
			wantCount: 0,
		},
		{
			name: "unary plus 0 literal",
			node: &ast.AssignStmt{
				Tok: token.MUL_ASSIGN,
				Lhs: []ast.Expr{ast.NewIdent("x")},
				Rhs: []ast.Expr{&ast.UnaryExpr{Op: token.ADD, X: &ast.BasicLit{Kind: token.INT, Value: "0"}}},
			},
			wantCount: 0,
		},
		{
			name: "unary minus 0 literal",
			node: &ast.AssignStmt{
				Tok: token.MUL_ASSIGN,
				Lhs: []ast.Expr{ast.NewIdent("x")},
				Rhs: []ast.Expr{&ast.UnaryExpr{Op: token.SUB, X: &ast.BasicLit{Kind: token.INT, Value: "0"}}},
			},
			wantCount: 0,
		},
		{
			name: "unary plus 1 non-zero literal",
			node: &ast.AssignStmt{
				Tok: token.MUL_ASSIGN,
				Lhs: []ast.Expr{ast.NewIdent("x")},
				Rhs: []ast.Expr{&ast.UnaryExpr{Op: token.ADD, X: &ast.BasicLit{Kind: token.INT, Value: "1"}}},
			},
			wantCount: 1,
		},
		{
			name: "unary minus 1 non-zero literal",
			node: &ast.AssignStmt{
				Tok: token.MUL_ASSIGN,
				Lhs: []ast.Expr{ast.NewIdent("x")},
				Rhs: []ast.Expr{&ast.UnaryExpr{Op: token.SUB, X: &ast.BasicLit{Kind: token.INT, Value: "1"}}},
			},
			wantCount: 1,
		},
		{
			name: "unary bitwise not 0 literal",
			node: &ast.AssignStmt{
				Tok: token.MUL_ASSIGN,
				Lhs: []ast.Expr{ast.NewIdent("x")},
				Rhs: []ast.Expr{&ast.UnaryExpr{Op: token.XOR, X: &ast.BasicLit{Kind: token.INT, Value: "0"}}},
			},
			wantCount: 1,
		},
		{
			name: "int 1 non-zero literal",
			node: &ast.AssignStmt{
				Tok: token.MUL_ASSIGN,
				Lhs: []ast.Expr{ast.NewIdent("x")},
				Rhs: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "1"}},
			},
			wantCount: 1,
		},
		{
			name: "variable y operand",
			node: &ast.AssignStmt{
				Tok: token.MUL_ASSIGN,
				Lhs: []ast.Expr{ast.NewIdent("x")},
				Rhs: []ast.Expr{ast.NewIdent("y")},
			},
			wantCount: 1,
		},
		{
			name: "addition with zero operand not skipped",
			node: &ast.AssignStmt{
				Tok: token.ADD_ASSIGN,
				Lhs: []ast.Expr{ast.NewIdent("x")},
				Rhs: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "0"}},
			},
			wantCount: 1,
		},
		{
			name: "subtraction with zero operand not skipped",
			node: &ast.AssignStmt{
				Tok: token.SUB_ASSIGN,
				Lhs: []ast.Expr{ast.NewIdent("x")},
				Rhs: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "0"}},
			},
			wantCount: 1,
		},
		{
			name: "empty Rhs does not panic and is not skipped",
			node: &ast.AssignStmt{
				Tok: token.MUL_ASSIGN,
				Lhs: []ast.Expr{ast.NewIdent("x")},
				Rhs: nil,
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			muts := MutatorArithmeticAssignInvert(nil, nil, tt.node)
			assert.Len(t, muts, tt.wantCount)
		})
	}
}

func TestMutatorArithmeticAssignInvert_SkipsZeroMultiplication_WithTypeInfo(t *testing.T) {
	src := `package main

const zeroConst = 0
const nonZeroConst = 5

func calc(x int) int {
	x *= 0
	x *= 0.0
	x *= zeroConst
	x *= 1
	y := 2
	x *= y
	x += 0
	x *= nonZeroConst
	return x
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
		stmt, ok := n.(*ast.AssignStmt)
		if !ok || stmt.Tok != token.MUL_ASSIGN || len(stmt.Rhs) == 0 {
			return true
		}
		muts := MutatorArithmeticAssignInvert(nil, info, stmt)
		switch r := stmt.Rhs[0].(type) {
		case *ast.BasicLit:
			if r.Value == "0" || r.Value == "0.0" {
				zeroMutations += len(muts)
			} else {
				nonZeroMutations += len(muts)
			}
		case *ast.Ident:
			if r.Name == "zeroConst" {
				zeroMutations += len(muts)
			} else {
				nonZeroMutations += len(muts)
			}
		}
		return true
	})

	assert.Equal(t, 0, zeroMutations, "expected 0 mutations on *= 0, *= 0.0, *= zeroConst")
	assert.Equal(t, 3, nonZeroMutations, "expected 3 mutations on *= 1, *= y, and *= nonZeroConst")
}

func TestMutatorArithmeticAssignInvert_MutantsCompileCleanly(t *testing.T) {
	src := `package main

const zeroConst = 0

func calc(x int) int {
	x *= 0
	x *= 0.0
	x *= zeroConst
	x *= 1
	y := 2
	x *= y
	return x
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
		muts := MutatorArithmeticAssignInvert(nil, info, n)
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
