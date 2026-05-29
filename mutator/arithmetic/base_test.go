package arithmetic

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorArithmeticBase(t *testing.T) {
	test.Mutator(
		t,
		MutatorArithmeticBase,
		"../../testdata/arithmetic/base.go",
		5,
	)
}

func TestMutatorArithmeticBaseRegistered(t *testing.T) {
	if _, err := mutator.New("arithmetic/base"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}
