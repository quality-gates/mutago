package engine

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/quality-gates/mutago/v2/internal/gitdiff"
	"github.com/quality-gates/mutago/v2/internal/models"

	// Register mutators for tests
	_ "github.com/quality-gates/mutago/v2/mutator/arithmetic"
	_ "github.com/quality-gates/mutago/v2/mutator/branch"
	_ "github.com/quality-gates/mutago/v2/mutator/composite"
	_ "github.com/quality-gates/mutago/v2/mutator/concurrency"
	_ "github.com/quality-gates/mutago/v2/mutator/conditional"
	_ "github.com/quality-gates/mutago/v2/mutator/expression"
	_ "github.com/quality-gates/mutago/v2/mutator/loop"
	_ "github.com/quality-gates/mutago/v2/mutator/numbers"
	_ "github.com/quality-gates/mutago/v2/mutator/select"
	_ "github.com/quality-gates/mutago/v2/mutator/statement"
)

func TestRunCustomExecHonorsCancelledContext(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original.go")
	mutated := filepath.Join(dir, "mutated.go")
	if err := os.WriteFile(original, []byte("package sample\nvar n = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mutated, []byte("package sample\nvar n = 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	opts := &models.Options{}
	opts.Exec.Timeout = 30
	job := execJob{
		ctx:   ctx,
		opts:  opts,
		pkg:   types.NewPackage("sample", "sample"),
		execs: []string{"sh", "-c", "sleep 10"},
		source: mutationSource{
			originalFile: original,
			mutationFile: mutated,
		},
	}
	started := time.Now()
	code := runCustomExec(job, &models.Mutant{})
	if code != 3 {
		t.Fatalf("expected cancelled command to return tool error 3, got %d", code)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("cancelled command took %s", elapsed)
	}
}

func TestMutationEditMaterializesChangedNode(t *testing.T) {
	source := []byte("package sample\nvar value = 1\n")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "sample.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	var literal *ast.BasicLit
	ast.Inspect(file, func(node ast.Node) bool {
		if candidate, ok := node.(*ast.BasicLit); ok {
			literal = candidate
		}
		return true
	})
	literal.Value = "2"

	start, end := literal.Pos(), literal.End()
	edit, err := captureMutationEdit(fset, literal, start, end, source)
	if err != nil {
		t.Fatal(err)
	}
	got, err := edit.materialize(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "package sample\nvar value = 2\n" {
		t.Fatalf("unexpected mutation: %q", got)
	}
}

func TestEngineDryRun(t *testing.T) {
	opts := &models.Options{}
	opts.General.DryRun = true
	opts.Remaining.Targets = []string{"../../example"}

	var stdout, stderr bytes.Buffer
	e := &Engine{
		Stdout: &stdout,
		Stderr: &stderr,
	}

	res, err := e.Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", res.ExitCode)
	}

	if res.Report == nil {
		t.Fatal("expected report to be populated, got nil")
	}

	// Verify that the engine processed the example package and counted mutants.
	if res.Report.Stats.TotalMutantsCount <= 0 {
		t.Errorf("expected total mutants counted in dry run to be > 0, got %d", res.Report.Stats.TotalMutantsCount)
	}
}

func TestEngineNoopFail(t *testing.T) {
	// Create testdata dir if not exists
	_ = os.MkdirAll("./testdata", 0755)

	// Create temporary package directory inside testdata
	tempDir, err := os.MkdirTemp("./testdata", "noop-fail-*")
	if err != nil {
		t.Fatalf("failed to create temp package: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dummyGo := `package noopfail
`
	dummyTestGo := `package noopfail
import "testing"
func TestShouldFail(t *testing.T) {
	t.Fatal("failing on purpose")
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "dummy.go"), []byte(dummyGo), 0644); err != nil {
		t.Fatalf("failed to write dummy.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "dummy_test.go"), []byte(dummyTestGo), 0644); err != nil {
		t.Fatalf("failed to write dummy_test.go: %v", err)
	}

	opts := &models.Options{}
	opts.General.Noop = true
	// Target the newly created temporary package
	opts.Remaining.Targets = []string{"./" + tempDir}

	var stdout, stderr bytes.Buffer
	e := &Engine{
		Stdout: &stdout,
		Stderr: &stderr,
	}

	res, err := e.Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The engine must return exit code 3 (returnError) when NOOP suite fails
	if res.ExitCode != 3 {
		t.Errorf("expected exit code 3 for failing NOOP check, got %d", res.ExitCode)
	}
}

func TestSkipForGitDiffUsesOriginalASTLine(t *testing.T) {
	job := execJob{
		opts: &models.Options{},
		source: mutationSource{
			absFile: "/repo/fixture.go",
		},
	}
	gitChangedLines := gitdiff.ChangedLines{
		"fixture.go": {{Start: 4, End: 4}},
	}
	job.mutant.Mutator.OriginalStartLine = 4
	if skipForGitDiff(job, gitChangedLines) {
		t.Error("expected skipForGitDiff to be false, got true")
	}

	job.mutant.Mutator.OriginalStartLine = 3
	if !skipForGitDiff(job, gitChangedLines) {
		t.Error("expected skipForGitDiff to be true, got false")
	}
}

// TestEngineCoverageHonorsTestFlags ensures --test-flags reaches the initial
// coverage collection step. Without -short, testdata/covflags fails (simulating
// missing credentials) and mutants are incorrectly marked NOT COVERED.
func TestEngineCoverageHonorsTestFlags(t *testing.T) {
	opts := &models.Options{}
	opts.Exec.Coverage = true
	opts.Exec.Timeout = 30
	opts.Exec.TestFlags = "-short"
	opts.Remaining.Targets = []string{"./testdata/covflags"}

	var stdout, stderr bytes.Buffer
	e := &Engine{
		Stdout: &stdout,
		Stderr: &stderr,
	}

	res, err := e.Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Report == nil {
		t.Fatal("expected report to be populated, got nil")
	}
	if res.Report.Stats.TotalMutantsCount == 0 {
		t.Fatal("expected at least one mutant in covflags fixture")
	}

	// Symptom of the bug: coverage collection omitted -short, so the profile
	// has 0% coverage and every mutant is marked NOT COVERED.
	if res.Report.Stats.NotCoveredCount == res.Report.Stats.TotalMutantsCount {
		t.Fatalf("all %d mutants marked NOT COVERED despite --test-flags=-short; stdout=%q stderr=%q",
			res.Report.Stats.TotalMutantsCount, stdout.String(), stderr.String())
	}
	if res.Report.Stats.KilledCount == 0 {
		t.Fatalf("expected at least one killed mutant when coverage honors -short; notCovered=%d escaped=%d stdout=%q",
			res.Report.Stats.NotCoveredCount, res.Report.Stats.EscapedCount, stdout.String())
	}
}

// TestEngineCoverageSkipsExecForUncoveredMutants ensures --coverage does not run
// the exec command for mutants on uncovered lines.
func TestEngineCoverageSkipsExecForUncoveredMutants(t *testing.T) {
	_ = os.MkdirAll("./testdata", 0755)
	tempDir, err := os.MkdirTemp("./testdata", "covskip-*")
	if err != nil {
		t.Fatalf("failed to create temp package: %v", err)
	}
	defer os.RemoveAll(tempDir)

	src := `package covskip

func Covered(a, b int) int { return a + b }

func Uncovered(a, b int) int { return a - b }
`
	testSrc := `package covskip

import "testing"

func TestCovered(t *testing.T) {
	if got := Covered(1, 2); got != 3 {
		t.Fatalf("Covered(1, 2) = %d", got)
	}
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "pkg.go"), []byte(src), 0644); err != nil {
		t.Fatalf("failed to write pkg.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "pkg_test.go"), []byte(testSrc), 0644); err != nil {
		t.Fatalf("failed to write pkg_test.go: %v", err)
	}

	logFile := filepath.Join(tempDir, "exec.log")
	script := filepath.Join(tempDir, "count.sh")
	if err := os.WriteFile(script, []byte("echo x >> \"$1\"\nexit 0\n"), 0644); err != nil {
		t.Fatalf("failed to write count.sh: %v", err)
	}

	opts := &models.Options{}
	opts.Exec.Coverage = true
	opts.Exec.Timeout = 10
	opts.Exec.Exec = "/bin/sh " + script + " " + logFile
	opts.Mutator.DisableMutators = []string{
		"branch/*", "composite/*", "concurrency/*", "conditional/*",
		"expression/*", "loop/*", "numbers/*", "select/*", "statement/*",
	}
	opts.Remaining.Targets = []string{"./" + tempDir}

	var stdout, stderr bytes.Buffer
	e := &Engine{
		Stdout: &stdout,
		Stderr: &stderr,
	}
	res, err := e.Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Report == nil {
		t.Fatal("expected report to be populated, got nil")
	}
	stats := res.Report.Stats
	if stats.NotCoveredCount == 0 {
		t.Fatalf("expected at least one not-covered mutant; stdout=%q stderr=%q",
			stdout.String(), stderr.String())
	}

	execCount := 0
	if data, readErr := os.ReadFile(logFile); readErr == nil {
		for _, line := range bytes.Split(data, []byte("\n")) {
			if len(line) > 0 {
				execCount++
			}
		}
	}
	scored := stats.KilledCount + stats.EscapedCount + stats.ErrorCount + stats.SkippedCount
	if int64(execCount) != scored {
		t.Fatalf("exec ran for not-covered mutants: exec=%d scored=%d notCovered=%d total=%d stdout=%q stderr=%q",
			execCount, scored, stats.NotCoveredCount, stats.TotalMutantsCount, stdout.String(), stderr.String())
	}
}
