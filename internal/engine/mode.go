package engine

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/quality-gates/mutago/v2"
	"github.com/quality-gates/mutago/v2/internal/baseline"
	"github.com/quality-gates/mutago/v2/internal/models"
)

// runMode is what a run does with each admitted mutant. It is resolved once
// from the options; setup, discovery, and post-run steps ask it rather than
// re-reading --dry-run and --no-exec.
type runMode int

const (
	// modeExecute tests every admitted mutant and reports the results.
	modeExecute runMode = iota
	// modeDryRun counts admitted mutants without writing files.
	modeDryRun
	// modeGenerate writes each admitted mutant's source and runs nothing.
	modeGenerate
)

func resolveRunMode(opts *models.Options) runMode {
	switch {
	case opts.General.DryRun:
		return modeDryRun
	case opts.Exec.NoExec:
		return modeGenerate
	default:
		return modeExecute
	}
}

// executes reports whether the run tests mutants, so it needs workers,
// baseline checks, and coverage.
func (m runMode) executes() bool {
	return m == modeExecute
}

// materializes reports whether the run writes mutant sources to a temp dir
// and records report bookkeeping.
func (m runMode) materializes() bool {
	return m != modeDryRun
}

// mutantSink receives each mutant that discovery admits.
type mutantSink interface {
	// admit takes one admitted mutant.
	admit(fc *fileContext, mutation mutago.PositionedMutation, mutationID int, prepared preparedMutation)
	// flush ends one discovery pass over fc.
	flush(fc *fileContext)
	// finish runs the post-run steps that apply and returns the exit code.
	finish(run *mutationRun, bl *baseline.File) int
}

func newMutantSink(r *mutationRun, jobs chan<- execJob) mutantSink {
	switch r.exec.mode {
	case modeDryRun:
		return &dryRunSink{stdout: r.stdout, totals: make(map[string]int), pending: make(map[string]int)}
	case modeGenerate:
		return &generateSink{stderr: r.stderr}
	default:
		return &executeSink{r: r, jobs: jobs}
	}
}

// executeSink queues each mutant for a worker and reports the results.
type executeSink struct {
	r    *mutationRun
	jobs chan<- execJob
}

func (s *executeSink) admit(fc *fileContext, mutation mutago.PositionedMutation, mutationID int, prepared preparedMutation) {
	queueMutation(s.r, s.jobs, fc, mutation, mutationID, fc.originalSource, prepared)
}

func (s *executeSink) flush(*fileContext) {}

func (s *executeSink) finish(run *mutationRun, bl *baseline.File) int {
	run.report.Calculate()
	return finalizeResults(run.stdout, run.stderr, run.opts, run.report, bl, run.roots, run.runMutantIDFound.Load())
}

// dryRunSink counts mutants per file and mutator.
type dryRunSink struct {
	stdout  io.Writer
	totals  map[string]int
	pending map[string]int
}

func (s *dryRunSink) admit(_ *fileContext, _ mutago.PositionedMutation, _ int, prepared preparedMutation) {
	name := prepared.mutant.Mutator.MutatorName
	s.pending[name]++
	s.totals[name]++
}

func (s *dryRunSink) flush(fc *fileContext) {
	if len(s.pending) == 0 {
		return
	}
	fmt.Fprintf(s.stdout, "%s:\n", fc.sourceFile)
	var keys []string
	for k := range s.pending {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(s.stdout, "\t%s: %d\n", k, s.pending[k])
	}
	s.pending = make(map[string]int)
}

func (s *dryRunSink) finish(run *mutationRun, _ *baseline.File) int {
	var total int
	for _, count := range s.totals {
		total += count
	}
	run.report.Stats.TotalMutantsCount = int64(total)
	printDryRunReport(s.stdout, total, s.totals)
	return returnOk
}

// generateSink writes each mutant's source next to the file's .original copy.
// It skips the summary, baseline update, reports, and quality gates, because
// no mutant was tested.
type generateSink struct {
	stderr io.Writer
	failed bool
}

func (s *generateSink) admit(fc *fileContext, _ mutago.PositionedMutation, mutationID int, prepared preparedMutation) {
	mutated, err := prepared.edit.materialize(fc.originalSource)
	if err == nil {
		err = os.WriteFile(mutationFilePath(fc, mutationID), mutated, 0666)
	}
	if err != nil {
		fmt.Fprintf(s.stderr, "Cannot write mutation: %v\n", err)
		s.failed = true
	}
}

func (s *generateSink) flush(*fileContext) {}

func (s *generateSink) finish(run *mutationRun, _ *baseline.File) int {
	if code := checkRunMutantIDFound(run.stderr, run.opts, run.runMutantIDFound.Load()); code != returnOk {
		return code
	}
	fmt.Fprintln(run.stdout, "Cannot do a mutation testing summary since no exec command was executed.")
	if s.failed {
		return returnError
	}
	return returnOk
}
