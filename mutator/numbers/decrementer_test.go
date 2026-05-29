package numbers

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorNumbersDecrementer(t *testing.T) {
	test.Mutator(
		t,
		MutatorNumbersDecrementer,
		"../../testdata/numbers/decrementer.go",
		3,
	)
}

func TestMutatorNumbersDecrementerRegistered(t *testing.T) {
	if _, err := mutator.New("numbers/decrementer"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}
