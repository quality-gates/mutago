package ship

import "testing"

func TestFee(t *testing.T) {
	if Fee(10) != 5 || Fee(100) != 0 {
		t.Fatal("fee")
	}
}
