package arithmetic

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorArithmeticAssignment(t *testing.T) {
	test.Mutator(
		t,
		MutatorArithmeticAssignment,
		"../../testdata/arithmetic/assignment.go",
		11,
	)
}

func TestMutatorArithmeticAssignmentRegistered(t *testing.T) {
	if _, err := mutator.New("arithmetic/assignment"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}
