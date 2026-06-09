package expression

import (
	"go/ast"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("expression/recover-clear", MutatorRecoverClear)
}

// MutatorRecoverClear neutralises a recover() call by turning it into a typed
// nil:
//
//	if r := recover(); r != nil { ... }  →  if r := any(nil); r != nil { ... }
//
// recover() returns any, and so does any(nil), so the rewrite type-checks in
// every context recover() can appear in (assignment, comparison, bare call).
// The difference is behavioural: the recovered value is always nil, so the
// guarded recovery branch never runs and a panic propagates instead of being
// swallowed. Deferred recover blocks are classic untested defensive code —
// generated "just in case" and never exercised. If this mutant survives, the
// panic-safety net is dead weight or, worse, silently masking real failures.
func MutatorRecoverClear(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
	call, ok := node.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return nil
	}
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != "recover" {
		return nil
	}

	originalFun := call.Fun
	originalArgs := call.Args
	return []mutator.Mutation{
		{
			Position: call.Pos(),
			Change: func() {
				call.Fun = ast.NewIdent("any")
				call.Args = []ast.Expr{ast.NewIdent("nil")}
			},
			Reset: func() {
				call.Fun = originalFun
				call.Args = originalArgs
			},
		},
	}
}
