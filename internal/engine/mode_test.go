package engine

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestEngineNoExecSkipsQualityGates(t *testing.T) {
	opts := escapingPackageOptions(t)
	opts.Exec.NoExec = true
	opts.Score.MinMsi = 101
	opts.Score.FailOnEscaped = true

	var stdout, stderr bytes.Buffer
	e := &Engine{Stdout: &stdout, Stderr: &stderr}
	res, err := e.Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ExitCode != returnOk {
		t.Errorf("expected exit code %d, got %d; stderr: %s", returnOk, res.ExitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Cannot do a mutation testing summary") {
		t.Errorf("expected generate-only notice on stdout, got %q", stdout.String())
	}
	if strings.Contains(stderr.String(), "below minimum required") {
		t.Errorf("expected no quality gate to run, got stderr %q", stderr.String())
	}
}

func TestEngineNoExecLeavesBaselineUntouched(t *testing.T) {
	opts := escapingPackageOptions(t)
	opts.Exec.NoExec = true
	opts.Baseline.Update = true
	opts.Baseline.File = filepath.Join(t.TempDir(), "baseline.json")
	original := []byte(`{"version":1,"mutants":[{"file":"pkg.go","mutator":"arithmetic/base","checksum":"abc"}]}`)
	if err := os.WriteFile(opts.Baseline.File, original, 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	e := &Engine{Stdout: &stdout, Stderr: &stderr}
	res, err := e.Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ExitCode != returnOk {
		t.Errorf("expected exit code %d, got %d", returnOk, res.ExitCode)
	}
	got, err := os.ReadFile(opts.Baseline.File)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("expected baseline unchanged, got %s", got)
	}
}

func TestEngineNoExecWritesOneMutatedFilePerAdmittedMutant(t *testing.T) {
	opts := escapingPackageOptions(t)
	opts.General.DryRun = true
	e := &Engine{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	dry, err := e.Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	want := int(dry.Report.Stats.TotalMutantsCount)
	if want == 0 {
		t.Fatal("expected the dry run to admit mutants")
	}

	tmpRoot := t.TempDir()
	t.Setenv("TMPDIR", tmpRoot)
	opts.General.DryRun = false
	opts.Exec.NoExec = true
	opts.General.DoNotRemoveTmpFolder = true
	res, err := e.Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ExitCode != returnOk {
		t.Fatalf("expected exit code %d, got %d", returnOk, res.ExitCode)
	}

	mutated := regexp.MustCompile(`pkg\.go\.\d+$`)
	var originals, mutants []string
	_ = filepath.WalkDir(tmpRoot, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil || d.IsDir():
		case strings.HasSuffix(path, "pkg.go.original"):
			originals = append(originals, path)
		case mutated.MatchString(path):
			mutants = append(mutants, path)
		}
		return nil
	})
	if len(originals) != 1 {
		t.Fatalf("expected one pkg.go.original, got %v", originals)
	}
	if len(mutants) != want {
		t.Fatalf("expected %d mutated files (dry-run count), got %d: %v", want, len(mutants), mutants)
	}
	original, _ := os.ReadFile(originals[0])
	for _, path := range mutants {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(got, original) {
			t.Errorf("expected %s to differ from the original", path)
		}
	}
}
