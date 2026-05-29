package conditional

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorConditionalNot(t *testing.T) {
	test.Mutator(
		t,
		MutatorConditionalNot,
		"../../testdata/conditional/not.go",
		2,
	)
}

func TestMutatorConditionalNotForStmt(t *testing.T) {
	test.Mutator(
		t,
		MutatorConditionalNot,
		"../../testdata/conditional/not_for.go",
		1,
	)
}

func TestMutatorConditionalNotRegistered(t *testing.T) {
	if _, err := mutator.New("conditional/not"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}
