package branch

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorIf(t *testing.T) {
	test.Mutator(
		t,
		MutatorIf,
		"../../testdata/branch/mutateif.go",
		2,
	)
}

func TestMutatorIfRegistered(t *testing.T) {
	if _, err := mutator.New("branch/if"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}
