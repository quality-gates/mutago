package pertestdir

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestDoubleRuns covers Double without checking its result, so it kills no
// mutant.
func TestDoubleRuns(t *testing.T) {
	_ = Double(1)
}

// TestDouble reads its cases by a path relative to the package directory,
// where go test runs it with a matching PWD.
func TestDouble(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if pwd := os.Getenv("PWD"); pwd != dir {
		t.Fatalf("PWD = %q, want %q", pwd, dir)
	}
	data, err := os.ReadFile("testdata/cases.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var in, want int
		if _, err := fmt.Sscan(line, &in, &want); err != nil {
			t.Fatal(err)
		}
		if got := Double(in); got != want {
			t.Errorf("Double(%d) = %d, want %d", in, got, want)
		}
	}
}
