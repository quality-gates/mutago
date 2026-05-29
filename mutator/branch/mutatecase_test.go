package branch

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorCase(t *testing.T) {
	test.Mutator(
		t,
		MutatorCase,
		"../../testdata/branch/mutatecase.go",
		3,
	)
}

func TestMutatorCaseRegistered(t *testing.T) {
	if _, err := mutator.New("branch/case"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}
