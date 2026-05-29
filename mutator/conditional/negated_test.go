package conditional

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorConditionalNegated(t *testing.T) {
	test.Mutator(
		t,
		MutatorConditionalNegated,
		"../../testdata/conditional/negated.go",
		6,
	)
}

func TestMutatorConditionalNegatedRegistered(t *testing.T) {
	if _, err := mutator.New("conditional/negated"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}
