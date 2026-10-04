package filter

import (
	"go/ast"
	"go/token"
)

// NodeCollector defines the interface for types that can collect and process AST nodes from a source file.
type NodeCollector interface {
	Collect(file *ast.File, fset *token.FileSet, fileAbs string)
}

// NodeFilter defines the interface for types that can determine if an AST node should be excluded from mutation.
type NodeFilter interface {
	ShouldSkip(node ast.Node, mutatorName string) bool
}

// MutationPositionFilter checks whether a mutation should be excluded by its
// original source position.
type MutationPositionFilter interface {
	ShouldSkipPosition(pos token.Pos, mutatorName string) bool
}
