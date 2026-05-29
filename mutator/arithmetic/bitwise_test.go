package arithmetic

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorArithmeticBitwise(t *testing.T) {
	test.Mutator(
		t,
		MutatorArithmeticBitwise,
		"../../testdata/arithmetic/bitwise.go",
		6,
	)
}

func TestMutatorArithmeticBitwiseSkipsTypeUnion(t *testing.T) {
	// A file whose only | operators are generics type union constraints must
	// produce zero mutations — mutating them yields unparseable code.
	test.Mutator(
		t,
		MutatorArithmeticBitwise,
		"../../testdata/arithmetic/bitwise_generics.go",
		0,
	)
}

func TestMutatorArithmeticBitwiseRegistered(t *testing.T) {
	if _, err := mutator.New("arithmetic/bitwise"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}
