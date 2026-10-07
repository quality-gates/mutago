package engine

import (
	"os"
	"os/exec"
	"path/filepath"
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
		got, err := mutantIDForEdit("calc.go", name, src, edit)
		if err != nil {
			t.Fatalf("%s: id: %v", name, err)
		}
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
