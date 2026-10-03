package ann

import "testing"

func TestCap(t *testing.T) {
	if Cap(5) != 5 {
		t.Fatal("cap")
	}
}

func TestBump(t *testing.T) {
	if Bump(-1) != 0 {
		t.Fatal("bump")
	}
}
