package expression

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorLogicalRegistered(t *testing.T) {
	if _, err := mutator.New("expression/logical"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorLogical(t *testing.T) {
	test.Mutator(
		t,
		MutatorLogical,
		"../../testdata/expression/logical.go",
		2,
	)
}
