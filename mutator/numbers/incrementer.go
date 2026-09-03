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
	mutator.Register("numbers/incrementer", MutatorNumbersIncrementer)
}

// MutatorNumbersIncrementer implements a mutator to increment int and float.
func MutatorNumbersIncrementer(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.BasicLit)
	if !ok {
		return nil
	}

	if n.Kind == token.INT {
		original := n.Value
		info, ok := parseIntLiteral(n.Value)
		if !ok {
			return nil
		}

		mutatedVal := info.val + 1
		mutated := formatIntLiteral(mutatedVal, info)

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

		originalFloat++
		mutated := strconv.FormatFloat(originalFloat, 'f', -1, 64)

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
