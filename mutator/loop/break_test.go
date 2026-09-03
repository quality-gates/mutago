package loop

import (
	"go/ast"
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

func TestMutatorLoopBreak_ComplexScoping(t *testing.T) {
	src := `package main

func bar(x any, ch chan int) {
	for i := 0; i < 10; i++ {
		if i == 1 {
			break
		}
		if i == 2 {
			continue
		}
		switch i {
		case 3:
			break
		case 4:
			continue
		}
		switch v := x.(type) {
		case int:
			_ = v
			break
		case string:
			_ = v
			continue
		}
		select {
		case <-ch:
			break
		default:
			continue
		}
		for j := 0; j < 5; j++ {
			break
		}
		fn := func() {
			for {
				break
			}
		}
		_ = fn
	MyLabel:
		for {
			break MyLabel
		}
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
	if count != 7 {
		t.Fatalf("expected 7 mutations across all loops, got %d", count)
	}
}

func TestMutatorLoopBreak_NilGuards(t *testing.T) {
	if ms := MutatorLoopBreak(nil, nil, &ast.ForStmt{}); len(ms) != 0 {
		t.Fatalf("expected 0 mutations for empty ForStmt")
	}
	if ms := MutatorLoopBreak(nil, nil, &ast.RangeStmt{}); len(ms) != 0 {
		t.Fatalf("expected 0 mutations for empty RangeStmt")
	}
	if ms := MutatorLoopBreak(nil, nil, &ast.Ident{}); len(ms) != 0 {
		t.Fatalf("expected 0 mutations for Ident")
	}
	c := &loopBranchCollector{}
	if visitor := c.Visit(nil); visitor != nil {
		t.Fatalf("expected nil for Visit(nil)")
	}
}
