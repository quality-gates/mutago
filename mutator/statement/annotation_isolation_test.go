package statement

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/quality-gates/mutago/v2/internal/annotation"
)

func TestStatementMutationUnaffectedByOtherFileAnnotation(t *testing.T) {
	parse := func(source string) (*ast.File, *token.FileSet) {
		t.Helper()
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "file.go", source, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		return file, fset
	}
	first, fset1 := parse("package p\nfunc f(x int){\n// ordinary comment           \nx++\n}\n")
	second, fset2 := parse("package p\nfunc f(x int){\n// mutator-disable-next-line *\nx++\n}\n")
	unannotated, annotated := annotation.NewProcessor(), annotation.NewProcessor()
	defer unannotated.Close()
	defer annotated.Close()
	unannotated.Collect(first, fset1, "first.go")
	annotated.Collect(second, fset2, "second.go")
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}}
	pkg, err := (&types.Config{}).Check("p", fset1, []*ast.File{first}, info)
	if err != nil {
		t.Fatal(err)
	}
	block := first.Decls[0].(*ast.FuncDecl).Body
	filtered := annotation.DecoratorFilter(MutatorRemoveStatement, "statement/remove", unannotated)
	if got := filtered(pkg, info, block); len(got) != 1 {
		t.Fatalf("unannotated increment produced %d mutations, want 1", len(got))
	}
}
