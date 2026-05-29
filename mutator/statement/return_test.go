package statement

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorReturnValue(t *testing.T) {
	test.Mutator(
		t,
		MutatorReturnValue,
		"../../testdata/statement/return.go",
		3,
	)
}

func TestMutatorReturnValuePointer(t *testing.T) {
	test.Mutator(
		t,
		MutatorReturnValue,
		"../../testdata/statement/return_pointer.go",
		2,
	)
}

func TestMutatorReturnValueRegistered(t *testing.T) {
	if _, err := mutator.New("statement/return"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}
