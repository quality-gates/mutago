package covflags

import "testing"

func TestAdd(t *testing.T) {
	// Simulates tests that need -short (e.g. skip live GCP calls).
	if !testing.Short() {
		t.Fatal("requires credentials (simulated by missing -short)")
	}
	if got := Add(1, 2); got != 3 {
		t.Fatalf("Add(1,2)=%d", got)
	}
}
