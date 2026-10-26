package engine

import (
	"runtime"
	"strings"

	"github.com/quality-gates/mutago/v2/internal/models"
)

// runnerPolicy resolves custom execution once. Go test preparation retains its
// own flags; they must never become arguments to an arbitrary custom command.
type runnerPolicy struct {
	command   []string
	testFlags []string
	workers   int
	baseline  bool
	goTest    bool
	coverage  bool
	adaptive  bool
	perTest   bool
	run       func(execJob, *models.Mutant) int
}

func resolveRunner(opts *models.Options, mode runMode) (runnerPolicy, []string) {
	r := runnerPolicy{
		command:   strings.Fields(opts.Exec.Exec),
		testFlags: strings.Fields(opts.Exec.TestFlags),
		workers:   requestedWorkers(opts.General.Workers),
		coverage:  mode.executes() && opts.Exec.Coverage,
		run:       runBuiltinExec,
	}
	custom := len(r.command) > 0
	builtin := mode.executes() && !custom
	r.goTest = builtin || r.coverage
	r.baseline = builtin && !r.coverage
	r.adaptive = builtin && opts.Exec.TimeoutCoefficient > 0
	r.perTest = builtin && opts.Exec.PerTest
	if !custom {
		return r, nil
	}
	r.workers = 1 // custom scripts may modify the original source file
	r.run = runCustomExec
	if !mode.executes() {
		return r, nil
	}
	return r, r.customWarnings(opts)
}

func requestedWorkers(n int) int {
	if n <= 0 {
		return runtime.NumCPU()
	}
	return n
}

func (r runnerPolicy) customWarnings(opts *models.Options) []string {
	var warnings []string
	if opts.Exec.PerTest {
		warnings = append(warnings, "--per-test is ignored with --exec")
	}
	if opts.Exec.TimeoutCoefficient > 0 {
		warnings = append(warnings, "--timeout-coefficient is ignored with --exec; --exec-timeout remains the script timeout")
	}
	if len(r.testFlags) > 0 && !r.goTest {
		warnings = append(warnings, "--test-flags is unused with --exec without --coverage")
	}
	return warnings
}

func (r runnerPolicy) validateTestCount() error {
	if !r.adaptive {
		return nil
	}
	return validateAdaptiveTestCount(r.testFlags)
}
