package mutago

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"os"

	"github.com/quality-gates/mutago/v2/mutator"
)

// CountWalk returns the number of corresponding mutations for a given mutator.
// It traverses the AST of the given node and calls the method Check of the given mutator for every node and sums up the returned counts. After completion of the traversal the final counter is returned.
func CountWalk(pkg *types.Package, info *types.Info, node ast.Node, m mutator.Mutator) int {
	w := &countWalk{
		count:   0,
		mutator: m,
		pkg:     pkg,
		info:    info,
	}

	ast.Walk(w, node)

	return w.count
}

type countWalk struct {
	count   int
	mutator mutator.Mutator
	pkg     *types.Package
	info    *types.Info
}

// Visit implements the Visit method of the ast.Visitor interface
func (w *countWalk) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return w
	}

	w.count += len(w.mutator(w.pkg, w.info, node))

	return w
}

// MutateWalk mutates the given node with the given mutator returning a channel to control the mutation steps.
// It traverses the AST of the given node and calls the method Check of the given mutator to verify that a node can be mutated by the mutator. If a node can be mutated the method Mutate of the given mutator is executed with the node and the control channel. After completion of the traversal the control channel is closed.
func MutateWalk(pkg *types.Package, info *types.Info, node ast.Node, m mutator.Mutator) chan bool {
	changed := make(chan bool)
	go walkWithHandshake(pkg, info, node, m, changed, func(PositionedMutation) bool { return true }, true)
	return changed
}

// PositionedMutation identifies the original source position of a mutation.
// The channel also carries zero values for the reset/advance handshake.
type PositionedMutation struct {
	Position token.Pos
	Node     ast.Node
	Start    token.Pos
	End      token.Pos
}

// MutateWalkWithPositions behaves like MutateWalk but reports the source
// position of each changed node. It is used by the CLI so filtering and report
// locations come from the parsed source instead of printer-generated diffs.
func MutateWalkWithPositions(pkg *types.Package, info *types.Info, node ast.Node, m mutator.Mutator) chan PositionedMutation {
	changed := make(chan PositionedMutation)
	go walkWithHandshake(pkg, info, node, m, changed, func(m PositionedMutation) PositionedMutation { return m }, PositionedMutation{})
	return changed
}

func walkWithHandshake[T any](pkg *types.Package, info *types.Info, node ast.Node, m mutator.Mutator, changed chan T, applied func(PositionedMutation) T, reset T) {
	defer close(changed)
	handshake := mutationHandshake[T]{changed: changed, applied: applied, reset: reset}
	w := mutationWalk{
		ctx: context.Background(), pkg: pkg, info: info, mutator: m,
		onApplied: handshake.acknowledgeApplied,
		onReset:   handshake.acknowledgeReset,
	}
	ast.Walk(&w, node)
}

type mutationHandshake[T any] struct {
	changed chan T
	applied func(PositionedMutation) T
	reset   T
}

func (h mutationHandshake[T]) acknowledgeApplied(mutation PositionedMutation) error {
	h.changed <- h.applied(mutation)
	<-h.changed
	return nil
}

func (h mutationHandshake[T]) acknowledgeReset() {
	h.changed <- h.reset
	<-h.changed
}

// VisitMutations calls visit while each mutation is applied and restores the
// syntax before advancing, returning when visit fails or ctx is cancelled.
func VisitMutations(ctx context.Context, pkg *types.Package, info *types.Info, node ast.Node, m mutator.Mutator, visit func(PositionedMutation) error) error {
	w := mutationWalk{ctx: ctx, pkg: pkg, info: info, mutator: m, onApplied: visit}
	ast.Walk(&w, node)
	if w.err != nil {
		return w.err
	}
	return ctx.Err()
}

type mutationWalk struct {
	ctx       context.Context
	mutator   mutator.Mutator
	pkg       *types.Package
	info      *types.Info
	onApplied func(PositionedMutation) error
	onReset   func()
	err       error
}

func (w *mutationWalk) Visit(node ast.Node) ast.Visitor {
	if node == nil || w.err != nil {
		return nil
	}
	if w.err = w.ctx.Err(); w.err != nil {
		return nil
	}

	for _, m := range w.mutator(w.pkg, w.info, node) {
		if w.err = w.ctx.Err(); w.err != nil {
			return nil
		}
		position := m.Position
		if !position.IsValid() {
			position = node.Pos()
		}
		start, end := node.Pos(), node.End()

		w.err = withMutation(m, func() error {
			return w.onApplied(PositionedMutation{Position: position, Node: node, Start: start, End: end})
		})
		if w.err != nil {
			return nil
		}
		if w.onReset != nil {
			w.onReset()
		}
	}

	return w
}

func withMutation(m mutator.Mutation, visit func() error) error {
	defer m.Reset()
	m.Change()
	return visit()
}

// PrintWalk traverses the AST of the given node and prints every node to STDOUT.
func PrintWalk(node ast.Node) {
	printWalkTo(os.Stdout, nil, node)
}

func printWalkTo(w io.Writer, fset *token.FileSet, root ast.Node) {
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			return true
		}
		if fset == nil {
			fmt.Fprintf(w, "%T pos=%d end=%d\n", node, node.Pos(), node.End())
		} else {
			fmt.Fprintf(w, "%T pos=%s end=%s\n", node, fset.Position(node.Pos()), fset.Position(node.End()))
		}
		return true
	})
}
