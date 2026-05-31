package expression

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorErrorfWrapRegistered(t *testing.T) {
	if _, err := mutator.New("expression/errorf-wrap"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorErrorfWrap(t *testing.T) {
	test.Mutator(
		t,
		MutatorErrorfWrap,
		"../../testdata/expression/errorf_wrap.go",
		3,
	)
}
