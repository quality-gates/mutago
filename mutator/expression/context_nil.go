package expression

import (
	"go/ast"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("expression/context-nil", MutatorContextNil)
}

// MutatorContextNil replaces context.Context arguments at call sites with nil.
// Mirrors ooze's context-cancellation operator.
func MutatorContextNil(_ *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	call, ok := node.(*ast.CallExpr)
	if !ok || info == nil {
		return nil
	}

	var mutations []mutator.Mutation
	for i, arg := range call.Args {
		if !isContextType(info, arg) {
			continue
		}
		if ident, ok := arg.(*ast.Ident); ok {
			switch ident.Name {
			case "nil":
				continue
			}
		}

		idx := i
		original := call.Args[idx]
		mutations = append(mutations, mutator.Mutation{
			Position: original.Pos(),
			Change:   func() { call.Args[idx] = ast.NewIdent("nil") },
			Reset:    func() { call.Args[idx] = original },
		})
	}
	return mutations
}

// isContextType reports whether expr has type context.Context.
func isContextType(info *types.Info, expr ast.Expr) bool {
	t := info.TypeOf(expr)
	return t != nil && t.String() == "context.Context"
}
