package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// goTestKind names the purpose of a `go test` invocation.
type goTestKind int

const (
	// baselineRun checks that the unmutated package is green.
	baselineRun goTestKind = iota
	// coverageRun writes the coverage profile used to skip uncovered mutants.
	coverageRun
	// mutantRun executes the tests against one mutant via an overlay.
	mutantRun
	// listTestsRun lists the top-level tests of one package (--per-test).
	listTestsRun
	// compileTestBinary builds one package's coverage-instrumented test
	// binary, which per-test profiling runs once per test (--per-test).
	compileTestBinary
)

// goTestTarget is the import path `go test` runs for a mutant and the
// directory it runs in.
type goTestTarget struct {
	pkg string
	dir string
}

// goTestInvocation describes one `go test` command. Baseline, coverage,
// mutant and per-test runs are all built here so the package target,
// recursion, vet, timeout and user flag rules stay identical across them;
// only the per-kind extras differ.
type goTestInvocation struct {
	kind   goTestKind
	target string
	// dir is the directory go runs in; empty means the current directory
	// (see goWorkDir).
	dir            string
	recursive      bool
	timeoutSeconds uint
	testFlags      []string
	// profilePath is the -coverprofile destination (coverageRun only).
	profilePath string
	// overlay is the -overlay file that swaps in the mutant (mutantRun only).
	overlay string
	// runFilter restricts the tests run to those covering the mutant
	// (mutantRun only, from --per-test).
	runFilter string
	// execProgram is the -exec wrapper for a mutant run. Empty means go test
	// runs the test binary directly. A user -exec in test flags wins.
	execProgram string
	// coverPkg is the -coverpkg target (compileTestBinary only).
	coverPkg string
	// binaryPath is the -o destination (compileTestBinary only).
	binaryPath string
}

// args returns the argument list for `go`.
//
// Mutants are not meant to be lint-clean, so `go vet` is disabled by default
// (see #106): `go test` runs a vet subset that exits 1 on any diagnostic, and
// mapTestExitToResult would count such a mutant as KILLED even though no test
// failed. The baseline and coverage runs disable vet too, so they judge the
// same test outcome the mutants do. An explicit -vet in the user's test flags
// wins everywhere.
//
// With --test-recursive every kind targets pkg/..., so the baseline and
// coverage runs see the same tests as the mutants. The coverage run also
// passes -coverpkg=pkg so tests in subpackages count towards the target's
// coverage.
//
// Listing tests takes only the user's build flags: they decide which test
// files exist, while runner flags such as -run or -v would change the list.
//
// -failfast is on by default for mutant runs only: one failing test is enough
// to kill a mutant, so the rest of the suite need not run. An explicit
// -failfast in the user's test flags wins.
func (g goTestInvocation) args() []string {
	args := []string{"test"}
	args = append(args, g.kindArgs()...)
	args = append(args, "-timeout", fmt.Sprintf("%ds", g.timeoutSeconds))
	testFlags := g.testFlags
	if g.kind == listTestsRun {
		testFlags = buildTestFlags(testFlags)
	}
	args = append(args, testFlags...)
	if !hasTestFlag(testFlags, "vet") {
		args = append(args, "-vet=off")
	}
	args = append(args, g.mutantArgs()...)
	target := g.target
	if g.recursive {
		target += "/..."
	}
	return append(args, target)
}

// command returns the `go test` command, run in g.dir.
func (g goTestInvocation) command(ctx context.Context) *exec.Cmd {
	return goCommandIn(ctx, g.dir, g.args()...)
}

func (g goTestInvocation) kindArgs() []string {
	switch g.kind {
	case coverageRun:
		args := []string{"-coverprofile=" + g.profilePath}
		if g.recursive {
			args = append(args, "-coverpkg="+g.target)
		}
		return args
	case mutantRun:
		return []string{"-overlay=" + g.overlay}
	case listTestsRun:
		return []string{"-list", ".*"}
	case compileTestBinary:
		return []string{"-c", "-cover", "-covermode=set", "-coverpkg=" + g.coverPkg, "-o", g.binaryPath}
	default:
		return nil
	}
}

func (g goTestInvocation) mutantArgs() []string {
	if g.kind != mutantRun {
		return nil
	}
	var args []string
	if g.execProgram != "" && !hasTestFlag(g.testFlags, "exec") {
		args = append(args, "-exec="+g.execProgram)
	}
	if !hasTestFlag(g.testFlags, "failfast") {
		args = append(args, "-failfast")
	}
	if g.runFilter != "" {
		args = append(args, "-run", g.runFilter)
	}
	return args
}

// perTestToolchain builds the commands --per-test profiling runs, so the
// user's test flags reach them by the same rules as every other go test run.
type perTestToolchain struct {
	testFlags      []string
	timeoutSeconds uint
	dir            string
}

func (t perTestToolchain) ListTests(pkgPath string) *exec.Cmd {
	return goTestInvocation{kind: listTestsRun, target: pkgPath, dir: t.dir, timeoutSeconds: t.timeoutSeconds, testFlags: t.testFlags}.command(context.Background())
}

