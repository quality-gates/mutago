package astutil

import (
	"go/ast"
	"reflect"
	"testing"
)

func TestIncrementalIdentifierIndexMatchesFreshAfterClear(t *testing.T) {
	_, file, pkg, info := parseAndTypeCheckSource(t, `package example
func f(x int) int { y := x + 1; println(y); return x + y }`)
	stmts := file.Decls[0].(*ast.FuncDecl).Body.List
	names := func(exprs []ast.Expr) []string {
		var result []string
		for _, e := range exprs {
			if id, ok := e.(*ast.Ident); ok {
				result = append(result, id.Name)
			}
		}
		return result
	}
	ClearIdentifierCache()
	identifiersInStatements(pkg, info, stmts[0:1])
	incremental := names(identifiersInStatements(pkg, info, stmts[0:]))
	ClearIdentifierCache()
	fresh := names(identifiersInStatements(pkg, info, stmts[0:]))
	if len(fresh) == 0 {
		t.Fatal("expected identifier uses")
	}
	if !reflect.DeepEqual(incremental, fresh) {
		t.Fatalf("incremental identifiers %v differ from fresh %v", incremental, fresh)
	}
	last := stmts[len(stmts)-1].(*ast.ReturnStmt).Results[0]
	before := len(UnsafeLocalVars(info, last))
	ClearSafetyCache()
	if after := len(UnsafeLocalVars(info, last)); before != after {
		t.Fatalf("safety result changed after clear: %d to %d", before, after)
	}
}

func TestSafetyIndexRebuildAfterClear(t *testing.T) {
	_, file, _, info := parseAndTypeCheckSource(t, `package example
func f() int { x := 1; return x }`)
	result := file.Decls[0].(*ast.FuncDecl).Body.List[1].(*ast.ReturnStmt).Results[0].(*ast.Ident)
	ClearSafetyCache()
	if len(UnsafeLocalVars(info, result)) != 1 {
		t.Fatal("expected initially unsafe local")
	}
	info.Uses[&ast.Ident{Name: "x"}] = info.Uses[result]
	ClearSafetyCache()
	if len(UnsafeLocalVars(info, result)) != 0 {
		t.Fatal("stale safety index")
	}
}
