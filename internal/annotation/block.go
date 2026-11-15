package annotation

import (
	"go/ast"
	"go/token"
	"sync/atomic"
)

type blockExclusions struct {
	regex map[int]map[token.Pos]mutatorInfo
	line  map[int]map[token.Pos]mutatorInfo
}

var currentBlockExclusions atomic.Pointer[blockExclusions]

// HandleBlockStmt checks the latest collected exclusions for a statement.
// If mutatorName is omitted, it defaults to "statement/remove".
func HandleBlockStmt(node ast.Stmt, mutatorName ...string) bool {
	name := "statement/remove"
	if len(mutatorName) > 0 && mutatorName[0] != "" {
		name = mutatorName[0]
	}

	exclusions := currentBlockExclusions.Load()
	if exclusions == nil {
		return false
	}
	for _, kind := range []map[int]map[token.Pos]mutatorInfo{exclusions.regex, exclusions.line} {
		for _, nodes := range kind {
			if info, exists := nodes[node.Pos()]; exists && shouldSkipMutator(info, name) {
				return true
			}
		}
	}

	return false
}

func copyBlockExclusions(source map[int]map[token.Pos]mutatorInfo) map[int]map[token.Pos]mutatorInfo {
	result := make(map[int]map[token.Pos]mutatorInfo, len(source))
	for line, nodes := range source {
		result[line] = make(map[token.Pos]mutatorInfo, len(nodes))
		for pos, info := range nodes {
			result[line][pos] = info
		}
	}
	return result
}
