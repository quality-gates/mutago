package annotation

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestDifferentCollectionsDoNotAffectEachOthersStatements(t *testing.T) {
	parse := func(source string) (*ast.File, *token.FileSet) {
		fs := token.NewFileSet()
		file, err := parser.ParseFile(fs, "test.go", source, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		return file, fs
	}
	unannotated, fs1 := parse("package p\nfunc f(){\n// ordinary comment           \nx++\n}\n")
	annotated, fs2 := parse("package p\nfunc f(){\n// mutator-disable-next-line *\nx++\n}\n")
	stmt := unannotated.Decls[0].(*ast.FuncDecl).Body.List[0]
	first := NewProcessor()
	defer first.Close()
	first.Collect(unannotated, fs1, "a.go")
	second := NewProcessor()
	defer second.Close()
	second.Collect(annotated, fs2, "b.go")
	if HandleBlockStmt(stmt, "statement/remove") {
		t.Fatal("other file's annotation excluded unannotated statement")
	}
}

func TestClosingOneProcessorKeepsOtherExclusions(t *testing.T) {
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, "test.go", "package p\nfunc f(){\n// mutator-disable-next-line *\nx++\n}\n", parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	stmt := file.Decls[0].(*ast.FuncDecl).Body.List[0]
	first, second := NewProcessor(), NewProcessor()
	first.Collect(file, fs, "test.go")
	second.Collect(file, fs, "test.go")
	first.Close()
	if !HandleBlockStmt(stmt) {
		t.Fatal("closing one owner removed another owner's exclusion")
	}
	second.Close()
	if HandleBlockStmt(stmt) {
		t.Fatal("closed processors left an exclusion behind")
	}
	second.Close()
}
