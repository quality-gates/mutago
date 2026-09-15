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

func TestMutatorArithmeticAssignment(t *testing.T) {
	test.Mutator(
		t,
		MutatorArithmeticAssignment,
		"../../testdata/arithmetic/assignment.go",
		11,
	)
}

func TestMutatorArithmeticAssignmentRegistered(t *testing.T) {
	if _, err := mutator.New("arithmetic/assignment"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorArithmeticAssignment_SkipsMixedTypeShift(t *testing.T) {
	src := `package main
func foo(shift uint) uint64 {
	var x uint64 = 1
	x <<= shift
	return x
}
`
	assertShiftMutation(t, src, token.SHL_ASSIGN, false)
}

func TestMutatorArithmeticAssignment_ShiftAssignability(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		tok        token.Token
		wantMutate bool
	}{
		{
			name:       "mixed-type shr skipped",
			src:        "package main\nfunc f(s uint) uint64 { var x uint64 = 1; x >>= s; return x }",
			tok:        token.SHR_ASSIGN,
			wantMutate: false,
		},
		{
			name:       "same-type shl mutated",
			src:        "package main\nfunc f(s uint64) uint64 { var x uint64 = 1; x <<= s; return x }",
			tok:        token.SHL_ASSIGN,
			wantMutate: true,
		},
		{
			name:       "same-type shr mutated",
			src:        "package main\nfunc f(s uint64) uint64 { var x uint64 = 1; x >>= s; return x }",
			tok:        token.SHR_ASSIGN,
			wantMutate: true,
		},
		{
			name:       "untyped constant shl mutated",
			src:        "package main\nfunc f() uint64 { var x uint64 = 1; x <<= 1; return x }",
			tok:        token.SHL_ASSIGN,
			wantMutate: true,
		},
		{
			name:       "overflowing untyped constant shl skipped",
			src:        "package main\nfunc f() int8 { var x int8 = 1; x <<= 200; return x }",
			tok:        token.SHL_ASSIGN,
			wantMutate: false,
		},
		{
			name:       "overflowing untyped constant shr skipped",
			src:        "package main\nfunc f() int8 { var x int8 = 1; x >>= 200; return x }",
			tok:        token.SHR_ASSIGN,
			wantMutate: false,
		},
		{
			name:       "int8-fitting untyped constant shl mutated",
			src:        "package main\nfunc f() int8 { var x int8 = 1; x <<= 1; return x }",
			tok:        token.SHL_ASSIGN,
			wantMutate: true,
		},
		{
			name:       "int8 max untyped constant shl mutated",
			src:        "package main\nfunc f() int8 { var x int8 = 1; x <<= 127; return x }",
			tok:        token.SHL_ASSIGN,
			wantMutate: true,
		},
		{
			name:       "int8 max-plus-one untyped constant shl skipped",
			src:        "package main\nfunc f() int8 { var x int8 = 1; x <<= 128; return x }",
			tok:        token.SHL_ASSIGN,
			wantMutate: false,
		},
		{
			name:       "uint8-fitting untyped constant shl mutated",
			src:        "package main\nfunc f() uint8 { var x uint8 = 1; x <<= 200; return x }",
			tok:        token.SHL_ASSIGN,
			wantMutate: true,
		},
		{
			name:       "uint8 max untyped constant shl mutated",
			src:        "package main\nfunc f() uint8 { var x uint8 = 1; x <<= 255; return x }",
			tok:        token.SHL_ASSIGN,
			wantMutate: true,
		},
		{
			name:       "uint8 overflowing untyped constant shl skipped",
			src:        "package main\nfunc f() uint8 { var x uint8 = 1; x <<= 256; return x }",
			tok:        token.SHL_ASSIGN,
			wantMutate: false,
		},
		{
			name:       "named int8 overflowing untyped constant shl skipped",
			src:        "package main\ntype MyInt8 int8\nfunc f() MyInt8 { var x MyInt8 = 1; x <<= 200; return x }",
			tok:        token.SHL_ASSIGN,
			wantMutate: false,
		},
		{
			name:       "const identifier overflowing untyped shl skipped",
			src:        "package main\nconst n = 200\nfunc f() int8 { var x int8 = 1; x <<= n; return x }",
			tok:        token.SHL_ASSIGN,
			wantMutate: false,
		},
		{
			name:       "untyped constant shr mutated",
			src:        "package main\nfunc f() uint64 { var x uint64 = 1; x >>= 1; return x }",
			tok:        token.SHR_ASSIGN,
			wantMutate: true,
		},
		{
			name:       "add-assign still mutated",
			src:        "package main\nfunc f() int { var x int = 1; x += 1; return x }",
			tok:        token.ADD_ASSIGN,
			wantMutate: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertShiftMutation(t, tt.src, tt.tok, tt.wantMutate)
		})
	}
}

