package mutago

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"testing"
	"time"

	"github.com/quality-gates/mutago/v2/mutator"
)

func probeMutator(t *testing.T, id *ast.Ident) mutator.Mutator {
	t.Helper()
	return func(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
		if node != id {
			return nil
		}
		var ms []mutator.Mutation
		for i := 1; i <= 2; i++ {
			name := fmt.Sprintf("changed%d", i)
			ms = append(ms, mutator.Mutation{Change: func() {
				if id.Name != "original" {
					t.Errorf("next mutation saw un-restored source: %s", id.Name)
				}
				id.Name = name
			}, Reset: func() { id.Name = "original" }})
		}
		return ms
	}
}

func TestWalkRestorationProbe(t *testing.T) {
	for _, outcome := range []string{"ok", "error", "cancel", "panic"} {
		t.Run(outcome, func(t *testing.T) {
			id := &ast.Ident{Name: "original"}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sentinel := errors.New("callback failed")
			calls := 0
			var err error
			var recovered any
			func() {
				defer func() { recovered = recover() }()
				err = VisitMutations(ctx, nil, nil, id, probeMutator(t, id), func(_ PositionedMutation) error {
					calls++
					if want := fmt.Sprintf("changed%d", calls); id.Name != want {
						t.Errorf("callback saw %q, want %q", id.Name, want)
					}
					switch outcome {
					case "error":
						return sentinel
					case "cancel":
						cancel()
					case "panic":
						panic("callback panic")
					}
					return nil
				})
			}()
			if id.Name != "original" {
				t.Errorf("source not restored: %s", id.Name)
			}
			wantCalls := 1
			if outcome == "ok" {
				wantCalls = 2
			}
			if calls != wantCalls {
				t.Errorf("callbacks = %d, want %d", calls, wantCalls)
			}
			switch outcome {
			case "ok":
				if err != nil || recovered != nil {
					t.Errorf("unexpected error/panic: %v / %v", err, recovered)
				}
			case "error":
				if !errors.Is(err, sentinel) || recovered != nil {
					t.Errorf("error/panic = %v / %v", err, recovered)
				}
			case "cancel":
				if !errors.Is(err, context.Canceled) || recovered != nil {
					t.Errorf("error/panic = %v / %v", err, recovered)
				}
			case "panic":
				if recovered != "callback panic" {
					t.Errorf("panic = %v", recovered)
				}
			}
		})
	}
}

func TestWalkHandshakeProbe(t *testing.T) {
	for _, positioned := range []bool{false, true} {
		t.Run(fmt.Sprint(positioned), func(t *testing.T) {
			id := &ast.Ident{Name: "original"}
			// Drive both protocol phases from the caller; each receive synchronizes
			// observation of syntax with the walk goroutine.
			if positioned {
				ch := MutateWalkWithPositions(nil, nil, id, probeMutator(t, id))
				for i := 1; i <= 2; i++ {
					v := receiveProbe(t, ch)
					if v.Node != id || id.Name != fmt.Sprintf("changed%d", i) {
						t.Errorf("applied phase: %#v %s", v, id.Name)
					}
					ch <- PositionedMutation{}
					v = receiveProbe(t, ch)
					if v.Node != nil || id.Name != "original" {
						t.Errorf("reset phase: %#v %s", v, id.Name)
					}
					ch <- PositionedMutation{}
				}
				if _, ok := <-ch; ok {
					t.Error("channel remained open")
				}
			} else {
				ch := MutateWalk(nil, nil, id, probeMutator(t, id))
				for i := 1; i <= 2; i++ {
					if !receiveProbe(t, ch) || id.Name != fmt.Sprintf("changed%d", i) {
						t.Errorf("applied phase: %s", id.Name)
					}
					ch <- false
					if !receiveProbe(t, ch) || id.Name != "original" {
						t.Errorf("reset phase: %s", id.Name)
					}
					ch <- false
				}
				if _, ok := <-ch; ok {
					t.Error("channel remained open")
				}
			}
		})
	}
}

func receiveProbe[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v, ok := <-ch:
		if !ok {
			t.Fatal("channel closed early")
		}
		return v
	case <-time.After(3 * time.Second):
		t.Fatal("handshake timed out")
		var zero T
		return zero
	}
}
