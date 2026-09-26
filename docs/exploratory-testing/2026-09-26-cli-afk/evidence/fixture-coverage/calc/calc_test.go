package calc

import "testing"

func TestAdd(t *testing.T) {
	if got := Add(2, 3); got != 5 {
		t.Fatalf("Add(2, 3) = %d, want 5", got)
	}
	if got := Add(2, 0); got != 2 {
		t.Fatalf("Add(2, 0) = %d, want 2", got)
	}
}

func TestIsPositive(t *testing.T) {
	for _, test := range []struct {
		n    int
		want bool
	}{{1, true}, {0, false}, {-1, false}} {
		if got := IsPositive(test.n); got != test.want {
			t.Errorf("IsPositive(%d) = %t, want %t", test.n, got, test.want)
		}
	}
}

func TestMultiply(t *testing.T) {
	if got := Multiply(3, 4); got != 12 {
		t.Fatalf("Multiply(3, 4) = %d, want 12", got)
	}
}
