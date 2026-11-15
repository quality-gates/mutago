package annotation

import (
	"go/ast"
	"go/token"
	"sync"
)

type blockRegistry struct {
	mu         sync.RWMutex
	statements map[ast.Stmt]map[*Processor][]mutatorInfo
}

var blockExclusions = blockRegistry{statements: make(map[ast.Stmt]map[*Processor][]mutatorInfo)}

// HandleBlockStmt checks the collected exclusions for the supplied statement.
// If mutatorName is omitted, it defaults to "statement/remove".
func HandleBlockStmt(node ast.Stmt, mutatorName ...string) bool {
	name := "statement/remove"
	if len(mutatorName) > 0 && mutatorName[0] != "" {
		name = mutatorName[0]
	}
	blockExclusions.mu.RLock()
	defer blockExclusions.mu.RUnlock()
	for _, infos := range blockExclusions.statements[node] {
		for _, info := range infos {
			if shouldSkipMutator(info, name) {
				return true
			}
		}
	}
	return false
}

func (p *Processor) collectNodesForBlockStmt(file *ast.File) {
	byPosition := make(map[token.Pos][]mutatorInfo)
	for _, kind := range []map[int]map[token.Pos]mutatorInfo{p.RegexAnnotation.Exclusions, p.LineAnnotation.Exclusions} {
		for _, nodes := range kind {
			for pos, info := range nodes {
				byPosition[pos] = append(byPosition[pos], info)
			}
		}
	}
	entries := make(map[ast.Stmt][]mutatorInfo)
	ast.Inspect(file, func(node ast.Node) bool {
		stmt, ok := node.(ast.Stmt)
		if !ok {
			return true
		}
		if infos := byPosition[stmt.Pos()]; len(infos) > 0 {
			entries[stmt] = infos
		}
		return true
	})
	blockExclusions.mu.Lock()
	defer blockExclusions.mu.Unlock()
	p.releaseBlockExclusions()
	for stmt, infos := range entries {
		owners := blockExclusions.statements[stmt]
		if owners == nil {
			owners = make(map[*Processor][]mutatorInfo)
			blockExclusions.statements[stmt] = owners
		}
		owners[p] = infos
		p.blockStatements = append(p.blockStatements, stmt)
	}
}

// Close releases this processor's statement exclusions after mutation completes.
func (p *Processor) Close() {
	blockExclusions.mu.Lock()
	defer blockExclusions.mu.Unlock()
	p.releaseBlockExclusions()
}

func (p *Processor) releaseBlockExclusions() {
	for _, stmt := range p.blockStatements {
		owners := blockExclusions.statements[stmt]
		delete(owners, p)
		if len(owners) == 0 {
			delete(blockExclusions.statements, stmt)
		}
	}
	p.blockStatements = nil
}
