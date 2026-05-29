package expression

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorComparison(t *testing.T) {
	test.Mutator(
		t,
		MutatorComparison,
		"../../testdata/expression/comparison.go",
		4,
	)
}

func TestMutatorComparisonRegistered(t *testing.T) {
	if _, err := mutator.New("expression/comparison"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}
