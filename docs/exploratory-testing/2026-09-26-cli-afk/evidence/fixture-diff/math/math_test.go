package math

import "testing"

func TestCalculate(t *testing.T) {
	if got := Calculate(5); got != 7 {
		t.Fatalf("Calculate(5) = %d, want 7", got)
	}
}
