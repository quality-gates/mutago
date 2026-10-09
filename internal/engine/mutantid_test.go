package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quality-gates/mutago/v2"
	"github.com/quality-gates/mutago/v2/internal/baseline"
	"github.com/quality-gates/mutago/v2/internal/parser"
	"github.com/quality-gates/mutago/v2/mutator"
)

func TestMutantIDForEditMatchesExternalDiff(t *testing.T) {
	parser.ClearPackageCache()
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "calc.go")
	src := []byte("package calc\n\nfunc Add(a, b int) int { return a + b }\n\nfunc Guard(n int) int {\n\tif n > 0 {\n\t\treturn n\n\t}\n\treturn 0\n}\n\nfunc Loop(n int) int {\n\ts := 0\n\tfor i := 0; i < n; i++ {\n\t\ts = s + i\n\t}\n\treturn s\n}\n")
	if err := os.WriteFile(srcPath, src, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/idmatch\n\ngo 1.26.6\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	checked, err := parser.ParseAndTypeCheckFile(srcPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	compared := 0
	for _, name := range mutator.List() {
		m, err := mutator.New(name)
		if err != nil {
			t.Fatal(err)
		}
		compared += compareMutatorIDs(t, name, m, checked, srcPath, src)
	}
	if compared == 0 {
		t.Fatal("expected mutators to produce edits")
	}
}

func compareMutatorIDs(t *testing.T, name string, m mutator.Mutator, checked *parser.CheckedFile, srcPath string, src []byte) int {
	t.Helper()
	compared := 0
	ch := mutago.MutateWalkWithPositions(checked.Pkg, checked.Info, checked.File, m)
	for {
		mutation, ok := <-ch
		if !ok {
			return compared
		}
		edit, err := captureMutationEdit(checked.Fset, mutation.Node, mutation.Start, mutation.End, src)
		if err != nil {
			t.Fatalf("%s: capture: %v", name, err)
		}
		mutated, err := edit.materialize(src)
		if err != nil {
			t.Fatalf("%s: materialize: %v", name, err)
		}
		got := baseline.MutantID("calc.go", name, formatChangedLines(changedLines(src, mutated)))
		if got != externalDiffID(t, "calc.go", name, srcPath, mutated) {
			t.Errorf("%s: in-memory id %s does not match diff -u", name, got)
		}
		compared++
		ch <- mutago.PositionedMutation{}
		<-ch
		ch <- mutago.PositionedMutation{}
	}
}

func externalDiffID(t *testing.T, relFile, name, originalPath string, mutated []byte) string {
	t.Helper()
	f, err := os.CreateTemp("", "mutago-id-*.go")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(mutated); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	out, _ := exec.Command("diff", "--label=Original", "--label=New", "-u", originalPath, f.Name()).CombinedOutput()
	return baseline.MutantID(relFile, name, string(out))
}

// identitiesInSourceOrder returns the arithmetic/base IDs of src in walk order.
func identitiesInSourceOrder(t *testing.T, src string) []string {
	t.Helper()
	parser.ClearPackageCache()
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "calc.go")
	if err := os.WriteFile(srcPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/identity\n\ngo 1.26.6\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	checked, err := parser.ParseAndTypeCheckFile(srcPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := mutator.New("arithmetic/base")
	if err != nil {
		t.Fatal(err)
	}
	ids := mutantIdentities(checked.Pkg, checked.Info, checked.Fset, checked.File, []byte(src), "calc.go", "arithmetic/base", m)
	var ordered []string
	ch := mutago.MutateWalkWithPositions(checked.Pkg, checked.Info, checked.File, m)
	for mutation := range ch {
		edit, err := captureMutationEdit(checked.Fset, mutation.Node, mutation.Start, mutation.End, []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		ordered = append(ordered, ids[edit.key()])
		ch <- mutago.PositionedMutation{}
		<-ch
		ch <- mutago.PositionedMutation{}
	}
	return ordered
}

const sameTextSrc = "package calc\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\nfunc Sum(a, b int) int {\n\treturn a + b\n}\n"

func TestMutantIdentitiesSeparateSameTextEdits(t *testing.T) {
	ids := identitiesInSourceOrder(t, sameTextSrc)
	if len(ids) != 2 {
		t.Fatalf("want 2 edits, got %d", len(ids))
	}
	if ids[0] == "" || ids[0] == ids[1] {
		t.Fatalf("same-text edits at different offsets need distinct IDs, got %q", ids)
	}
	want0 := baseline.MutantID("calc.go", "arithmetic/base", "-\treturn a + b\n+\treturn a - b\n")
	if ids[0] != want0 {
		t.Fatalf("occurrence 0 must keep the pre-#274 MutantID: got %s, want %s", ids[0], want0)
	}
}

func TestMutantIdentitiesSurviveLineShifts(t *testing.T) {
	before := identitiesInSourceOrder(t, sameTextSrc)
	shifted := strings.Replace(sameTextSrc, "package calc\n", "package calc\n\n// Calc adds.\nvar _ = 0\n", 1)
	after := identitiesInSourceOrder(t, shifted)
	if len(after) != 2 || after[0] != before[0] || after[1] != before[1] {
		t.Fatalf("inserting lines above must keep IDs: before %q, after %q", before, after)
	}
}
