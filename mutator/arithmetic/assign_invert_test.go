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
