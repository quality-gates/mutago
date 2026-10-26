package engine

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/quality-gates/mutago/v2/internal/importing"
	"github.com/quality-gates/mutago/v2/internal/models"
)

const prepCancelSrc = "package prepcancel\n\nfunc Add(a, b int) int { return a + b }\n"

// prepCancelTestSrc blocks each test of the stage named by MUTAGO_PREP_STAGE
// until its marker directory is removed. The baseline stage runs without
// coverage; the coverage and per-test profiling runs with it. Each test that
// reaches the blocking stage writes its readiness marker first.
const prepCancelTestSrc = `package prepcancel

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBlock(t *testing.T) {
	if Add(1, 2) != 3 {
		t.Fatal("Add")
	}
	block("block")
}

func TestAfter(t *testing.T) { block("after") }

func block(name string) {
	stage := os.Getenv("MUTAGO_PREP_STAGE")
	if stage == "" || (stage == "baseline") == (testing.CoverMode() != "") {
		return
	}
	dir := os.Getenv("MUTAGO_PREP_MARKERS")
	_ = os.WriteFile(filepath.Join(dir, name), nil, 0o644)
	for i := 0; i < 600; i++ {
		if _, err := os.Stat(dir); err != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}
`

// TestEngineCancelDuringPreparation covers #305: cancelling the run while a
// preparation command runs stops it promptly with a tool error, starts no
// further work, and still cleans up (or keeps) the run's temp directory.
func TestEngineCancelDuringPreparation(t *testing.T) {
	tests := []struct {
		name   string
		stage  string
		retain bool
		setup  func(*models.Options)
	}{
		{name: "baseline", stage: "baseline", setup: func(*models.Options) {}},
		{name: "coverage", stage: "covered", setup: func(o *models.Options) { o.Exec.Coverage = true }},
		{name: "per-test profiling", stage: "covered", setup: func(o *models.Options) { o.Exec.PerTest = true }},
		{name: "per-test profiling retains tmp", stage: "covered", retain: true, setup: func(o *models.Options) { o.Exec.PerTest = true }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := prepCancelOptions(t)
			tt.setup(opts)
			opts.General.DoNotRemoveTmpFolder = tt.retain
			markers := t.TempDir()
			runTmp := t.TempDir()
			t.Setenv("TMPDIR", runTmp)
			t.Setenv("MUTAGO_PREP_STAGE", tt.stage)
			t.Setenv("MUTAGO_PREP_MARKERS", markers)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cancelled := make(chan time.Time, 1)
			go func() {
				for ctx.Err() == nil {
					if _, err := os.Stat(filepath.Join(markers, "block")); err == nil {
						cancelled <- time.Now()
						cancel()
						return
					}
					time.Sleep(10 * time.Millisecond)
				}
			}()

			var stdout, stderr bytes.Buffer
			res, err := (&Engine{Stdout: &stdout, Stderr: &stderr}).Run(ctx, opts, nil)
			returned := time.Now()

			var cancelledAt time.Time
			select {
			case cancelledAt = <-cancelled:
			default:
				t.Fatalf("the %s stage was never reached; err=%v stdout=%q stderr=%q", tt.stage, err, stdout.String(), stderr.String())
			}
			if wait := returned.Sub(cancelledAt); wait > 10*time.Second {
				t.Errorf("run returned %s after cancellation", wait)
			}
			assertCancelledRun(t, res, err, stderr.String())
			if _, err := os.Stat(filepath.Join(markers, "after")); err == nil {
				t.Error("a test started after cancellation")
			}
			left, _ := filepath.Glob(filepath.Join(runTmp, "mutago-*"))
			if want := map[bool]int{true: 1, false: 0}[tt.retain]; len(left) != want {
				t.Errorf("run tmp dirs = %q, want %d", left, want)
			}
		})
	}
}

// assertCancelledRun checks that a run ended by cancellation reports it as a
// tool error and ran no mutant.
func assertCancelledRun(t *testing.T, res Result, err error, stderr string) {
	t.Helper()
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled; stderr=%q", err, stderr)
	}
	if strings.Contains(stderr, "green baseline") {
		t.Errorf("cancellation reported as a failing baseline: %q", stderr)
	}
	if res.ExitCode != returnError {
		t.Errorf("exit code = %d, want %d", res.ExitCode, returnError)
	}
	if res.Report != nil && res.Report.Stats.KilledCount+res.Report.Stats.EscapedCount != 0 {
		t.Errorf("mutants ran after cancellation: %+v", res.Report.Stats)
	}
}

func prepCancelOptions(t *testing.T) *models.Options {
	t.Helper()
	_ = os.MkdirAll("./testdata", 0755)
	tempDir, err := os.MkdirTemp("./testdata", "prepcancel-*")
	if err != nil {
		t.Fatalf("failed to create temp package: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })
	if err := os.WriteFile(filepath.Join(tempDir, "pkg.go"), []byte(prepCancelSrc), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "pkg_test.go"), []byte(prepCancelTestSrc), 0644); err != nil {
		t.Fatal(err)
	}

	opts := &models.Options{}
	opts.Exec.Timeout = 60
	opts.Exec.TestFlags = "-count=1"
	opts.General.Workers = 1
	opts.Mutator.DisableMutators = []string{
		"branch/*", "composite/*", "concurrency/*", "conditional/*",
		"expression/*", "loop/*", "numbers/*", "select/*", "statement/*",
	}
	opts.Remaining.Targets = []string{"./" + tempDir}
	return opts
}

// A cancelled run is an error from per-test profiling, not a missing profile
// that would send mutants to the full suite.
func TestBuildPerTestCoverageProfileReportsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	opts := &models.Options{}
	opts.Exec.Timeout = 60

	prof, err := buildPerTestCoverageProfile(ctx, io.Discard, opts, "github.com/quality-gates/mutago/v2/internal/engine", "github.com/quality-gates/mutago/v2", t.TempDir(), execConfig{numWorkers: 1})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if prof != nil {
		t.Error("a cancelled run returned a profile")
	}
}

// A package left unresolved because the run was cancelled reports the
// cancellation, not a package mutago cannot test.
func TestBaselineChecksReportCancellationBeforeResolving(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	opts := &models.Options{}
	opts.Exec.Timeout = 60
	pkgs := []importing.Package{{Files: []string{"gotest.go"}}}
	runner, _ := resolveRunner(opts, modeExecute)
	ec := execConfig{mode: modeExecute, runner: runner, importPaths: newImportPaths(ctx, "", nil)}
	var stderr bytes.Buffer

	code := runBaselineChecks(ctx, &stderr, opts, pkgs, ec)

	if code != returnError {
		t.Errorf("exit code = %d, want %d", code, returnError)
	}
	if !strings.Contains(stderr.String(), "cancelled") || strings.Contains(stderr.String(), "Cannot resolve") {
		t.Errorf("stderr = %q, want a cancellation", stderr.String())
	}
}
