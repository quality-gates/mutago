package cgofixture

import "testing"

func TestValue(t *testing.T) {
	if got := Value(); got != 1 {
		t.Fatalf("Value() = %d, want 1", got)
	}
}
