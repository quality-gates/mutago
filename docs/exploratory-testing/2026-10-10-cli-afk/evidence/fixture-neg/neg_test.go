package neg

import "testing"

func TestAbs(t *testing.T) {
	if Abs(-3) != 3 {
		t.Fatal("Abs(-3)")
	}
}
