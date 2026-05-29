package statement

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorDeferRemove(t *testing.T) {
	test.Mutator(
		t,
		MutatorDeferRemove,
		"../../testdata/statement/defer_remove.go",
		2,
	)
}

func TestMutatorDeferRemoveSelect(t *testing.T) {
	test.Mutator(
		t,
		MutatorDeferRemove,
		"../../testdata/statement/defer_remove_select.go",
		1,
	)
}

func TestMutatorDeferRemoveRegistered(t *testing.T) {
	if _, err := mutator.New("statement/defer-remove"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}
