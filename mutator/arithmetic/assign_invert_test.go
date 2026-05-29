package arithmetic

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorArithmeticAssignInvert(t *testing.T) {
	test.Mutator(
		t,
		MutatorArithmeticAssignInvert,
		"../../testdata/arithmetic/assign_invert.go",
		5,
	)
}

func TestMutatorArithmeticAssignInvertRegistered(t *testing.T) {
	if _, err := mutator.New("arithmetic/assign_invert"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}
