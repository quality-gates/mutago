package baseline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/quality-gates/mutago/v2/internal/models"
)

const storeDiff = "--- Original\n+++ New\n@@ -1,1 +1,1 @@\n-\treturn a + b\n+\treturn a - b\n"

func storeMutant(path string) models.Mutant {
	var m models.Mutant
	m.Mutator.OriginalFilePath = path
	m.Mutator.MutatorName = "arithmetic/base"
	m.Diff = storeDiff
	return m
}

// newModule creates a module root with internal/store/store.go and returns
// its absolute, symlink-free path.
func newModule(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "store"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestNewEscapes_MatchesOnlyRecordedIDs(t *testing.T) {
	first := storeMutant("internal/store/store.go")
	first.ID = MutantIDAt("internal/store/store.go", "arithmetic/base", storeDiff, 0)
	second := storeMutant("internal/store/store.go")
	second.ID = MutantIDAt("internal/store/store.go", "arithmetic/base", storeDiff, 1)
	accepted := &File{Version: 1, Mutants: []Entry{{ID: first.ID, File: "internal/store/store.go"}}}

	got := accepted.NewEscapes([]models.Mutant{first, second})
	if len(got) != 1 || got[0].ID != second.ID {
		t.Fatalf("want only the second same-text mutant as a new escape, got %+v", got)
	}
}

func TestMutantIDAt_OccurrenceZeroKeepsMutantID(t *testing.T) {
	id0 := MutantIDAt("internal/store/store.go", "arithmetic/base", storeDiff, 0)
	if id0 != MutantID("internal/store/store.go", "arithmetic/base", storeDiff) {
		t.Fatal("occurrence 0 must keep the MutantID hash so existing baselines stay valid")
	}
	if id1 := MutantIDAt("internal/store/store.go", "arithmetic/base", storeDiff, 1); id1 == id0 {
		t.Fatal("occurrence 1 must not share occurrence 0's ID")
	}
}

func TestMutantID_CanonicalPathIDUnchanged(t *testing.T) {
	// Pinned from a pre-#248 build: IDs for canonical paths must not change.
	const want = "6f89bf4c68aebd57452c37c0583e9a02"
	if got := MutantID("internal/store/store.go", "arithmetic/base", storeDiff); got != want {
		t.Fatalf("MutantID changed for a canonical path: got %s, want %s", got, want)
	}
}

func TestRelPath_CanonicalisesTargetSpellings(t *testing.T) {
	root := newModule(t)
	t.Chdir(filepath.Join(root, "internal"))

	for _, path := range []string{
		"./store/store.go",
		"store/store.go",
		"../internal/store/./store.go",
		filepath.Join(root, "internal", "store", "store.go"),
	} {
		if got := RelPath(path, root); got != "internal/store/store.go" {
			t.Errorf("RelPath(%q) = %q, want %q", path, got, "internal/store/store.go")
		}
	}
}

func TestRelPath_WithoutModuleRootKeepsPath(t *testing.T) {
	if got := RelPath("./store/store.go", ""); got != "store/store.go" {
		t.Fatalf("RelPath without module root = %q, want %q", got, "store/store.go")
	}
}
