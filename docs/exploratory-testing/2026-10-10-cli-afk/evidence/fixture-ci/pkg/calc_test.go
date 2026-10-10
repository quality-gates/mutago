package pkg

import "testing"

func TestInc(t *testing.T) {
	if Inc(1) <= 1 {
		t.Fatal("inc")
	}
	_ = IncAgain(1)
}

func TestClamp(t *testing.T) {
	if Clamp(5, 0, 10) != 5 {
		t.Fatal("clamp")
	}
}
