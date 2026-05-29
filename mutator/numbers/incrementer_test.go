package numbers

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorNumbersIncrementer(t *testing.T) {
	test.Mutator(
		t,
		MutatorNumbersIncrementer,
		"../../testdata/numbers/incrementer.go",
		3,
	)
}

func TestMutatorNumbersIncrementerRegistered(t *testing.T) {
	if _, err := mutator.New("numbers/incrementer"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}
