package selectmutator

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorSelectCaseRemoveRegistered(t *testing.T) {
	if _, err := mutator.New("select/case-remove"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorSelectCaseRemove(t *testing.T) {
	test.Mutator(
		t,
		MutatorSelectCaseRemove,
		"../../testdata/select/case_remove.go",
		2,
	)
}