func TestMutatorArithmeticAssignment_UntypedConstantOverflowByType(t *testing.T) {
	cases := []struct {
		typ      string
		fit      string
		overflow string
	}{
		{"int8", "127", "128"},
		{"int16", "32767", "32768"},
		{"int32", "2147483647", "2147483648"},
		{"int64", "9223372036854775807", "9223372036854775808"},
		{"int", "9223372036854775807", "9223372036854775808"},
		{"uint8", "255", "256"},
		{"uint16", "65535", "65536"},
		{"uint32", "4294967295", "4294967296"},
	}
	for _, tt := range cases {
		fitSrc := "package main\nfunc f() " + tt.typ + " { var x " + tt.typ + " = 1; x <<= " + tt.fit + "; return x }"
		overSrc := "package main\nfunc f() " + tt.typ + " { var x " + tt.typ + " = 1; x <<= " + tt.overflow + "; return x }"
		t.Run(tt.typ+" fit", func(t *testing.T) {
			assertShiftMutation(t, fitSrc, token.SHL_ASSIGN, true)
		})
		t.Run(tt.typ+" overflow", func(t *testing.T) {
			assertShiftMutation(t, overSrc, token.SHL_ASSIGN, false)
		})
	}
}

func assertShiftMutation(t *testing.T, src string, tok token.Token, wantMutate bool) {
	t.Helper()
	info, file := mustTypeCheck(t, src)

	var found bool
	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || assign.Tok != tok {
			return true
		}
		found = true
		muts := MutatorArithmeticAssignment(nil, info, assign)
		if wantMutate {
			assert.Len(t, muts, 1)
		} else {
			assert.Nil(t, muts)
		}
		return true
	})
	assert.True(t, found)
}

func mustTypeCheck(t *testing.T, src string) (*types.Info, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	assert.NoError(t, err)

	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
	}
	_, err = new(types.Config).Check("main", fset, []*ast.File{file}, info)
	assert.NoError(t, err)
	return info, file
}

func TestMutatorArithmeticAssignment_NonShiftIgnoresAssignability(t *testing.T) {
	lhs := ast.NewIdent("x")
	rhs := ast.NewIdent("s")
	info := &types.Info{
		Types: map[ast.Expr]types.TypeAndValue{
			lhs: {Type: types.Typ[types.Uint64]},
			rhs: {Type: types.Typ[types.Uint]},
		},
	}
	node := &ast.AssignStmt{
		Tok: token.ADD_ASSIGN,
		Lhs: []ast.Expr{lhs},
		Rhs: []ast.Expr{rhs},
	}
	assert.Len(t, MutatorArithmeticAssignment(nil, info, node), 1)
}

func TestMutatorArithmeticAssignment_NilInfoMutatesShift(t *testing.T) {
	node := &ast.AssignStmt{
		Tok: token.SHL_ASSIGN,
		Lhs: []ast.Expr{ast.NewIdent("x")},
		Rhs: []ast.Expr{ast.NewIdent("s")},
	}
	assert.Len(t, MutatorArithmeticAssignment(nil, nil, node), 1)
}

func TestMutatorArithmeticAssignment_EmptyOperandsMutateShift(t *testing.T) {
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}
	assert.Len(t, MutatorArithmeticAssignment(nil, info, &ast.AssignStmt{Tok: token.SHL_ASSIGN}), 1)
	assert.Len(t, MutatorArithmeticAssignment(nil, info, &ast.AssignStmt{
		Tok: token.SHL_ASSIGN,
		Lhs: []ast.Expr{ast.NewIdent("x")},
	}), 1)
	assert.Len(t, MutatorArithmeticAssignment(nil, info, &ast.AssignStmt{
		Tok: token.SHR_ASSIGN,
		Rhs: []ast.Expr{ast.NewIdent("s")},
	}), 1)
}

func TestMutatorArithmeticAssignment_MissingTypesMutateShift(t *testing.T) {
	lhs := ast.NewIdent("x")
	rhs := ast.NewIdent("s")
	node := &ast.AssignStmt{
		Tok: token.SHL_ASSIGN,
		Lhs: []ast.Expr{lhs},
		Rhs: []ast.Expr{rhs},
	}
	assert.Len(t, MutatorArithmeticAssignment(nil, &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}, node), 1)
	assert.Len(t, MutatorArithmeticAssignment(nil, &types.Info{Types: map[ast.Expr]types.TypeAndValue{
		lhs: {Type: types.Typ[types.Uint64]},
	}}, node), 1)
	assert.Len(t, MutatorArithmeticAssignment(nil, &types.Info{Types: map[ast.Expr]types.TypeAndValue{
		rhs: {Type: types.Typ[types.Uint]},
	}}, node), 1)
}
