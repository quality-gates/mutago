package loop

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorLoopRangeBreakRegistered(t *testing.T) {
	if _, err := mutator.New("loop/range_break"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorLoopRangeBreak(t *testing.T) {
	test.Mutator(
		t,
		MutatorLoopRangeBreak,
		"../../testdata/loop/range_break.go",
		2,
	)
}
