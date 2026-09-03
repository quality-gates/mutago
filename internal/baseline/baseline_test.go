package baseline

import (
	"testing"
)

func TestMutantID_PreservesDecrementAndPreIncrement(t *testing.T) {
	diffA := "--- a/foo.go\n+++ b/foo.go\n@@ -1,1 +1,1 @@\n---x\n+0\n"
	diffB := "--- a/foo.go\n+++ b/foo.go\n@@ -1,1 +1,1 @@\n---y\n+0\n"

	idA := MutantID("foo.go", "decrementer", diffA)
	idB := MutantID("foo.go", "decrementer", diffB)

	if idA == idB {
		t.Fatalf("MutantID should differ when removing --x vs --y, got same hash %s", idA)
	}

	diffAddA := "--- a/foo.go\n+++ b/foo.go\n@@ -1,1 +1,1 @@\n-0\n+++x\n"
	diffAddB := "--- a/foo.go\n+++ b/foo.go\n@@ -1,1 +1,1 @@\n-0\n+++y\n"

	idAddA := MutantID("foo.go", "incrementer", diffAddA)
	idAddB := MutantID("foo.go", "incrementer", diffAddB)

	if idAddA == idAddB {
		t.Fatalf("MutantID should differ when adding ++x vs ++y, got same hash %s", idAddA)
	}
}
