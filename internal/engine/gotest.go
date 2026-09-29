package engine

import (
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
)

// goTestInvocation describes one `go test` command. Baseline, coverage and
// mutant runs are all built here so the package target, recursion, vet and
// timeout rules stay identical across them; only the per-kind extras differ.
type goTestInvocation struct {
	kind           goTestKind
	target         string
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
// -failfast is on by default for mutant runs only: one failing test is enough
// to kill a mutant, so the rest of the suite need not run. An explicit
// -failfast in the user's test flags wins.
func (g goTestInvocation) args() []string {
	args := []string{"test"}
	args = append(args, g.kindArgs()...)
	args = append(args, "-timeout", fmt.Sprintf("%ds", g.timeoutSeconds))
	args = append(args, g.testFlags...)
	if !hasVetFlag(g.testFlags) {
		args = append(args, "-vet=off")
	}
	args = append(args, g.mutantArgs()...)
	target := g.target
	if g.recursive {
		target += "/..."
	}
	return append(args, target)
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

func hasVetFlag(testFlags []string) bool {
	for _, flag := range testFlags {
		if flag == "-vet" || flag == "--vet" || strings.HasPrefix(flag, "-vet=") || strings.HasPrefix(flag, "--vet=") {
			return true
		}
	}
	return false
}

func hasTestFlag(testFlags []string, name string) bool {
	for _, flag := range testFlags {
		trimmed := strings.TrimPrefix(strings.TrimPrefix(flag, "-"), "-")
		if trimmed == name || strings.HasPrefix(trimmed, name+"=") {
			return true
		}
	}
	return false
}

// importPaths resolves a package's import path from its files and remembers
// the answer per directory, so `go list` runs at most once per package per
// run however many of the baseline, coverage and per-test steps need it. It
// is used from the run's orchestrating goroutine only.
type importPaths struct {
	lookup func(dir string) string
	byDir  map[string]string
}

func newImportPaths() *importPaths {
	return &importPaths{lookup: goListImportPath}
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

func goListImportPath(dir string) string {
	cmd := exec.Command("go", "list", dir)
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
