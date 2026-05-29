package selectmutator

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorSelectDefaultRemoveRegistered(t *testing.T) {
	if _, err := mutator.New("select/default-remove"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorSelectDefaultRemove(t *testing.T) {
	test.Mutator(
		t,
		MutatorSelectDefaultRemove,
		"../../testdata/select/default_remove.go",
		1,
	)
}
