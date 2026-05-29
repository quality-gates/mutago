package genericsexample

import "testing"

func TestWrap(t *testing.T) {
	a := &A{Val: 1}
	if Wrap(a) == nil {
		t.Fatal("expected non-nil result")
	}
}
