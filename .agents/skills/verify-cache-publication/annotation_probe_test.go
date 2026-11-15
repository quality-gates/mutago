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
		if mask&1 != 0 {
			p.LineAnnotation.Exclusions[1] = map[token.Pos]mutatorInfo{42: newMutatorInfo([]string{"*"})}
		}
		if mask&2 != 0 {
			p.RegexAnnotation.Exclusions[1] = map[token.Pos]mutatorInfo{42: newMutatorInfo([]string{"*"})}
		}
		p.collectNodesForBlockStmt()
		if got := HandleBlockStmt(statement); got != (mask != 0) {
			t.Fatalf("published exclusion mismatch mask=%d", mask)
		}
		delete(p.LineAnnotation.Exclusions, 1)
		delete(p.RegexAnnotation.Exclusions, 1)
		if got := HandleBlockStmt(statement); got != (mask != 0) {
			t.Fatalf("published snapshot changed after collection mask=%d", mask)
		}
		NewProcessor().collectNodesForBlockStmt()
		if HandleBlockStmt(statement) {
			t.Fatal("empty snapshot did not replace previous collection")
		}
	}
}
