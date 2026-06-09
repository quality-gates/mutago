// Package composite holds mutators that operate on composite literals
// (struct, map, and keyed array/slice literals).
package composite

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("composite/field-clear", MutatorFieldClear)
}

// MutatorFieldClear drops one keyed element from a composite literal, letting
// the field fall back to its zero value:
//
//	Config{Timeout: 30, Retries: 3}  →  Config{Retries: 3}
//	map[string]int{"a": 1, "b": 2}    →  map[string]int{"b": 2}
//
// This targets fields that are set to a meaningful (non-zero) value but never
// asserted by any test — a pervasive shape in generated Go, where an options
// or config struct is populated in full yet only one or two fields actually
// matter to the test suite. One mutation is emitted per non-zero keyed element.
//
// Elements whose value already looks like the zero value (0, "", false, nil)
// are skipped: dropping them produces an identical-behaviour mutant that would
// always survive and add noise rather than signal. Positional (unkeyed)
// elements are skipped too, since removing one shifts the remaining elements
// and changes meaning unpredictably.
func MutatorFieldClear(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
	lit, ok := node.(*ast.CompositeLit)
	if !ok || len(lit.Elts) == 0 {
		return nil
	}

	original := lit.Elts

	var mutations []mutator.Mutation
	for i, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok || isZeroish(kv.Value) {
			continue
		}

		idx := i
		mutations = append(mutations, mutator.Mutation{
			Position: kv.Pos(),
			Change:   func() { lit.Elts = without(original, idx) },
			Reset:    func() { lit.Elts = original },
		})
	}

	return mutations
}

// without returns a new slice with the element at idx removed, leaving the
// input untouched so Reset can restore it.
func without(elts []ast.Expr, idx int) []ast.Expr {
	out := make([]ast.Expr, 0, len(elts)-1)
	out = append(out, elts[:idx]...)
	out = append(out, elts[idx+1:]...)
	return out
}

// isZeroish reports whether expr is a literal spelling of a zero value, where
// removing the field would not change behaviour.
func isZeroish(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		switch e.Name {
		case "nil", "false":
			return true
		}
	case *ast.BasicLit:
		switch e.Kind {
		case token.INT, token.FLOAT:
			return e.Value == "0" || e.Value == "0.0" || e.Value == "0x0"
		case token.STRING:
			return e.Value == `""` || e.Value == "``"
		}
	}
	return false
}
