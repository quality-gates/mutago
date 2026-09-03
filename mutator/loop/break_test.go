package loop

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/quality-gates/mutago/v2"
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

func TestMutatorLoopBreak_IgnoresSwitchAndSelect(t *testing.T) {
	src := `package main

func foo(x int, ch chan int) {
	switch x {
	case 1:
		break
	}
	select {
	case <-ch:
		break
	}
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	changed := mutago.MutateWalkWithPositions(nil, nil, file, MutatorLoopBreak)
	var count int
	for range changed {
		count++
		changed <- mutago.PositionedMutation{}
		<-changed
		changed <- mutago.PositionedMutation{}
	}
	if count != 0 {
		t.Fatalf("expected 0 mutations for break in switch/select, got %d", count)
	}
}
