package expression

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorRemoveTermRegistered(t *testing.T) {
	if _, err := mutator.New("expression/remove"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorRemoveTerm(t *testing.T) {
	test.Mutator(
		t,
		MutatorRemoveTerm,
		"../../testdata/expression/remove.go",
		6,
	)
}
