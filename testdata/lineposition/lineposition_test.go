package lineposition

import "testing"

func TestFunctionsReturnInput(t *testing.T) {
	for _, fn := range []func(int) int{NearTop, BelowComment} {
		if got := fn(7); got != 7 {
			t.Fatalf("function returned %d, want 7", got)
		}
	}
}
