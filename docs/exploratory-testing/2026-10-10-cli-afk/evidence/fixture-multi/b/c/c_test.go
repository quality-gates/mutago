package c

import (
	"testing"

	"example.com/multi/b"
)

func TestMax(t *testing.T) {
	if b.Max(1, 2) != 2 || b.Max(3, 1) != 3 {
		t.Fatal("max")
	}
}