// ListPackages runs `go list` with the user's build flags, so a package whose
// files all sit behind a build tag is still found.
func (t perTestToolchain) ListPackages(pattern string) *exec.Cmd {
	args := append([]string{"list"}, buildTestFlags(t.testFlags)...)
	return goCommandIn(context.Background(), t.dir, append(args, pattern)...)
}

func (t perTestToolchain) CompileTestBinary(pkgPath, coverPkg, binaryPath string) *exec.Cmd {
	return goTestInvocation{
		kind:           compileTestBinary,
		target:         pkgPath,
		dir:            t.dir,
		timeoutSeconds: t.timeoutSeconds,
		testFlags:      t.testFlags,
		coverPkg:       coverPkg,
		binaryPath:     binaryPath,
	}.command(context.Background())
}

// RunTest runs one test of a binary from CompileTestBinary. The user's runner
// flags come first so the explicit -test.run, profile and timeout win.
func (t perTestToolchain) RunTest(binaryPath, testName, profilePath string) *exec.Cmd {
	args := append(testBinaryFlags(t.testFlags),
		"-test.run=^"+testName+"$",
		"-test.coverprofile="+profilePath,
		fmt.Sprintf("-test.timeout=%ds", t.timeoutSeconds))
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = os.Environ()
	return cmd
}

func goCommand(args ...string) *exec.Cmd {
	cmd := exec.Command("go", args...)
	cmd.Env = os.Environ()
	return cmd
}

// goCommandIn returns a go command run in dir ("" for the current directory).
// Its PWD names dir, so go resolves relative and absolute paths against the
// same spelling of the directory as mutago does, symlinks included.
func goCommandIn(ctx context.Context, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = cmd.Environ()
	return cmd
}

// importPaths resolves a package's import path from its files and remembers
// the answer per directory, so `go list` runs at most once per package per
// run however many of the baseline, coverage and per-test steps need it. It
// is used from the run's orchestrating goroutine only.
type importPaths struct {
	lookup func(dir string) string
	byDir  map[string]string
}

// newImportPaths resolves import paths by running `go list` in workDir with
// the user's build flags, so a package whose files all sit behind a build tag
// is still found.
func newImportPaths(workDir string, testFlags []string) *importPaths {
	buildFlags := buildTestFlags(testFlags)
	return &importPaths{lookup: func(dir string) string {
		return goListImportPath(workDir, buildFlags, dir)
	}}
}

// forFiles returns the import path of the package containing files, or ""
// when it cannot be determined.
func (p *importPaths) forFiles(files []string) string {
	if len(files) == 0 {
		return ""
	}
	f, err := filepath.Abs(files[0])
	if err != nil {
		return ""
	}
	dir := filepath.Dir(f)
	if path, ok := p.byDir[dir]; ok {
		return path
	}
	if p.byDir == nil {
		p.byDir = make(map[string]string)
	}
	path := p.lookup(dir)
	p.byDir[dir] = path
	return path
}

func goListImportPath(workDir string, buildFlags []string, dir string) string {
	args := append(append([]string{"list"}, buildFlags...), dir)
	out, err := goCommandIn(context.Background(), workDir, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// goEnv is the go environment of the current directory.
type goEnv struct {
	goMod  string // `go env GOMOD`: "" in GOPATH mode, os.DevNull outside a module
	goWork string // `go env GOWORK`: "" when no workspace is in use
}

func currentGoEnv() goEnv {
	out, err := goCommand("env", "GOMOD", "GOWORK").Output()
	if err != nil {
		return goEnv{}
	}
	lines := strings.Split(string(out), "\n")
	for len(lines) < 2 {
		lines = append(lines, "")
	}
	return goEnv{goMod: strings.TrimSpace(lines[0]), goWork: strings.TrimSpace(lines[1])}
}

// goWorkDir returns the directory the run's go commands start in. That is the
// current directory ("") when the module or workspace go uses there holds the
// targets, as in a normal run. Otherwise it is the root of the module that
// holds them, so a run from a parent directory or from an unrelated module
// still builds and tests the targets rather than the current directory (#287).
func goWorkDir(cwd goEnv, files []string) (string, error) {
	if cwd.goMod == "" || cwd.goWork != "" {
		return "", nil
	}
	goMods := make(map[string]struct{})
	for _, file := range files {
		abs, err := filepath.Abs(file)
		if err != nil {
			return "", err
		}
		goMod := nearestGoMod(filepath.Dir(abs))
		if goMod == "" {
			return "", fmt.Errorf("%s is not inside a Go module", file)
		}
		goMods[goMod] = struct{}{}
	}
	if len(goMods) > 1 {
		return "", errTargetsSpanModules
	}
	for goMod := range goMods {
		if goMod != cwd.goMod {
			return filepath.Dir(goMod), nil
		}
	}
	return "", nil
}

var errTargetsSpanModules = errors.New("targets belong to more than one Go module; run mutago once per module")

// nearestGoMod returns the go.mod go would use for dir: the closest one in
// dir or its parents, or "" when there is none.
func nearestGoMod(dir string) string {
	for {
		goMod := filepath.Join(dir, "go.mod")
		if info, err := os.Stat(goMod); err == nil && !info.IsDir() {
			return goMod
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
