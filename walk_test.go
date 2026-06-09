package mutago

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	changed <- true
	<-changed
	changed <- true
	_, ok = <-changed
	assert.False(t, ok)
}
