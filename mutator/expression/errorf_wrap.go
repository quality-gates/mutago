package expression

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("expression/errorf-wrap", MutatorErrorfWrap)
}

// MutatorErrorfWrap downgrades the error-wrapping verb in Errorf-style calls:
//
//	fmt.Errorf("load config: %w", err)  →  fmt.Errorf("load config: %v", err)
//
// The mutated message is byte-for-byte identical, but the returned error no
// longer wraps the cause: errors.Is and errors.As against the original error
// stop matching. This catches the extremely common pattern where code wraps
// errors with %w purely out of habit and no test ever unwraps the result —
// a hallmark of machine-generated Go that looks correct but pins no behaviour.
//
// One mutation is emitted per %w verb in the format string.
func MutatorErrorfWrap(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
	call, ok := node.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return nil
	}
	if !isErrorfCall(call.Fun) {
		return nil
	}

	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return nil
	}

	var mutations []mutator.Mutation
	original := lit.Value
	for _, pos := range wrapVerbPositions(lit.Value) {
		verbPos := pos
		mutations = append(mutations, mutator.Mutation{
			Position: lit.ValuePos,
			Change: func() {
				// verbPos indexes the '%'; the 'w' sits one byte later.
				lit.Value = original[:verbPos+1] + "v" + original[verbPos+2:]
			},
			Reset: func() { lit.Value = original },
		})
	}

	return mutations
}

// isErrorfCall reports whether fun names an Errorf-style function. It matches
// both the package-qualified fmt.Errorf and any selector or identifier ending
// in Errorf (logger wrappers, errors.Errorf shims) so the operator stays
// useful across codebases without depending on import resolution.
func isErrorfCall(fun ast.Expr) bool {
	switch f := fun.(type) {
	case *ast.SelectorExpr:
		return f.Sel.Name == "Errorf"
	case *ast.Ident:
		return f.Name == "Errorf"
	}
	return false
}

// wrapVerbPositions returns the byte offsets of each genuine %w verb in a Go
// string literal value (quotes included). Escaped %% sequences are skipped so
// "100%% done: %w" reports only the real verb.
func wrapVerbPositions(s string) []int {
	var positions []int
	for i := 0; i+1 < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		switch s[i+1] {
		case '%':
			// Escaped percent: consume both bytes.
			i++
		case 'w':
			positions = append(positions, i)
			i++
		}
	}
	return positions
}
