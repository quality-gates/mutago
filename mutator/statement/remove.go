package statement

import (
	"github.com/quality-gates/mutago/v2/internal/annotation"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/quality-gates/mutago/v2/astutil"
	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("statement/remove", MutatorRemoveStatement)
}

func checkRemoveStatement(node ast.Stmt) bool {
	skip := annotation.HandleBlockStmt(node)
	if skip {
		return false
	}

	switch n := node.(type) {
	case *ast.AssignStmt:
		if n.Tok != token.DEFINE {
			// Skip assignments whose entire LHS is already blank identifiers
			// (e.g. _, _, _ = a, b, c). The noop replacement would be identical,
			// producing a diff-less mutation that always escapes.
			allBlank := true
			for _, lhs := range n.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok {
					allBlank = false
					break
				}
				switch id.Name {
				case "_":
				default:
					allBlank = false
				}
			}
			if !allBlank {
				return true
			}
		}
	case *ast.ExprStmt, *ast.IncDecStmt:
		return true
	}

	return false
}

// MutatorRemoveStatement implements a mutator to remove statements.
func MutatorRemoveStatement(pkg *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	var l []ast.Stmt

	switch n := node.(type) {
	case *ast.BlockStmt:
		l = n.List
	case *ast.CaseClause:
		l = n.Body
	case *ast.CommClause:
		l = n.Body
	}

	var mutations []mutator.Mutation

	for i, ni := range l {
		if checkRemoveStatement(ni) {
			li := i
			old := l[li]

			mutations = append(mutations, mutator.Mutation{
				Position: old.Pos(),
				Change: func() {
					l[li] = astutil.CreateNoopOfStatement(pkg, info, old)
				},
				Reset: func() {
					l[li] = old
				},
			})
		}
	}

	return mutations
}
