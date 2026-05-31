package composite

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorFieldClearRegistered(t *testing.T) {
	if _, err := mutator.New("composite/field-clear"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorFieldClear(t *testing.T) {
	test.Mutator(
		t,
		MutatorFieldClear,
		"../../testdata/composite/field_clear.go",
		4,
	)
}
