package expression

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorStringLiteral(t *testing.T) {
	test.Mutator(
		t,
		MutatorStringLiteral,
		"../../testdata/expression/string_literal.go",
		2,
	)
}

func TestMutatorStringLiteralRegistered(t *testing.T) {
	if _, err := mutator.New("expression/string-literal"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}
