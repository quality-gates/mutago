package concurrency

import (
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorGoroutineRemoveRegistered(t *testing.T) {
	if _, err := mutator.New("concurrency/goroutine-remove"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorGoroutineRemove(t *testing.T) {
	test.Mutator(
		t,
		MutatorGoroutineRemove,
		"../../testdata/concurrency/goroutine_remove.go",
		2,
	)
}
