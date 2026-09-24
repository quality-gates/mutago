package engine

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/quality-gates/mutago/v2"
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
	var stderr bytes.Buffer
	job := execJob{
		ctx:   ctx,
		opts:  opts,
		pkg:   types.NewPackage("sample", "sample"),
		execs: []string{"sh", "-c", "sleep 10"},
		out:   jobOutput{stderr: &stderr},
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
	if !strings.Contains(stderr.String(), "custom exec failed to start") {
		t.Errorf("expected cancellation message on job stderr, got %q", stderr.String())
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

func TestStableMutationEditKeyIsPositionAware(t *testing.T) {
	source := []byte("package sample\n\nfunc a() int { return 0 }\n\nfunc b() int { return 0 }\n")
	editA := mutationEdit{start: 32, end: 33, replacement: []byte("1")}
	editB := mutationEdit{start: 61, end: 62, replacement: []byte("1")}

	keyA := stableMutationEditKey("sample.go", source, editA)
	keyB := stableMutationEditKey("sample.go", source, editB)
	if keyA == keyB {
		t.Fatal("expected identical text at different offsets to produce different dedup keys")
	}
	if again := stableMutationEditKey("sample.go", source, editA); again != keyA {
		t.Fatal("expected the same edit to produce the same dedup key")
	}
	if keyA != stableMutationEditKey("sample.go", source, mutationEdit{start: 32, end: 33, replacement: []byte("1")}) {
		t.Fatal("expected an equal edit at the same offset to produce an equal dedup key")
	}
	if keyA == stableMutationEditKey("other.go", source, editA) {
		t.Fatal("expected the same edit in a different file to produce a different dedup key")
	}
	if len(keyA) != 32 {
		t.Fatalf("expected 32-char MD5 hex key for blacklist compatibility, got %d chars", len(keyA))
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

// escapingPackageOptions writes a package whose test never checks results, so
// every arithmetic mutant escapes, and returns options targeting it.
func escapingPackageOptions(t *testing.T) *models.Options {
	t.Helper()
	_ = os.MkdirAll("./testdata", 0755)
	tempDir, err := os.MkdirTemp("./testdata", "escaping-*")
	if err != nil {
		t.Fatalf("failed to create temp package: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	src := "package escaping\n\nfunc Add(a, b int) int { return a + b }\n"
	testSrc := "package escaping\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) { Add(1, 2) }\n"
	if err := os.WriteFile(filepath.Join(tempDir, "pkg.go"), []byte(src), 0644); err != nil {
		t.Fatalf("failed to write pkg.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "pkg_test.go"), []byte(testSrc), 0644); err != nil {
		t.Fatalf("failed to write pkg_test.go: %v", err)
	}

	opts := &models.Options{}
	opts.Exec.Timeout = 30
	opts.Mutator.DisableMutators = []string{
		"branch/*", "composite/*", "concurrency/*", "conditional/*",
		"expression/*", "loop/*", "numbers/*", "select/*", "statement/*",
	}
	opts.Remaining.Targets = []string{"./" + tempDir}
	return opts
}

func TestEngineQualityGateMessagesUseInjectedStderr(t *testing.T) {
	opts := escapingPackageOptions(t)
	opts.Score.FailOnEscaped = true
	opts.Score.MinMsi = 101
	opts.Score.MinCoveredMsi = 50

	var stdout, stderr bytes.Buffer
	e := &Engine{Stdout: &stdout, Stderr: &stderr}

	res, err := e.Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ExitCode != returnMsiThresholdNotMet {
		t.Errorf("expected exit code %d, got %d", returnMsiThresholdNotMet, res.ExitCode)
	}
	for _, want := range []string{
		"mutant(s) escaped",
		"is below minimum required 101.00%",
		"Covered MSI cannot be checked",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("expected %q on injected stderr, got %q", want, stderr.String())
		}
	}
}

func TestEngineGitHubAnnotationsUseInjectedStdout(t *testing.T) {
	opts := escapingPackageOptions(t)
	opts.Logger.GitHub = true

	var stdout, stderr bytes.Buffer
	e := &Engine{Stdout: &stdout, Stderr: &stderr}

	if _, err := e.Run(context.Background(), opts, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "::warning file=") {
		t.Errorf("expected GitHub annotation on injected stdout, got %q", stdout.String())
	}
}

func TestEngineDebugGoTestOutputUsesInjectedStdout(t *testing.T) {
	opts := escapingPackageOptions(t)
	opts.General.Debug = true

	var stdout, stderr bytes.Buffer
	e := &Engine{Stdout: &stdout, Stderr: &stderr}

	if _, err := e.Run(context.Background(), opts, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "ok  \t") {
		t.Errorf("expected mutant go test output on injected stdout, got %q", stdout.String())
	}
}

func TestEngineCustomExecOutputUsesInjectedWriters(t *testing.T) {
	opts := escapingPackageOptions(t)
	opts.General.Verbose = true
	script := filepath.Join(t.TempDir(), "exec.sh")
	if err := os.WriteFile(script, []byte("echo exec-stdout-marker\necho exec-stderr-marker >&2\nexit 2\n"), 0644); err != nil {
		t.Fatalf("failed to write exec.sh: %v", err)
	}
	opts.Exec.Exec = "/bin/sh " + script

	var stdout, stderr bytes.Buffer
	e := &Engine{Stdout: &stdout, Stderr: &stderr}

	if _, err := e.Run(context.Background(), opts, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"exec-stdout-marker", "Mutation did not compile"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("expected %q on injected stdout, got %q", want, stdout.String())
		}
	}
	if !strings.Contains(stderr.String(), "exec-stderr-marker") {
		t.Errorf("expected exec stderr on injected stderr, got %q", stderr.String())
	}
}

func TestEnginePerTestProgressUsesInjectedStdout(t *testing.T) {
	opts := escapingPackageOptions(t)
	opts.Exec.PerTest = true

	var stdout, stderr bytes.Buffer
	e := &Engine{Stdout: &stdout, Stderr: &stderr}

	if _, err := e.Run(context.Background(), opts, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Building per-test coverage map") {
		t.Errorf("expected per-test progress on injected stdout, got %q", stdout.String())
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
	if !strings.Contains(stderr.String(), "Baseline test failed") {
		t.Errorf("expected baseline failure on injected stderr, got %q", stderr.String())
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

func TestIsGitDiffSkipped(t *testing.T) {
	// When gitChangedLines is nil, nothing is skipped.
	if isGitDiffSkipped(nil, "foo.go", "/repo/foo.go", 10) {
		t.Error("expected isGitDiffSkipped to be false when gitChangedLines is nil")
	}

	gitChangedLines := gitdiff.ChangedLines{
		"pkg/foo.go": {{Start: 10, End: 15}},
	}

	// Line within range for relative path: not skipped.
	if isGitDiffSkipped(gitChangedLines, "pkg/foo.go", "/repo/pkg/foo.go", 12) {
		t.Error("expected line 12 to not be skipped")
	}

	// Line outside range for relative path: skipped.
	if !isGitDiffSkipped(gitChangedLines, "pkg/foo.go", "/repo/pkg/foo.go", 20) {
		t.Error("expected line 20 to be skipped")
	}

	// Fallback to absFile when relFile is empty.
	if isGitDiffSkipped(gitChangedLines, "", "/repo/pkg/foo.go", 12) {
		t.Error("expected line 12 with absFile to not be skipped")
	}
	if !isGitDiffSkipped(gitChangedLines, "", "/repo/pkg/foo.go", 20) {
		t.Error("expected line 20 with absFile to be skipped")
	}
}

func TestRecordOneMutationDryRunGitDiff(t *testing.T) {
	opts := &models.Options{}
	opts.General.DryRun = true
	gitChangedLines := gitdiff.ChangedLines{
		"foo.go": {{Start: 10, End: 10}},
	}
	r := &mutationRun{
		opts:            opts,
		gitChangedLines: gitChangedLines,
		moduleRoot:      "/repo",
	}
	m := mutatorItem{Name: "test-mutator"}
	fc := &fileContext{
		absFile: "/repo/foo.go",
	}

	dryRunCounts := make(map[string]int)
	dryRunGlobalTotals := make(map[string]int)

	// Mutation on changed line 10 should be counted.
	recordOneMutation(r, m, fc, mutago.PositionedMutation{}, 0, 10, nil, dryRunCounts, dryRunGlobalTotals)
	if dryRunCounts["test-mutator"] != 1 || dryRunGlobalTotals["test-mutator"] != 1 {
		t.Errorf("expected mutation on line 10 to be counted, got counts=%v, globals=%v", dryRunCounts, dryRunGlobalTotals)
	}

	// Mutation on unchanged line 20 should NOT be counted.
	recordOneMutation(r, m, fc, mutago.PositionedMutation{}, 1, 20, nil, dryRunCounts, dryRunGlobalTotals)
	if dryRunCounts["test-mutator"] != 1 || dryRunGlobalTotals["test-mutator"] != 1 {
		t.Errorf("expected mutation on line 20 to not be counted, got counts=%v, globals=%v", dryRunCounts, dryRunGlobalTotals)
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

// TestEngineCoverageTimeoutFailsBaseline ensures --coverage passes the execution timeout
// to the initial coverage collection step and fails fast if the unmutated test suite times out (#129).
func TestEngineCoverageTimeoutFailsBaseline(t *testing.T) {
	_ = os.MkdirAll("./testdata", 0755)
	tempDir, err := os.MkdirTemp("./testdata", "covtimeout-*")
	if err != nil {
		t.Fatalf("failed to create temp package: %v", err)
	}
	defer os.RemoveAll(tempDir)

	src := `package covtimeout

func Foo() int { return 42 }
`
	testSrc := `package covtimeout

import (
	"testing"
	"time"
)

func TestFoo(t *testing.T) {
	time.Sleep(2 * time.Second)
	_ = Foo()
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "pkg.go"), []byte(src), 0644); err != nil {
		t.Fatalf("failed to write pkg.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "pkg_test.go"), []byte(testSrc), 0644); err != nil {
		t.Fatalf("failed to write pkg_test.go: %v", err)
	}

	opts := &models.Options{}
	opts.Exec.Coverage = true
	opts.Exec.Timeout = 1
	opts.Remaining.Targets = []string{tempDir}

	var stdout, stderr bytes.Buffer
	e := &Engine{
		Stdout: &stdout,
		Stderr: &stderr,
	}

	res, err := e.Run(context.Background(), opts, nil)
	if err == nil {
		t.Fatal("expected baseline coverage timeout to return error, got nil")
	}
	if res.ExitCode != 3 {
		t.Fatalf("expected exit code 3 on baseline coverage timeout, got %d", res.ExitCode)
	}
	if !strings.Contains(err.Error(), "coverage test failed") {
		t.Fatalf("expected error to contain 'coverage test failed', got: %v", err)
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected error to mention 'timed out', got: %v", err)
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

// TestEngineCoverageRunsConstMutations is a regression test for #83: numeric
// literals in package-level const (and var) declarations are never recorded as
// covered by `go test` coverage profiles, so --coverage wrongly skipped them
// as NOT COVERED — even when a test asserts the exact value and would KILL
// them. Such mutations must be executed (scored killed/escaped), not skipped.
func TestEngineCoverageRunsConstMutations(t *testing.T) {
	_ = os.MkdirAll("./testdata", 0755)
	tempDir, err := os.MkdirTemp("./testdata", "constcov-*")
	if err != nil {
		t.Fatalf("failed to create temp package: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// `const Pi` is on line 3 of pkg.go; `Double` on line 5.
	src := `package constcov

const Pi = 3.14159

func Double(x int) int { return x * 2 }
`
	testSrc := `package constcov

import "testing"

func TestPi(t *testing.T) {
	if Pi != 3.14159 {
		t.Fatalf("Pi=%f", Pi)
	}
}

func TestDouble(t *testing.T) {
	if Double(3) != 6 {
		t.Fatalf("Double(3)=%d", Double(3))
	}
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "pkg.go"), []byte(src), 0644); err != nil {
		t.Fatalf("failed to write pkg.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "pkg_test.go"), []byte(testSrc), 0644); err != nil {
		t.Fatalf("failed to write pkg_test.go: %v", err)
	}

	// Restrict to numbers mutators so the only mutations are the numeric
	// literals in `const Pi` (line 3) and `Double` (line 5).
	opts := &models.Options{}
	opts.Exec.Coverage = true
	opts.Exec.Timeout = 10
	opts.Mutator.DisableMutators = []string{
		"arithmetic/*", "branch/*", "composite/*", "concurrency/*", "conditional/*",
		"expression/*", "loop/*", "select/*", "statement/*",
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

	// Count how many mutants on the `const Pi` line (line 3) landed in each
	// bucket. Before the fix all three float mutations on line 3 were NOT
	// COVERED; they must instead be scored (killed here, since TestPi asserts
	// the exact value).
	const constLine = int64(3)
	countByLine := func(muts []models.Mutant) int64 {
		var n int64
		for _, m := range muts {
			if m.Mutator.OriginalStartLine == constLine {
				n++
			}
		}
		return n
	}

	if n := countByLine(res.Report.NotCovered); n != 0 {
		t.Fatalf("const mutations must not be skipped as NOT COVERED; got %d on line %d (notCovered=%d killed=%d escaped=%d total=%d) stdout=%q stderr=%q",
			n, constLine, res.Report.Stats.NotCoveredCount, res.Report.Stats.KilledCount, res.Report.Stats.EscapedCount, res.Report.Stats.TotalMutantsCount, stdout.String(), stderr.String())
	}
	if n := countByLine(res.Report.Killed); n < 2 {
		t.Fatalf("expected at least 2 const mutations KILLED on line %d, got %d (notCovered=%d killed=%d escaped=%d total=%d) stdout=%q stderr=%q",
			constLine, n, res.Report.Stats.NotCoveredCount, res.Report.Stats.KilledCount, res.Report.Stats.EscapedCount, res.Report.Stats.TotalMutantsCount, stdout.String(), stderr.String())
	}
}

// TestEngineCoverageHonorsLineDirectives is a regression test for #84: a
// //line directive shifts Go's coverage-profile line attribution to the
// directive's line (and, for a named directive, its filename), while mutago
// reported the mutation's line using the adjusted line but looked up coverage
// using the physical file path. Covered mutations on directive-shifted lines
// were therefore classified NOT COVERED and skipped, understating MSI and
// inflating covered-code MSI. Such mutations must be scored, not skipped.
func TestEngineCoverageHonorsLineDirectives(t *testing.T) {
	_ = os.MkdirAll("./testdata", 0755)
	tempDir, err := os.MkdirTemp("./testdata", "linedir-*")
	if err != nil {
		t.Fatalf("failed to create temp package: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// The //line :200 directive shifts Sub's `return` to reported line 200.
	// Sub is exercised by TestSub; its mutation must be scored (KILLED), not
	// NOT COVERED. Plain (a function with no directive) guards the non-shifted
	// path so the test does not pass trivially by accident.
	src := "package linedir\n\n" +
		"func Sub(a, b int) int {\n" +
		"//line :200\n" +
		"\treturn a - b\n" +
		"}\n\n" +
		"func Plain(a, b int) int {\n" +
		"\treturn a + b\n" +
		"}\n"
	testSrc := "package linedir\n\n" +
		"import \"testing\"\n\n" +
		"func TestSub(t *testing.T) {\n" +
		"\tif got := Sub(5, 3); got != 2 { t.Fatalf(\"Sub=%d\", got) }\n" +
		"}\n\n" +
		"func TestPlain(t *testing.T) {\n" +
		"\tif got := Plain(1, 2); got != 3 { t.Fatalf(\"Plain=%d\", got) }\n" +
		"}\n"
	if err := os.WriteFile(filepath.Join(tempDir, "pkg.go"), []byte(src), 0644); err != nil {
		t.Fatalf("failed to write pkg.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "pkg_test.go"), []byte(testSrc), 0644); err != nil {
		t.Fatalf("failed to write pkg_test.go: %v", err)
	}

	opts := &models.Options{}
	opts.Exec.Coverage = true
	opts.Exec.Timeout = 10
	// Restrict to arithmetic mutators so the only mutations are the `a - b`
	// (Sub, directive-shifted to line 200) and `a + b` (Plain, physical line).
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

	// The Plain mutation is reported at the directive-shifted line 204: the
	// //line :200 directive shifts Sub's `return` to line 200 and, because the
	// directive persists, Plain's `return` to line 204. Go records Plain's
	// coverage block under the directive filename ("."), while mutago looked
	// up coverage under the physical file path — so Plain's covered mutation
	// was wrongly classified NOT COVERED. It must be scored (KILLED here).
	const shiftedLine = int64(204)
	countOnShiftedLine := func(muts []models.Mutant) int64 {
		var n int64
		for _, m := range muts {
			if m.Mutator.OriginalStartLine == shiftedLine {
				n++
			}
		}
		return n
	}

	if n := countOnShiftedLine(res.Report.NotCovered); n != 0 {
		t.Fatalf("directive-shifted covered mutations must not be NOT COVERED; got %d on line %d (notCovered=%d killed=%d escaped=%d total=%d) stdout=%q stderr=%q",
			n, shiftedLine, res.Report.Stats.NotCoveredCount, res.Report.Stats.KilledCount, res.Report.Stats.EscapedCount, res.Report.Stats.TotalMutantsCount, stdout.String(), stderr.String())
	}
	if n := countOnShiftedLine(res.Report.Killed); n < 1 {
		t.Fatalf("expected the Plain mutation KILLED on shifted line %d, got %d (notCovered=%d killed=%d escaped=%d total=%d) stdout=%q stderr=%q",
			shiftedLine, n, res.Report.Stats.NotCoveredCount, res.Report.Stats.KilledCount, res.Report.Stats.EscapedCount, res.Report.Stats.TotalMutantsCount, stdout.String(), stderr.String())
	}
}

// TestEngineBaselineRejectsBrokenBuild is a regression test for #85: without a
// baseline check by default, a package that does not compile reported 100%
// killed and exit 0 (a false-green) — `go test` exits 1 for a build failure,
// which mapTestExitToResult classified as KILLED. A broken baseline must fail
// fast with a tool error (exit 3) instead of a meaningless score.
func TestEngineBaselineRejectsBrokenBuild(t *testing.T) {
	_ = os.MkdirAll("./testdata", 0755)
	tempDir, err := os.MkdirTemp("./testdata", "brokenbuild-*")
	if err != nil {
		t.Fatalf("failed to create temp package: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Deliberate type error: undeclared variable. The package will not build.
	src := `package brokenbuild

func Add(a, b int) int {
	return a + b + undeclaredVar
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "pkg.go"), []byte(src), 0644); err != nil {
		t.Fatalf("failed to write pkg.go: %v", err)
	}

	opts := &models.Options{}
	opts.Exec.Timeout = 10
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
	if res.ExitCode != 3 {
		t.Fatalf("expected exit code 3 (tool error) for an uncompilable package, got %d (stdout=%q stderr=%q)",
			res.ExitCode, stdout.String(), stderr.String())
	}
	if res.Report != nil && res.Report.Stats.KilledCount > 0 && res.Report.Stats.TotalMutantsCount > 0 &&
		res.Report.Stats.KilledCount == res.Report.Stats.TotalMutantsCount {
		t.Fatalf("broken-build package reported all mutants KILLED (false green): killed=%d total=%d stdout=%q stderr=%q",
			res.Report.Stats.KilledCount, res.Report.Stats.TotalMutantsCount, stdout.String(), stderr.String())
	}
}

func TestCheckMsiGate_FloatPrecision(t *testing.T) {
	// 29 out of 100 mutants killed = exactly 29.0%
	report := &models.Report{
		Stats: models.Stats{
			TotalMutantsCount: 100,
			KilledCount:       29,
			EscapedCount:      71,
			Msi:               29.0 / 100.0,
		},
	}
	if checkMsiGate(io.Discard, report, 29.0) {
		t.Fatalf("checkMsiGate failed for exact 29%% threshold")
	}

	// 28 out of 100 mutants killed = 28.0%, should fail 29.0% gate
	reportBelow := &models.Report{
		Stats: models.Stats{
			TotalMutantsCount: 100,
			KilledCount:       28,
			EscapedCount:      72,
			Msi:               28.0 / 100.0,
		},
	}
	if !checkMsiGate(io.Discard, reportBelow, 29.0) {
		t.Fatalf("checkMsiGate expected to fail for 28%% when min is 29%%")
	}
}

func TestCheckCoveredMsiGate_FloatPrecision(t *testing.T) {
	// 58 out of 100 covered mutants killed = exactly 58.0%
	report := &models.Report{
		HasCoverage: true,
		Stats: models.Stats{
			TotalMutantsCount: 100,
			KilledCount:       58,
			EscapedCount:      42,
			CoveredCodeMsi:    58.0 / 100.0,
		},
	}
	if checkCoveredMsiGate(io.Discard, report, 58.0) {
		t.Fatalf("checkCoveredMsiGate failed for exact 58%% threshold")
	}

	// 57 out of 100 covered mutants killed = 57.0%, should fail 58.0% gate
	reportBelow := &models.Report{
		HasCoverage: true,
		Stats: models.Stats{
			TotalMutantsCount: 100,
			KilledCount:       57,
			EscapedCount:      43,
			CoveredCodeMsi:    57.0 / 100.0,
		},
	}
	if !checkCoveredMsiGate(io.Discard, reportBelow, 58.0) {
		t.Fatalf("checkCoveredMsiGate expected to fail for 57%% when min is 58%%")
	}
}

func vetArgs(args []string) []string {
	vets := make([]string, 0, 2)
	for _, arg := range args {
		if arg == "-vet" || arg == "--vet" || strings.HasPrefix(arg, "-vet=") || strings.HasPrefix(arg, "--vet=") {
			vets = append(vets, arg)
		}
	}
	return vets
}

func TestMutantGoTestArgsDisablesVetByDefault(t *testing.T) {
	args := mutantGoTestArgs("overlay.json", 60, nil, "", "example")
	if got := vetArgs(args); len(got) != 1 || got[0] != "-vet=off" {
		t.Fatalf("expected exactly one -vet argument (-vet=off), got %v in %v", got, args)
	}
}

func TestMutantGoTestArgsUserVetWins(t *testing.T) {
	args := mutantGoTestArgs("overlay.json", 60, []string{"-vet=atomic"}, "", "example")
	if got := vetArgs(args); len(got) != 1 || got[0] != "-vet=atomic" {
		t.Fatalf("user -vet must win with exactly one -vet argument, got %v in %v", got, args)
	}
}

func failfastArgs(args []string) []string {
	var got []string
	for _, arg := range args {
		if arg == "-failfast" || arg == "--failfast" || strings.HasPrefix(arg, "-failfast=") || strings.HasPrefix(arg, "--failfast=") {
			got = append(got, arg)
		}
	}
	return got
}

func TestMutantGoTestArgsFailFastByDefault(t *testing.T) {
	args := mutantGoTestArgs("overlay.json", 60, nil, "", "example")
	if got := failfastArgs(args); len(got) != 1 || got[0] != "-failfast" {
		t.Fatalf("expected exactly one -failfast argument, got %v in %v", got, args)
	}
}

func TestMutantGoTestArgsUserFailFastWins(t *testing.T) {
	args := mutantGoTestArgs("overlay.json", 60, []string{"-failfast=false"}, "", "example")
	if got := failfastArgs(args); len(got) != 1 || got[0] != "-failfast=false" {
		t.Fatalf("user -failfast must win with exactly one -failfast argument, got %v in %v", got, args)
	}
}

func TestClassifyGoTestBuildFailureAsSkipped(t *testing.T) {
	buildFailure := []byte("# example.com/aliasbug\nalias.go:8:9: undefined: url\nFAIL\texample.com/aliasbug [build failed]\n")
	if got := classifyGoTestResult(1, buildFailure); got != 2 {
		t.Fatalf("expected build failure to be skipped, got exit code %d", got)
	}

	setupFailure := []byte("FAIL\texample.com/aliasbug [setup failed]\n")
	if got := classifyGoTestResult(1, setupFailure); got != 2 {
		t.Fatalf("expected setup failure to be skipped, got exit code %d", got)
	}

	testFailure := []byte("--- FAIL: TestValue (0.00s)\nFAIL\texample.com/aliasbug\t0.001s\n")
	if got := classifyGoTestResult(1, testFailure); got != 1 {
		t.Fatalf("expected test failure to remain a killed result, got exit code %d", got)
	}

	if got := classifyGoTestResult(0, buildFailure); got != 0 {
		t.Fatalf("expected successful go test to remain successful, got exit code %d", got)
	}

	if got := classifyGoTestResult(0, setupFailure); got != 0 {
		t.Fatalf("expected successful go test to remain successful, got exit code %d", got)
	}
}
