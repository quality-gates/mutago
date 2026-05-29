package statement

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorRemoveStatement(t *testing.T) {
	test.Mutator(
		t,
		MutatorRemoveStatement,
		"../../testdata/statement/remove.go",
		17,
	)
}

func TestMutatorRemoveStatementRegistered(t *testing.T) {
	if _, err := mutator.New("statement/remove"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}
