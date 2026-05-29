package loop

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorLoopBreak(t *testing.T) {
	test.Mutator(
		t,
		MutatorLoopBreak,
		"../../testdata/loop/break.go",
		2,
	)
}

func TestMutatorLoopBreakRegistered(t *testing.T) {
	if _, err := mutator.New("loop/break"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}
