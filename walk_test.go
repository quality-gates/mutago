package mutago

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintWalkWritesOneLinePerNode(t *testing.T) {
	fset := token.NewFileSet()
	src, err := parser.ParseFile(fset, "fixture.go", "package fixture\nvar n = 1 + 2\n", 0)
	require.NoError(t, err)
	nodes := 0
	ast.Inspect(src, func(node ast.Node) bool {
		if node != nil {
			nodes++
		}
		return true
	})
	var output bytes.Buffer
	printWalkTo(&output, fset, src)
	assert.Equal(t, nodes, strings.Count(strings.TrimSpace(output.String()), "\n")+1)
	assert.NotContains(t, output.String(), "&ast.")
}

func TestMutateWalkWithPositionsUsesMutationPosition(t *testing.T) {
	fset := token.NewFileSet()
	src, err := parser.ParseFile(fset, "fixture.go", `package fixture
func f(x int) int {
	// keep this comment above the mutation
	x = x
	return x
}
`, parser.ParseComments)
	require.NoError(t, err)

	positioned := func(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
		block, ok := node.(*ast.BlockStmt)
		if !ok || len(block.List) < 1 {
			return nil
		}
		assign, ok := block.List[0].(*ast.AssignStmt)
		if !ok {
			return nil
		}
		original := block.List[0]
		return []mutator.Mutation{{
			Position: assign.Pos(),
			Change:   func() { block.List[0] = &ast.EmptyStmt{Semicolon: assign.Pos()} },
			Reset:    func() { block.List[0] = original },
		}}
	}

	changed := MutateWalkWithPositions(nil, nil, src, positioned)
	mutation, ok := <-changed
	require.True(t, ok)
	assert.Equal(t, 4, fset.Position(mutation.Position).Line)

	changed <- PositionedMutation{}
	<-changed
	changed <- PositionedMutation{}
	_, ok = <-changed
	assert.False(t, ok)
}

func TestMutateWalkWithPositionsFallsBackToVisitedNode(t *testing.T) {
	fset := token.NewFileSet()
	src, err := parser.ParseFile(fset, "fixture.go", "package fixture\nvar n = 1\n", 0)
	require.NoError(t, err)

	positionless := func(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
		lit, ok := node.(*ast.BasicLit)
		if !ok {
			return nil
		}
		original := lit.Value
		return []mutator.Mutation{{
			Change: func() { lit.Value = "2" },
			Reset:  func() { lit.Value = original },
		}}
	}

	changed := MutateWalkWithPositions(nil, nil, src, positionless)
	mutation, ok := <-changed
	require.True(t, ok)
	assert.Equal(t, 2, fset.Position(mutation.Position).Line)

	changed <- PositionedMutation{}
	<-changed
	changed <- PositionedMutation{}
	_, ok = <-changed
	assert.False(t, ok)
}

func TestMutateWalkCompatibility(t *testing.T) {
	src, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture\nvar n = 1\n", 0)
	require.NoError(t, err)
	lit := src.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Values[0].(*ast.BasicLit)

	positionless := func(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
		lit, ok := node.(*ast.BasicLit)
		if !ok {
			return nil
		}
		original := lit.Value
		return []mutator.Mutation{{
			Change: func() { lit.Value = "2" },
			Reset:  func() { lit.Value = original },
		}}
	}

	changed := MutateWalk(nil, nil, src, positionless)
	mutation, ok := <-changed
	require.True(t, ok)
	assert.True(t, mutation)
	assert.Equal(t, "2", lit.Value)

	changed <- true
	<-changed
	assert.Equal(t, "1", lit.Value)
	changed <- true
	_, ok = <-changed
	assert.False(t, ok)
}

func TestVisitMutationsRestoresSyntaxAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	src, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture\nvar n = 1\n", 0)
	require.NoError(t, err)
	lit := src.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Values[0].(*ast.BasicLit)
	m := func(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
		if node != lit {
			return nil
		}
		change := mutator.Mutation{Change: func() { lit.Value = "2" }, Reset: func() { lit.Value = "1" }}
		return []mutator.Mutation{change, change}
	}
	visits := 0
	err = VisitMutations(ctx, nil, nil, src, m, func(mutation PositionedMutation) error {
		visits++
		assert.Equal(t, "2", lit.Value)
		assert.Equal(t, lit.Pos(), mutation.Position)
		cancel()
		return nil
	})
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, visits)
	assert.Equal(t, "1", lit.Value)
}

func TestVisitMutationsRestoresSyntaxAfterPanic(t *testing.T) {
	src, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture\nvar n = 1\n", 0)
	require.NoError(t, err)
	lit := src.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Values[0].(*ast.BasicLit)
	m := func(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
		if node != lit {
			return nil
		}
		return []mutator.Mutation{{Change: func() { lit.Value = "2" }, Reset: func() { lit.Value = "1" }}}
	}
	assert.Panics(t, func() {
		_ = VisitMutations(context.Background(), nil, nil, src, m, func(PositionedMutation) error { panic("callback") })
	})
	assert.Equal(t, "1", lit.Value)
}

func TestVisitMutationsRestoresSyntaxAfterError(t *testing.T) {
	src, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture\nvar n = 1\n", 0)
	require.NoError(t, err)
	lit := src.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Values[0].(*ast.BasicLit)
	m := func(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
		if node != lit {
			return nil
		}
		return []mutator.Mutation{{Change: func() { lit.Value = "2" }, Reset: func() { lit.Value = "1" }}}
	}
	want := errors.New("stop")
	err = VisitMutations(context.Background(), nil, nil, src, m, func(PositionedMutation) error { return want })
	assert.ErrorIs(t, err, want)
	assert.Equal(t, "1", lit.Value)
}
