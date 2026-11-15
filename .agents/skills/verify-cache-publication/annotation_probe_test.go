package annotation

import (
	"go/ast"
	"go/token"
	"testing"
)

func TestVerificationAnnotationSnapshot(t *testing.T) {
	for mask := 0; mask < 4; mask++ {
		p := NewProcessor()
		statement := &ast.ReturnStmt{Return: token.Pos(42)}
		file := &ast.File{Decls: []ast.Decl{&ast.FuncDecl{Type: &ast.FuncType{Params: &ast.FieldList{}}, Body: &ast.BlockStmt{List: []ast.Stmt{statement}}}}}
		if mask&1 != 0 {
			p.LineAnnotation.Exclusions[1] = map[token.Pos]mutatorInfo{42: newMutatorInfo([]string{"*"})}
		}
		if mask&2 != 0 {
			p.RegexAnnotation.Exclusions[1] = map[token.Pos]mutatorInfo{42: newMutatorInfo([]string{"*"})}
		}
		p.collectNodesForBlockStmt(file)
		if got := HandleBlockStmt(statement); got != (mask != 0) {
			t.Fatalf("published exclusion mismatch mask=%d", mask)
		}
		delete(p.LineAnnotation.Exclusions, 1)
		delete(p.RegexAnnotation.Exclusions, 1)
		if got := HandleBlockStmt(statement); got != (mask != 0) {
			t.Fatalf("published snapshot changed mask=%d", mask)
		}
		if HandleBlockStmt(&ast.ReturnStmt{Return: 42}) {
			t.Fatal("cross-file annotation interfered")
		}
		p.Close()
		if HandleBlockStmt(statement) {
			t.Fatal("closed owner left exclusions")
		}
	}
}

func TestVerificationAnnotationOwners(t *testing.T) {
	statement := &ast.ReturnStmt{Return: 42}
	file := &ast.File{Decls: []ast.Decl{&ast.FuncDecl{Type: &ast.FuncType{Params: &ast.FieldList{}}, Body: &ast.BlockStmt{List: []ast.Stmt{statement}}}}}
	p, q := NewProcessor(), NewProcessor()
	defer p.Close()
	defer q.Close()
	for _, owner := range []*Processor{p, q} {
		owner.LineAnnotation.Exclusions[1] = map[token.Pos]mutatorInfo{42: newMutatorInfo([]string{"*"})}
		owner.collectNodesForBlockStmt(file)
	}
	p.Close()
	if !HandleBlockStmt(statement) {
		t.Fatal("closing one owner removed another owner's exclusion")
	}
	q.Close()
	if HandleBlockStmt(statement) {
		t.Fatal("closed owner left exclusions")
	}
}
