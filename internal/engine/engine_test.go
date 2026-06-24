package engine

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

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
		absFile: "/repo/fixture.go",
		opts:    &models.Options{},
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
