package shop

import "testing"

func TestDiscount(t *testing.T) {
	if got := Discount(200, 10); got != 180 {
		t.Fatalf("got %d", got)
	}
}

func TestShipping(t *testing.T) {
	if Shipping(10) != 5 {
		t.Fatal("small order should pay shipping")
	}
	if Shipping(100) != 0 {
		t.Fatal("big order ships free")
	}
}

func TestClamp(t *testing.T) {
	if Clamp(5, 0, 10) != 5 {
		t.Fatal("in range")
	}
}

func TestTax(t *testing.T) {
	if Tax(100) != 20 {
		t.Fatal("tax")
	}
}

func TestClampEdges(t *testing.T) {
	if Clamp(-1, 0, 10) != 0 || Clamp(11, 0, 10) != 10 || Clamp(0, 0, 10) != 0 || Clamp(10, 0, 10) != 10 {
		t.Fatal("edges")
	}
}
