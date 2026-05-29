package conditional

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorBoolLiteralRegistered(t *testing.T) {
	if _, err := mutator.New("conditional/bool-literal"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorBoolLiteral(t *testing.T) {
	test.Mutator(
		t,
		MutatorBoolLiteral,
		"../../testdata/conditional/bool_literal.go",
		2,
	)
}
