package loop

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorLoopCondition(t *testing.T) {
	test.Mutator(
		t,
		MutatorLoopCondition,
		"../../testdata/loop/condition.go",
		2,
	)
}

func TestMutatorLoopConditionNeq(t *testing.T) {
	test.Mutator(
		t,
		MutatorLoopCondition,
		"../../testdata/loop/condition_neq.go",
		1,
	)
}

func TestMutatorLoopConditionRegistered(t *testing.T) {
	if _, err := mutator.New("loop/condition"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}
