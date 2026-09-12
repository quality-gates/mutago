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
