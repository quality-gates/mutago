package arithmetic

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
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
	x >>= shift
	return x
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
	if _, err := (&types.Config{}).Check("main", fset, []*ast.File{file}, info); err != nil {
		t.Fatal(err)
	}

	var count int
	ast.Inspect(file, func(n ast.Node) bool {
		count += len(MutatorArithmeticAssignment(nil, info, n))
		return true
	})
	if count != 0 {
		t.Fatalf("expected 0 mutations on mixed-type <<= / >>=, got %d", count)
	}
}

func TestMutatorArithmeticAssignment_KeepsSameTypeShift(t *testing.T) {
	src := `package main
func foo(shift uint64) uint64 {
	var x uint64 = 1
	x <<= shift
	x >>= shift
	return x
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
	if _, err := (&types.Config{}).Check("main", fset, []*ast.File{file}, info); err != nil {
		t.Fatal(err)
	}

	var count int
	ast.Inspect(file, func(n ast.Node) bool {
		count += len(MutatorArithmeticAssignment(nil, info, n))
		return true
	})
	if count != 2 {
		t.Fatalf("expected 2 mutations on same-type <<= / >>=, got %d", count)
	}
}

func TestMutatorArithmeticAssignment_KeepsUntypedConstShift(t *testing.T) {
	src := `package main
func foo() uint64 {
	var x uint64 = 1
	x <<= 1
	x >>= 2
	return x
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
	if _, err := (&types.Config{}).Check("main", fset, []*ast.File{file}, info); err != nil {
		t.Fatal(err)
	}

	var count int
	ast.Inspect(file, func(n ast.Node) bool {
		count += len(MutatorArithmeticAssignment(nil, info, n))
		return true
	})
	if count != 2 {
		t.Fatalf("expected 2 mutations on untyped const <<= / >>=, got %d", count)
	}
}
