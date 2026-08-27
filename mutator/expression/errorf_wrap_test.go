package expression

import (
	"go/ast"
	"go/token"
	"runtime"
	"strings"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func BenchmarkMutatorErrorfWrapManyVerbs(b *testing.B) {
	literal := `"` + strings.Repeat("%w ", 1000) + `"`
	node := &ast.CallExpr{
		Fun:  &ast.SelectorExpr{X: ast.NewIdent("fmt"), Sel: ast.NewIdent("Errorf")},
		Args: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: literal}},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		runtime.KeepAlive(MutatorErrorfWrap(nil, nil, node))
	}
}

func TestMutatorErrorfWrapRegistered(t *testing.T) {
	if _, err := mutator.New("expression/errorf-wrap"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorErrorfWrap(t *testing.T) {
	test.Mutator(
		t,
		MutatorErrorfWrap,
		"../../testdata/expression/errorf_wrap.go",
		3,
	)
}
