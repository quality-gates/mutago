package arithmetic

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("arithmetic/bitwise", MutatorArithmeticBitwise)
}

var bitwiseMutations = map[token.Token]token.Token{
	token.AND:     token.OR,
	token.OR:      token.AND,
	token.XOR:     token.AND,
	token.AND_NOT: token.AND,
	token.SHL:     token.SHR,
	token.SHR:     token.SHL,
}

// MutatorArithmeticBitwise implements a mutator to change bitwise arithmetic.
func MutatorArithmeticBitwise(_ *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.BinaryExpr)
	if !ok {
		return nil
	}

	// In Go 1.18+ generics, a type union constraint such as `*A | *B | *C`
	// inside an interface body is represented as a chain of BinaryExpr nodes
	// with Op=token.OR. The type-checker classifies every node in the chain as
	// a type expression (IsType()==true). Mutating them produces unparseable
	// code (e.g. `*A & *B` in an interface body), so skip them.
	if info != nil {
		if tv, ok2 := info.Types[n]; ok2 && tv.IsType() {
			return nil
		}
	}

	original := n.Op
	mutated, ok := bitwiseMutations[n.Op]
	if !ok {
		return nil
	}

	return []mutator.Mutation{
		{
			Change: func() {
				n.Op = mutated
			},
			Reset: func() {
				n.Op = original
			},
		},
	}
}
