package numbers

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("numbers/decrementer", MutatorNumbersDecrementer)
}

// MutatorNumbersDecrementer implements a mutator to decrement int and float.
func MutatorNumbersDecrementer(_ *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.BasicLit)
	if !ok {
		return nil
	}

	if n.Kind == token.INT {
		original := n.Value
		litInfo, ok := parseIntLiteral(n.Value)
		if !ok {
			return nil
		}

		if shouldSkipDecrement(info, n, litInfo.val) {
			return nil
		}

		mutated := formatIntLiteral(litInfo.val-1, litInfo)

		return []mutator.Mutation{
			{
				Position: n.ValuePos,
				Change: func() {
					n.Value = mutated
				},
				Reset: func() {
					n.Value = original
				},
			},
		}
	}

	if n.Kind == token.FLOAT {
		original := n.Value
		cleaned := strings.ReplaceAll(n.Value, "_", "")
		originalFloat, err := strconv.ParseFloat(cleaned, 64)
		if err != nil {
			return nil
		}

		originalFloat--
		mutated := strconv.FormatFloat(originalFloat, 'f', -1, 64)
		if originalFloat < 0 {
			mutated = "(" + mutated + ")"
		}

		return []mutator.Mutation{
			{
				Position: n.ValuePos,
				Change: func() {
					n.Value = mutated
				},
				Reset: func() {
					n.Value = original
				},
			},
		}
	}

	return nil
}
