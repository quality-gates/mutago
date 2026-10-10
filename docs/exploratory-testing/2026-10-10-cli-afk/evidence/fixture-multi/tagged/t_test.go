//go:build integration

package tagged

import "testing"

func TestDouble(t *testing.T) {
	if Double(3) != 6 {
		t.Fatal("double")
	}
}
