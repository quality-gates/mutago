package expression

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorRecoverClearRegistered(t *testing.T) {
	if _, err := mutator.New("expression/recover-clear"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorRecoverClear(t *testing.T) {
	test.Mutator(
		t,
		MutatorRecoverClear,
		"../../testdata/expression/recover_clear.go",
		2,
	)
}
