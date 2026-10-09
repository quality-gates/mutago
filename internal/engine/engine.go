package engine

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/printer"
	"go/token"
	"go/types"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/quality-gates/mutago/v2"
	"github.com/quality-gates/mutago/v2/astutil"
	"github.com/quality-gates/mutago/v2/internal/annotation"
	"github.com/quality-gates/mutago/v2/internal/baseline"
	"github.com/quality-gates/mutago/v2/internal/console"
	"github.com/quality-gates/mutago/v2/internal/coverage"
	"github.com/quality-gates/mutago/v2/internal/filter"
	"github.com/quality-gates/mutago/v2/internal/gitdiff"
	"github.com/quality-gates/mutago/v2/internal/importing"
	"github.com/quality-gates/mutago/v2/internal/models"
	"github.com/quality-gates/mutago/v2/internal/parser"
	"github.com/quality-gates/mutago/v2/internal/reportmaker"
	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/zimmski/osutil"
)

const (
	returnOk                       = 0
	returnError                    = 3
	returnMsiThresholdNotMet       = 4
	adaptiveBaselineTimeoutSeconds = 300
)

// Engine orchestrates the mutation testing lifecycle.
type Engine struct {
	// Stdout is the writer for engine output. Writes during Run are serialized.
	Stdout io.Writer
	// Stderr is the writer for engine diagnostics. Writes during Run are serialized.
	Stderr io.Writer
}

type synchronizedWriter struct {
	mu     *sync.Mutex
	writer io.Writer
}

func (w *synchronizedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(p)
}

func synchronizedWriters(stdout, stderr io.Writer) (io.Writer, io.Writer) {
	mu := &sync.Mutex{}
	return &synchronizedWriter{mu: mu, writer: stdout}, &synchronizedWriter{mu: mu, writer: stderr}
}

// Result holds the final status of a mutation run.
type Result struct {
	Report   *models.Report
	ExitCode int
}

type mutatorItem struct {
	Name    string
	Mutator mutator.Mutator
}

type execConfig struct {
	numWorkers     int
	execs          []string
	extraTestFlags []string
	importPaths    *importPaths
	// goDir is the directory go commands run in (see goWorkDir).
	goDir string
}

type mutationRun struct {
	ctx              context.Context
	opts             *models.Options
	mutators         []mutatorItem
	scope            *mutantScope
	tmpDir           string
	exec             execConfig
	report           *models.Report
	mu               *sync.Mutex
	modulePath       string
	moduleRoot       string
	jobs             chan<- execJob
	stdout           io.Writer
	stderr           io.Writer
	runMutantIDFound *atomic.Bool
}

// jobOutput is where a worker writes console output for one mutation.
type jobOutput struct {
	stdout io.Writer
	stderr io.Writer
}

type execJob struct {
	ctx            context.Context
	opts           *models.Options
	pkg            *types.Package
	test           goTestTarget
	mutant         models.Mutant
	coverProfile   *coverage.Profile
	execs          []string
	perTestProf    *coverage.PerTestProfile
	extraTestFlags []string
	tmpDir         string
	out            jobOutput
	source         mutationSource
	// packageLevelDecl is true when the mutation sits inside a package-level
	// const/var/type/import declaration. Such declarations are not executable
	// and never appear in `go test` coverage profiles, so the --coverage
	// filter must not skip them as NOT COVERED (see #83).
	packageLevelDecl bool
	// directiveShifted is true when a //line directive altered the position
	// mutago reports for this mutation (filename and/or line differ from the
	// raw source position). Go's coverage profile records such positions under
	// the directive's filename (or "." for a filename-less directive), not the
	// physical file, so the coverage lookup must consult the directive
	// filename too (see #84).
	directiveShifted bool
	// adjRelFile is the directive-adjusted filename relative to the module
	// root (the filename Go's coverage profile uses for this position). It is
	// only meaningful when directiveShifted is true.
	adjRelFile string
}

type mutationSource struct {
	originalFile string
	mutationFile string
	absFile      string
	relFile      string
	moduleRoot   string
	original     []byte
	edit         mutationEdit
}

type fileContext struct {
	pkg            *types.Package
	testPkg        string
	info           *types.Info
	fset           *token.FileSet
	src            ast.Node
	sourceFile     string
	mutatedFile    string
	absFile        string
	coverProfile   *coverage.Profile
	perTestProf    *coverage.PerTestProfile
	filters        []filter.NodeFilter
	originalSource []byte
	// identities maps mutator name to the stable IDs of its edits in this
	// file, keyed by mutationEdit.key. See mutantIdentities.
	identities map[string]map[string]string
}

// Run executes the mutation testing lifecycle based on options and baseline.
func (e *Engine) Run(ctx context.Context, opts *models.Options, bl *baseline.File) (Result, error) {
	return e.RunResolved(ctx, opts, bl, importing.ResolveTargets(opts.Remaining.Targets, opts))
}

// RunResolved executes a mutation run using targets discovered by the caller.
func (e *Engine) RunResolved(ctx context.Context, opts *models.Options, bl *baseline.File, targets importing.ResolvedTargets) (Result, error) {
	e.initDefaults()
	stdout, stderr := synchronizedWriters(e.Stdout, e.Stderr)
	runEngine := &Engine{Stdout: stdout, Stderr: stderr}
	setup, err := runEngine.validateAndInitRun(ctx, opts, targets)
	if err != nil {
		return Result{ExitCode: returnError}, err
	}
	run, pkgs := setup.run, setup.pkgs
	if run == nil {
		// initRun returns a nil run with a non-nil result for early exits.
		return Result{ExitCode: returnError}, nil
	}

	cleanup := newRunCleanup(setup)
	defer cleanup()

	report := run.report
	if exitCode := runBaselineChecks(runEngine.Stderr, opts, pkgs, run.exec); exitCode != 0 {
		return Result{Report: report, ExitCode: exitCode}, nil
	}

	coverageProfiles, err := configureAdaptiveTimeoutAndCoverage(opts, pkgs, run)
	if err != nil {
		return Result{Report: report, ExitCode: returnError}, err
	}

	dryRunTotal, dryRunMutatorTotals, loopCode := mutateAll(run, pkgs, coverageProfiles)

	cleanup()

	if loopCode != returnOk {
		return Result{Report: report, ExitCode: loopCode}, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return Result{Report: report, ExitCode: returnError}, err
	}

	if opts.General.DryRun {
		report.Stats.TotalMutantsCount = int64(dryRunTotal)
		printDryRunReport(run.stdout, dryRunTotal, dryRunMutatorTotals)
		return Result{Report: report, ExitCode: returnOk}, nil
	}

	report.Calculate()
	exitCode := finalizeResults(runEngine.Stdout, runEngine.Stderr, opts, report, bl, run.moduleRoot, run.runMutantIDFound.Load())
	return Result{Report: report, ExitCode: exitCode}, nil
}

func newRunCleanup(setup *runSetup) func() {
	opts := setup.run.opts
	if opts.General.DryRun {
		return func() {}
	}
	var cleanedUp bool
	return func() {
		if !cleanedUp {
			cleanedUp = true
			shutdownAndCleanup(setup.run.stderr, opts, setup.jobs, setup.jobWg, setup.stopProgress, setup.progressWg, setup.run.tmpDir)
		}
	}
}

func (e *Engine) initDefaults() {
	if e.Stdout == nil {
		e.Stdout = os.Stdout
	}
	if e.Stderr == nil {
		e.Stderr = os.Stderr
	}
}

// runSetup is the initialised run plus the worker and progress handles that
// cleanup must shut down.
type runSetup struct {
	run          *mutationRun
	pkgs         []importing.Package
	jobs         chan execJob
	jobWg        *sync.WaitGroup
	stopProgress chan struct{}
	progressWg   *sync.WaitGroup
}

func (e *Engine) validateAndInitRun(ctx context.Context, opts *models.Options, targets importing.ResolvedTargets) (*runSetup, error) {
	if err := validateAdaptiveTimeoutTestCount(opts); err != nil {
		return nil, err
	}
	return e.initRun(ctx, opts, targets)
}

func (e *Engine) initRun(ctx context.Context, opts *models.Options, targets importing.ResolvedTargets) (*runSetup, error) {
	files := targets.Files
	if len(files) == 0 {
		return nil, fmt.Errorf("Could not find any suitable Go source files")
	}

	mutationBlackList, err := loadBlacklist(opts.Files.Blacklist)
	if err != nil {
		return nil, err
	}

	gitChangedLines, err := loadGitDiffLines(opts)
	if err != nil {
		return nil, fmt.Errorf("Cannot load git diff: %w", err)
	}

	execs, extraTestFlags := parseExecFlags(opts)
	goDir, err := loadTargetPackages(opts, execs, files)
	if err != nil {
		return nil, err
	}

	pkgs := targets.Packages

	warnIgnoredSelectors(e.Stderr, opts)

	report := &models.Report{}
	var reportMu sync.Mutex

	numWorkers := calcNumWorkers(opts, execs)
	console.Verbose(opts, "Running with %d parallel worker(s)", numWorkers)

	tmpDir, err := createTmpDir(opts)
	if err != nil {
		return nil, err
	}

	var jobs chan execJob
	var jobWg *sync.WaitGroup
	runMutantIDFound := &atomic.Bool{}
	if !opts.General.DryRun && !opts.Exec.NoExec {
		jobs, jobWg = startWorkerPool(opts, numWorkers, report, &reportMu, tmpDir)
	}

	var stopProgress chan struct{}
	var progressWg *sync.WaitGroup
	if progressMonitorEnabled(opts) {
		stopProgress, progressWg = startProgressMonitor(opts, report, &reportMu, e.Stderr)
	}

	run := &mutationRun{
		ctx:      ctx,
		opts:     opts,
		mutators: buildActiveMutators(opts),
		scope:    newMutantScope(gitChangedLines, opts.Exec.RunMutantID, mutationBlackList),
		tmpDir:   tmpDir,
		exec: execConfig{
			numWorkers:     numWorkers,
			execs:          execs,
			extraTestFlags: extraTestFlags,
			importPaths:    newImportPaths(goDir, extraTestFlags),
			goDir:          goDir,
		},
		report:           report,
		mu:               &reportMu,
		modulePath:       detectModulePath(goDir),
		moduleRoot:       detectModuleRoot(goDir),
		jobs:             jobs,
		stdout:           e.Stdout,
		stderr:           e.Stderr,
		runMutantIDFound: runMutantIDFound,
	}

	return &runSetup{
		run:          run,
		pkgs:         pkgs,
		jobs:         jobs,
		jobWg:        jobWg,
		stopProgress: stopProgress,
		progressWg:   progressWg,
	}, nil
}

func createTmpDir(opts *models.Options) (string, error) {
	if opts.General.DryRun {
		return "", nil
	}
	tmpDir, err := os.MkdirTemp("", "mutago-")
	if err != nil {
		return "", fmt.Errorf("Cannot create temp directory: %w", err)
	}
	console.Verbose(opts, "Save mutations into %q", tmpDir)
	return tmpDir, nil
}

func buildActiveMutators(opts *models.Options) []mutatorItem {
	enable, _ := mutator.ParseSelector(opts.Config.EnableMutators)
	disable, _ := mutator.ParseSelector(disablePatterns(opts))
	var mutators []mutatorItem
	for _, name := range mutator.List() {
		if len(opts.Config.EnableMutators) > 0 && !enable.Matches(name) {
			continue
		}
		if disable.Matches(name) {
			continue
		}
		console.Verbose(opts, "Enable mutator %q", name)
		m, _ := mutator.New(name)
		mutators = append(mutators, mutatorItem{Name: name, Mutator: m})
	}
	return mutators
}

// disablePatterns returns the --disable patterns followed by the
// disable_mutators patterns, in a new slice.
func disablePatterns(opts *models.Options) []string {
	return append(append([]string{}, opts.Mutator.DisableMutators...), opts.Config.DisableMutators...)
}

// warnIgnoredSelectors writes a warning for each enable or disable selector
// that matches no registered mutator and each ignore_source_lines pattern that
// does not compile, so typos are not silently ignored.
func warnIgnoredSelectors(w io.Writer, opts *models.Options) {
	selectors := append(disablePatterns(opts), opts.Config.EnableMutators...)
	var unknown *mutator.UnknownPatternsError
	if _, err := mutator.ParseSelector(selectors); errors.As(err, &unknown) {
		for _, selector := range unknown.Patterns {
			fmt.Fprintf(w, "warning: mutator selector %q matches no registered mutator\n", selector)
		}
	}
	for _, invalid := range filter.InvalidSourceLinePatterns(opts.Config.IgnoreSourceLines) {
		fmt.Fprintf(w, "warning: invalid ignore_source_lines regex %q ignored: %v\n", invalid.Pattern, invalid.Err)
	}
}

func loadGitDiffLines(opts *models.Options) (gitdiff.ChangedLines, error) {
	if !opts.GitDiff.Lines {
		return nil, nil
	}
	base := opts.GitDiff.Base
	if base == "" {
		base = detectDefaultBranch()
	}
	lines, err := gitdiff.ParseChangedLines(base)
	if err != nil {
		return nil, err
	}
	console.Verbose(opts, "Git diff filter active against %q (%d changed files)", base, len(lines))
	return lines, nil
}

func detectDefaultBranch() string {
	out, err := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD").Output()
	if err == nil {
		ref := strings.TrimSpace(string(out))
		return strings.TrimPrefix(ref, "refs/remotes/")
	}
	return "master"
}

// loadTargetPackages resolves the directory the run's go commands start in and
// loads the target packages from it.
func loadTargetPackages(opts *models.Options, execs []string, files []string) (string, error) {
	goDir, err := resolveGoDir(opts, execs, files)
	if err != nil {
		return "", err
	}
	astutil.ClearIdentifierCache()
	parser.ClearPackageCache()
	if err := parser.PreparePackages(files, goDir); err != nil {
		return "", fmt.Errorf("Cannot load target packages: %w", err)
	}
	return goDir, nil
}

// resolveGoDir returns the directory the run's go commands start in. A target
// outside any single module is an error only when mutago itself runs go test.
func resolveGoDir(opts *models.Options, execs []string, files []string) (string, error) {
	goDir, err := goWorkDir(currentGoEnv(), files)
	if err == nil {
		return goDir, nil
	}
	if runsGoTest(opts, execs) {
		return "", fmt.Errorf("Cannot resolve the module of the targets: %w", err)
	}
	return "", nil
}

func detectModulePath(goDir string) string {
	out, err := goCommandIn(context.Background(), goDir, "list", "-m").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectModuleRoot(goDir string) string {
	out, err := goCommandIn(context.Background(), goDir, "env", "GOMOD").Output()
	if err != nil {
		return ""
	}
	gomod := strings.TrimSpace(string(out))
	if gomod == "" || gomod == os.DevNull {
		return ""
	}
	return filepath.Dir(gomod)
}

func sumDryRunTotals(totals map[string]int) int {
	var total int
	for _, count := range totals {
		total += count
	}
	return total
}

func mutateAll(r *mutationRun, pkgs []importing.Package, coverageProfiles []*coverage.Profile) (dryRunTotal int, dryRunMutatorTotals map[string]int, exitCode int) {
	if r.opts.General.DryRun {
		dryRunMutatorTotals = make(map[string]int)
	}
	for i, importPkg := range pkgs {
		if r.ctx.Err() != nil {
			return 0, dryRunMutatorTotals, returnError
		}
		var coverProfile *coverage.Profile
		if coverageProfiles != nil {
			coverProfile = coverageProfiles[i]
		}
		perTestProf := perTestForPackage(r, importPkg)
		testPkg := r.exec.importPaths.forFiles(importPkg.Files)
		for _, file := range importPkg.Files {
			if r.ctx.Err() != nil {
				return 0, dryRunMutatorTotals, returnError
			}
			if _, code := processFile(r, file, testPkg, coverProfile, perTestProf, dryRunMutatorTotals); code != 0 {
				return 0, dryRunMutatorTotals, code
			}
		}
	}
	return sumDryRunTotals(dryRunMutatorTotals), dryRunMutatorTotals, 0
}

func configureAdaptiveTimeoutAndCoverage(opts *models.Options, pkgs []importing.Package, run *mutationRun) ([]*coverage.Profile, error) {
	if opts.Exec.Coverage && !opts.Exec.NoExec && !opts.General.DryRun {
		profiles, maxBaseline, err := prepareCoverageProfiles(opts, pkgs, run.exec, run.tmpDir, run.modulePath, run.report)
		if err != nil {
			return nil, err
		}
		if len(run.exec.execs) == 0 {
			applyAdaptiveTimeoutFromBaseline(opts, maxBaseline)
		}
		return profiles, nil
	}
	return nil, nil
}

func prepareCoverageProfiles(opts *models.Options, pkgs []importing.Package, ec execConfig, tmpDir string, modulePath string, report *models.Report) ([]*coverage.Profile, time.Duration, error) {
	profiles := make([]*coverage.Profile, len(pkgs))
	var maxBaseline time.Duration
	for i, importPkg := range pkgs {
		profile, elapsed, err := buildCoverageProfile(opts, ec.importPaths.forFiles(importPkg.Files), ec.goDir, tmpDir, modulePath, ec.extraTestFlags)
		if err != nil {
			return nil, 0, err
		}
		profiles[i] = profile
		if profile != nil {
			report.HasCoverage = true
		}
		if elapsed > maxBaseline {
			maxBaseline = elapsed
		}
	}
	return profiles, maxBaseline, nil
}

func perTestForPackage(r *mutationRun, importPkg importing.Package) *coverage.PerTestProfile {
	if !r.opts.Exec.PerTest || r.opts.Exec.NoExec || r.opts.General.DryRun || len(r.exec.execs) != 0 {
		return nil
	}
	return buildPerTestCoverageProfile(r.stdout, r.opts, r.exec.importPaths.forFiles(importPkg.Files), r.modulePath, r.tmpDir, r.exec)
}

func processFile(r *mutationRun, file string, testPkg string, coverProfile *coverage.Profile, perTestProf *coverage.PerTestProfile, dryRunMutatorTotals map[string]int) (int, int) {
	console.Verbose(r.opts, "Mutate %q", file)

	annotationProcessor := annotation.NewProcessor()
	skipFilterProcessor := filter.NewSkipMakeArgsFilter()
	nonNegativeDecrementFilter := filter.NewNonNegativeDecrementFilter()
	sourceLineFilter := filter.NewSourceLineRegexFilter(r.opts.Config.IgnoreSourceLines)

	collectors := []filter.NodeCollector{annotationProcessor, skipFilterProcessor, nonNegativeDecrementFilter, sourceLineFilter}
	nodeFilters := []filter.NodeFilter{annotationProcessor, skipFilterProcessor, nonNegativeDecrementFilter, sourceLineFilter}

	checked, err := parser.ParseAndTypeCheckFile(file, collectors)
	if err != nil {
		return 0, returnError
	}
	src, fset, pkg, info := checked.File, checked.Fset, checked.Pkg, checked.Info
	originalSource, err := os.ReadFile(file)
	if err != nil {
		return 0, returnError
	}
	if !r.opts.General.DryRun {
		r.mu.Lock()
		if r.report.Sources == nil {
			r.report.Sources = make(map[string]string)
		}
		r.report.Sources[file] = string(originalSource)
		r.mu.Unlock()
	}

	if !r.opts.General.DryRun {
		if err := os.MkdirAll(r.tmpDir+"/"+filepath.Dir(file), 0755); err != nil {
			return 0, returnError
		}
		tmpFile := r.tmpDir + "/" + file
		originalFile := fmt.Sprintf("%s.original", tmpFile)
		if err := osutil.CopyFile(file, originalFile); err != nil {
			return 0, returnError
		}
		console.Debug(r.opts, "Save original into %q", originalFile)
	}

	tmpFile := r.tmpDir + "/" + file
	absFile, _ := filepath.Abs(file)

	fc := &fileContext{
		pkg:            pkg,
		testPkg:        testPkg,
		info:           info,
		fset:           fset,
		src:            src,
		sourceFile:     file,
		mutatedFile:    tmpFile,
		absFile:        absFile,
		coverProfile:   coverProfile,
		perTestProf:    perTestProf,
		filters:        nodeFilters,
		originalSource: originalSource,
	}

	return mutateFile(r, fc, dryRunMutatorTotals)
}

func mutateFile(r *mutationRun, fc *fileContext, dryRunMutatorTotals map[string]int) (int, int) {
	mutationID := 0

	if r.opts.Filter.Match == "" {
		mutationID = mutate(r, fc, fc.src, mutationID, dryRunMutatorTotals)
		return mutationID, 0
	}

	m, err := regexp.Compile(r.opts.Filter.Match)
	if err != nil {
		return 0, returnError
	}
	for _, f := range astutil.Functions(fc.src) {
		if m.MatchString(f.Name.Name) {
			mutationID = mutate(r, fc, f, mutationID, dryRunMutatorTotals)
		}
	}
	return mutationID, 0
}

func mutate(r *mutationRun, fc *fileContext, node ast.Node, mutationID int, dryRunGlobalTotals map[string]int) int {
	originalSourceCode := fc.originalSource

	var dryRunCounts map[string]int
	if r.opts.General.DryRun {
		dryRunCounts = make(map[string]int)
	}

	for _, m := range r.mutators {
		mutationID = applyMutator(r, m, fc, node, mutationID, originalSourceCode, dryRunCounts, dryRunGlobalTotals)
	}

	printDryRunFileSummary(r.opts, r.stdout, fc.sourceFile, dryRunCounts)

	return mutationID
}

func applyMutator(r *mutationRun, m mutatorItem, fc *fileContext, node ast.Node, mutationID int, originalSourceCode []byte, dryRunCounts, dryRunGlobalTotals map[string]int) int {
	console.Debug(r.opts, "Mutator %s", m.Name)

	prepareMutantIdentities(r, fc, m)
	mutatorAnnotated := annotation.DecoratorFilter(m.Mutator, m.Name, fc.filters...)
	changed := mutago.MutateWalkWithPositions(fc.pkg, fc.info, node, mutatorAnnotated)

	for {
		if r.ctx.Err() != nil {
			break
		}
		mutation, ok := <-changed
		if !ok {
			break
		}

		originalStartLine := int64(fc.fset.Position(mutation.Position).Line)
		recordOneMutation(r, m, fc, mutation, mutationID, originalStartLine, originalSourceCode, dryRunCounts, dryRunGlobalTotals)

		changed <- mutago.PositionedMutation{}
		<-changed
		changed <- mutago.PositionedMutation{}

		mutationID++
	}
	return mutationID
}

type preparedMutation struct {
	candidate scopeCandidate
	mutant    models.Mutant
	edit      mutationEdit
	relFile   string
}

func recordOneMutation(r *mutationRun, m mutatorItem, fc *fileContext, mutation mutago.PositionedMutation, mutationID int, originalStartLine int64, originalSourceCode []byte, dryRunCounts, dryRunGlobalTotals map[string]int) {
	prepared, ok := prepareScopedMutation(r, m, fc, mutation, originalStartLine, originalSourceCode)
	if !ok {
		return
	}
	admit, reason := r.scope.Admit(prepared.candidate)
	if !admit {
		noteRejectedMutation(r, prepared.candidate, reason)
		return
	}
	if r.opts.Exec.RunMutantID != "" && r.runMutantIDFound != nil {
		r.runMutantIDFound.Store(true)
	}
	if r.opts.General.DryRun {
		countDryRunMutation(m.Name, dryRunCounts, dryRunGlobalTotals)
		return
	}
	if r.jobs == nil {
		return
	}
	queueMutation(r, fc, mutation, mutationID, originalSourceCode, prepared)
}

func prepareScopedMutation(r *mutationRun, m mutatorItem, fc *fileContext, mutation mutago.PositionedMutation, originalStartLine int64, originalSourceCode []byte) (preparedMutation, bool) {
	mutant := models.Mutant{}
	mutant.Mutator.MutatorName = m.Name
	mutant.Mutator.OriginalFilePath = fc.sourceFile
	mutant.Mutator.OriginalStartLine = originalStartLine

	edit, err := captureMutationEdit(fc.fset, mutation.Node, mutation.Start, mutation.End, originalSourceCode)
	if err != nil {
		recordCaptureFailure(r, mutant, err)
		return preparedMutation{}, false
	}
	relFile := baseline.RelPath(fc.absFile, r.moduleRoot)
	checksum := stableMutationEditKey(relFile, originalSourceCode, edit)
	mutant.Checksum = checksum
	id, err := discoveredMutantID(r, fc, m, edit)
	if err != nil {
		recordCaptureFailure(r, mutant, err)
		return preparedMutation{}, false
	}
	mutant.ID = id
	return preparedMutation{
		candidate: scopeCandidate{
			relFile:  relFile,
			absFile:  fc.absFile,
			line:     int(originalStartLine),
			checksum: checksum,
			id:       id,
		},
		mutant:  mutant,
		edit:    edit,
		relFile: relFile,
	}, true
}

// needsMutantIDs reports whether discovery must assign stable IDs.
// A dry run needs them only to select --run-mutant-id.
func needsMutantIDs(opts *models.Options) bool {
	return !opts.General.DryRun || opts.Exec.RunMutantID != ""
}

// prepareMutantIdentities assigns IDs to every edit of mutator m in the file.
// It must run before the filtered walk starts, because a walk changes the AST
// in place while each mutation is pending.
func prepareMutantIdentities(r *mutationRun, fc *fileContext, m mutatorItem) {
	if !needsMutantIDs(r.opts) {
		return
	}
	if _, done := fc.identities[m.Name]; done {
		return
	}
	if fc.identities == nil {
		fc.identities = make(map[string]map[string]string)
	}
	relFile := baseline.RelPath(fc.absFile, r.moduleRoot)
	fc.identities[m.Name] = mutantIdentities(fc.pkg, fc.info, fc.fset, fc.src, fc.originalSource, relFile, m.Name, m.Mutator)
}

// discoveredMutantID returns the stable ID that prepareMutantIdentities gave edit.
func discoveredMutantID(r *mutationRun, fc *fileContext, m mutatorItem, edit mutationEdit) (string, error) {
	if !needsMutantIDs(r.opts) {
		return "", nil
	}
	if id, ok := fc.identities[m.Name][edit.key()]; ok {
		return id, nil
	}
	return "", fmt.Errorf("no stable ID for %s edit at bytes %d:%d", m.Name, edit.start, edit.end)
}

func recordCaptureFailure(r *mutationRun, mutant models.Mutant, err error) {
	if r.opts.General.DryRun {
		return
	}
	out := fmt.Sprintf("INTERNAL ERROR %s\n", err.Error())
	fmt.Fprintf(r.stdout, "%s", out)
	mutant.ProcessOutput = out
	r.mu.Lock()
	r.report.Errored = append(r.report.Errored, mutant)
	r.report.Stats.ErrorCount++
	r.mu.Unlock()
}

func noteRejectedMutation(r *mutationRun, c scopeCandidate, reason skipReason) {
	switch reason {
	case skipGitDiff:
		console.Debug(r.opts, "Skip %s:%d (not in git diff)", c.relFile, c.line)
	case skipMutantID:
		console.Debug(r.opts, "Skip %s:%d (mutant id not selected)", c.relFile, c.line)
	case skipDuplicate, skipBlacklist:
		if r.opts.General.DryRun {
			return
		}
		r.mu.Lock()
		r.report.Stats.DuplicatedCount++
		r.mu.Unlock()
	}
}

func queueMutation(r *mutationRun, fc *fileContext, mutation mutago.PositionedMutation, mutationID int, originalSourceCode []byte, prepared preparedMutation) {
	adjPos := fc.fset.PositionFor(mutation.Position, true)
	rawPos := fc.fset.PositionFor(mutation.Position, false)
	job := execJob{
		ctx:            r.ctx,
		opts:           r.opts,
		pkg:            fc.pkg,
		test:           goTestTarget{pkg: fc.testPkg, dir: r.exec.goDir},
		mutant:         prepared.mutant,
		coverProfile:   fc.coverProfile,
		execs:          r.exec.execs,
		perTestProf:    fc.perTestProf,
		extraTestFlags: r.exec.extraTestFlags,
		tmpDir:         r.tmpDir,
		out:            jobOutput{stdout: r.stdout, stderr: r.stderr},
		source: mutationSource{
			originalFile: fc.sourceFile,
			mutationFile: fmt.Sprintf("%s.%d", fc.mutatedFile, mutationID),
			absFile:      fc.absFile,
			relFile:      prepared.relFile,
			moduleRoot:   r.moduleRoot,
			original:     originalSourceCode,
			edit:         prepared.edit,
		},
		packageLevelDecl: isPackageLevelDecl(fc.src, mutation.Position),
		directiveShifted: adjPos.Filename != rawPos.Filename || adjPos.Line != rawPos.Line,
		adjRelFile:       baseline.RelPath(filepath.Join(r.moduleRoot, adjPos.Filename), r.moduleRoot),
	}
	select {
	case <-r.ctx.Done():
		return
	case r.jobs <- job:
	}
}

func countDryRunMutation(name string, dryRunCounts, dryRunGlobalTotals map[string]int) {
	dryRunCounts[name]++
	if dryRunGlobalTotals != nil {
		dryRunGlobalTotals[name]++
	}
}

func printDryRunFileSummary(opts *models.Options, stdout io.Writer, originalFile string, counts map[string]int) {
	if !opts.General.DryRun || len(counts) == 0 {
		return
	}
	fmt.Fprintf(stdout, "%s:\n", originalFile)
	var keys []string
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(stdout, "\t%s: %d\n", k, counts[k])
	}
}

func printDryRunReport(stdout io.Writer, total int, totals map[string]int) {
	if len(totals) > 0 {
		names := make([]string, 0, len(totals))
		for name := range totals {
			names = append(names, name)
		}
		sort.Strings(names)
		fmt.Fprintln(stdout, "\nPer-mutator totals across all files:")
		for _, name := range names {
			fmt.Fprintf(stdout, "  %-40s %d\n", name, totals[name])
		}
	}
	fmt.Fprintf(stdout, "\nTotal: %d mutation(s) would be generated. No files written, no tests run.\n", total)
	fmt.Fprintln(stdout, "Note: this count uses the same scope as a real run (changed lines, blacklist, duplicate edits, and --run-mutant-id). It does not apply coverage.")
}

func parseExecFlags(opts *models.Options) (execs []string, extraTestFlags []string) {
	if opts.Exec.Exec != "" {
		execs = strings.Fields(opts.Exec.Exec)
	}
	if opts.Exec.TestFlags != "" && len(execs) == 0 {
		extraTestFlags = strings.Fields(opts.Exec.TestFlags)
	}
	return
}

// runBaselineChecks runs `go test` once per package without any mutations
// applied. A package whose baseline does not pass (including a build/compile
// failure) is meaningless to mutate: `go test` exits 1 and mapTestExitToResult
// classifies that as KILLED, so every mutant is "killed" by the pre-existing
// failure and the run reports a false 100% MSI (see #85). Fail fast with a tool
// error instead.
//
// The check runs by default for the built-in exec path. It is skipped for
// --coverage (the coverage build itself is the baseline and is validated
// separately), --no-exec, --dry-run, and custom --exec (the built-in `go test`
// baseline does not reflect a custom exec command). The --noop flag is now a
// no-op retained for backward compatibility, since the check is always on.
// When --timeout-coefficient is set, the check uses a generous timeout, measures
// the clean run, and sets the per-mutation timeout from that measurement.
func skipBaselineChecks(opts *models.Options, execs []string) bool {
	return opts.Exec.Coverage || opts.Exec.NoExec || opts.General.DryRun || len(execs) > 0
}

// runsGoTest reports whether the run builds and tests the targets with
// `go test`: for the built-in runner, or for the --coverage profile.
func runsGoTest(opts *models.Options, execs []string) bool {
	return !opts.Exec.NoExec && !opts.General.DryRun && (len(execs) == 0 || opts.Exec.Coverage)
}

func runBaselineChecks(stderr io.Writer, opts *models.Options, pkgs []importing.Package, ec execConfig) int {
	if skipBaselineChecks(opts, ec.execs) {
		return 0 // returnOk
	}
	timeout := opts.Exec.Timeout
	flags := ec.extraTestFlags
	measureAdaptive := opts.Exec.TimeoutCoefficient > 0
	if measureAdaptive {
		timeout = adaptiveBaselineTimeoutSeconds
		flags = uncachedTestFlags(ec.extraTestFlags)
	}
	var maxBaseline time.Duration
	for _, importPkg := range pkgs {
		pkgPath := ec.importPaths.forFiles(importPkg.Files)
		if pkgPath == "" {
			fmt.Fprintf(stderr, "Cannot resolve the package of %q — mutago must test the package of the mutated files\n", importPkg.Files)
			return 3 // returnError
		}
		inv := goTestInvocation{
			kind:           baselineRun,
			target:         pkgPath,
			dir:            ec.goDir,
			recursive:      opts.Test.Recursive,
			timeoutSeconds: timeout,
			testFlags:      flags,
		}
		cmd := inv.command(context.Background())
		start := time.Now()
		out, err := cmd.CombinedOutput()
		elapsed := time.Since(start)
		if err != nil {
			fmt.Fprintf(stderr, "Baseline test failed for %q — mutation testing requires a green baseline; fix the build/tests before running mutago:\n%s\n", pkgPath, out)
			return 3 // returnError
		}
		if elapsed > maxBaseline {
			maxBaseline = elapsed
		}
	}
	if measureAdaptive {
		applyAdaptiveTimeoutFromBaseline(opts, maxBaseline)
	}
	console.Verbose(opts, "Baseline check passed — all packages green before mutation")
	return 0 // returnOk
}

func applyAdaptiveTimeoutFromBaseline(opts *models.Options, baseline time.Duration) {
	if opts.Exec.TimeoutCoefficient <= 0 || baseline <= 0 {
		return
	}
	derived := uint(math.Ceil(opts.Exec.TimeoutCoefficient * baseline.Seconds()))
	if derived < 1 {
		derived = 1
	}
	opts.Exec.Timeout = derived
	console.Verbose(opts, "Adaptive timeout: baseline %.2fs × %.1f = %ds",
		baseline.Seconds(), opts.Exec.TimeoutCoefficient, derived)
}

func buildCoverageProfile(opts *models.Options, pkgPath string, goDir string, tmpDir string, modulePath string, extraTestFlags []string) (*coverage.Profile, time.Duration, error) {
	if opts.Exec.NoExec || !opts.Exec.Coverage {
		return nil, 0, nil
	}
	if pkgPath == "" {
		return nil, 0, fmt.Errorf("cannot determine package path for coverage")
	}
	profileDir := filepath.Join(tmpDir, "coverage", filepath.FromSlash(pkgPath))
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return nil, 0, fmt.Errorf("cannot create coverage directory for %q: %w", pkgPath, err)
	}
	profilePath := filepath.Join(profileDir, "coverage.out")
	coverageTestFlags := extraTestFlags
	timeout := opts.Exec.Timeout
	if opts.Exec.TimeoutCoefficient > 0 && strings.TrimSpace(opts.Exec.Exec) == "" {
		coverageTestFlags = uncachedTestFlags(extraTestFlags)
		timeout = adaptiveBaselineTimeoutSeconds
	}
	start := time.Now()
	inv := goTestInvocation{
		kind:           coverageRun,
		target:         pkgPath,
		dir:            goDir,
		recursive:      opts.Test.Recursive,
		timeoutSeconds: timeout,
		testFlags:      coverageTestFlags,
		profilePath:    profilePath,
	}
	if err := runCoverageProfile(inv); err != nil {
		return nil, time.Since(start), err
	}
	elapsed := time.Since(start)
	prof, err := coverage.ParseProfile(profilePath, modulePath)
	if err != nil {
		return nil, elapsed, fmt.Errorf("cannot parse coverage profile for %q: %w", pkgPath, err)
	}
	return prof, elapsed, nil
}

func validateAdaptiveTimeoutTestCount(opts *models.Options) error {
	if opts.Exec.TimeoutCoefficient <= 0 || opts.Exec.NoExec || opts.General.DryRun || strings.TrimSpace(opts.Exec.Exec) != "" {
		return nil
	}
	testFlags := strings.Fields(opts.Exec.TestFlags)
	for i := range testFlags {
		value, found := testCountValue(testFlags, i)
		if !found {
			continue
		}
		count, err := strconv.Atoi(value)
		if err != nil || count > 0 {
			continue
		}
		return fmt.Errorf("adaptive timeout requires a positive test count, got %d", count)
	}
	return nil
}

func runCoverageProfile(inv goTestInvocation) error {
	cmd := inv.command(context.Background())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("coverage test failed for %q: %w\n%s", inv.target, err, out)
	}
	if _, err := os.Stat(inv.profilePath); err != nil {
		return fmt.Errorf("coverage profile not created for %q", inv.target)
	}
	return nil
}

// buildPerTestCoverageProfile maps each line of pkgPath to the tests that cover
// it. With --test-recursive the tests of every subpackage are profiled too, so
// the -run filter keeps them eligible just as the recursive mutant run does.
func buildPerTestCoverageProfile(stdout io.Writer, opts *models.Options, pkgPath string, modulePath string, tmpDir string, ec execConfig) *coverage.PerTestProfile {
	if pkgPath == "" {
		return nil
	}
	tc := perTestToolchain{testFlags: ec.extraTestFlags, timeoutSeconds: opts.Exec.Timeout, dir: ec.goDir}
	pkgs, err := coverage.ListTestPackages(tc, pkgPath, opts.Test.Recursive)
	if err != nil {
		console.Verbose(opts, "Per-test coverage unavailable for %q: %v", pkgPath, err)
		return nil
	}
	testCount := 0
	for _, pkg := range pkgs {
		testCount += len(pkg.Tests)
	}
	if testCount > 0 {
		fmt.Fprintf(stdout, "Building per-test coverage map for %q (%d tests)...\n", pkgPath, testCount)
	}
	prof, err := coverage.BuildPerTestProfileForPackages(tc, pkgPath, pkgs, modulePath, tmpDir, ec.numWorkers)
	if err != nil {
		console.Verbose(opts, "Per-test coverage unavailable for %q: %v", pkgPath, err)
		return nil
	}
	if prof != nil {
		console.Verbose(opts, "Per-test coverage map built for %q", pkgPath)
	}
	return prof
}

func calcNumWorkers(opts *models.Options, execs []string) int {
	n := opts.General.Workers
	if n <= 0 {
		n = runtime.NumCPU()
	}
	if len(execs) > 0 {
		n = 1
	}
	return n
}

func startWorkerPool(opts *models.Options, numWorkers int, report *models.Report, mu *sync.Mutex, tmpDir string) (chan execJob, *sync.WaitGroup) {
	if opts.Exec.NoExec || opts.General.DryRun {
		return nil, nil
	}
	stableExec := prepareStableExec(tmpDir)
	jobs := make(chan execJob, numWorkers*2)
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(paths stableExecPaths) {
			defer wg.Done()
			for job := range jobs {
				if job.ctx != nil && job.ctx.Err() != nil {
					continue
				}
				runExecJob(withStableExec(job, paths), report, mu)
			}
		}(stableExecPaths{wrapper: stableExec, bin: stableBinForWorker(tmpDir, stableExec, i)})
	}
	return jobs, &wg
}

func stableBinForWorker(tmpDir, wrapper string, id int) string {
	if wrapper == "" {
		return ""
	}
	return filepath.Join(tmpDir, fmt.Sprintf("stable-bin-%d", id))
}

func withStableExec(job execJob, paths stableExecPaths) execJob {
	if paths.wrapper == "" {
		return job
	}
	ctx := job.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	job.ctx = context.WithValue(ctx, stableExecKey{}, paths)
	return job
}

// stableExecKey carries the Darwin test-binary reuse paths for one worker.
// macOS re-checks each new executable path. Reusing one file avoids that wait.
type stableExecKey struct{}

type stableExecPaths struct {
	wrapper string
	bin     string
}

func stableExecFrom(ctx context.Context) stableExecPaths {
	if ctx == nil {
		return stableExecPaths{}
	}
	paths, _ := ctx.Value(stableExecKey{}).(stableExecPaths)
	return paths
}

// prepareStableExec installs the Darwin inode-reuse wrapper. Other platforms
// pay little to execute a new test binary, and the extra copy would be pure cost.
func prepareStableExec(tmpDir string) string {
	if runtime.GOOS != "darwin" || tmpDir == "" {
		return ""
	}
	path, err := writeStableExecWrapper(tmpDir)
	if err != nil {
		return ""
	}
	return path
}

// stableExecScript copies the just-linked test binary onto a reused file in
// MUTAGO_STABLE_DIR, then execs that file. The file name is the binary's base
// name, so a recursive `go test` can run rec.test and sub.test at the same time
// without one overwriting the other. The first run creates the file. Later runs
// of the same name truncate it in place. A copy failure falls back to the
// original path so the mutant is still executed.
const stableExecScript = `#!/bin/sh
src="$1"
shift
dir="${MUTAGO_STABLE_DIR-}"
unset MUTAGO_STABLE_DIR
if [ -n "$dir" ] && mkdir -p "$dir"; then
	stable="$dir/$(basename "$src")"
	if cat "$src" > "$stable" && chmod +x "$stable"; then
		exec "$stable" "$@"
	fi
fi
exec "$src" "$@"
`

func writeStableExecWrapper(dir string) (string, error) {
	path := filepath.Join(dir, "mutago-stable-exec.sh")
	if err := os.WriteFile(path, []byte(stableExecScript), 0755); err != nil {
		return "", err
	}
	if err := os.Chmod(path, 0755); err != nil {
		return "", err
	}
	return path, nil
}

func progressMonitorEnabled(opts *models.Options) bool {
	return isTerminal() && !opts.General.Verbose && !opts.General.Debug &&
		!opts.Config.SilentMode && !opts.Exec.NoExec && !opts.General.DryRun
}

func isTerminal() bool {
	fi, err := os.Stderr.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func startProgressMonitor(opts *models.Options, report *models.Report, mu *sync.Mutex, stderr io.Writer) (chan struct{}, *sync.WaitGroup) {
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				mu.Lock()
				total := report.Stats.KilledCount + report.Stats.EscapedCount + report.Stats.ErrorCount + report.Stats.SkippedCount + report.Stats.NotCoveredCount
				fmt.Fprintf(stderr, "\rProcessed %d mutants (%d killed, %d escaped, %d not covered, %d errored)",
					total, report.Stats.KilledCount, report.Stats.EscapedCount, report.Stats.NotCoveredCount, report.Stats.ErrorCount)
				mu.Unlock()
			case <-stop:
				fmt.Fprint(stderr, "\r\x1b[K") // clear the line
				return
			}
		}
	}()
	return stop, &wg
}

func shutdownAndCleanup(stderr io.Writer, opts *models.Options, jobs chan execJob, jobWg *sync.WaitGroup, stopProgress chan struct{}, progressWg *sync.WaitGroup, tmpDir string) {
	if jobs != nil {
		close(jobs)
		jobWg.Wait()
	}
	if stopProgress != nil {
		close(stopProgress)
		progressWg.Wait()
	}
	if opts.General.DoNotRemoveTmpFolder {
		return
	}
	if err := os.RemoveAll(tmpDir); err != nil {
		fmt.Fprintf(stderr, "mutago: cannot remove %s: %v\n", tmpDir, err)
		return
	}
	console.Debug(opts, "Remove %q", tmpDir)
}

func finalizeResults(stdout, stderr io.Writer, opts *models.Options, report *models.Report, bl *baseline.File, moduleRoot string, runMutantIDFound bool) int {
	if opts.Exec.RunMutantID != "" && !runMutantIDFound {
		fmt.Fprintf(stderr, "No mutant with ID %q was found\n", opts.Exec.RunMutantID)
		return returnError
	}

	if handled, code := handleBaselineUpdate(stdout, stderr, opts, report, moduleRoot); handled {
		return code
	}

	printResultsIfNeeded(stdout, opts, report)

	if code := writeAllReports(stderr, opts, report, moduleRoot); code != returnOk {
		return code
	}

	if opts.Exec.RunMutantID != "" {
		return returnOk
	}
	return checkQualityGates(stderr, opts, report, bl)
}

func handleBaselineUpdate(stdout, stderr io.Writer, opts *models.Options, report *models.Report, moduleRoot string) (bool, int) {
	if !opts.Baseline.Update {
		return false, 0
	}
	if err := baseline.Write(opts.Baseline.File, report.Escaped, moduleRoot); err != nil {
		_, _ = fmt.Fprintf(stderr, "Cannot write baseline: %v\n", err)
		return true, returnError
	}
	fmt.Fprintf(stdout, "Baseline written to %q (%d surviving mutant(s))\n", opts.Baseline.File, len(report.Escaped))
	return true, returnOk
}

func printResultsIfNeeded(stdout io.Writer, opts *models.Options, report *models.Report) {
	if opts.Exec.NoExec {
		fmt.Fprintln(stdout, "Cannot do a mutation testing summary since no exec command was executed.")
		return
	}
	if opts.Exec.RunMutantID == "" {
		printSummary(stdout, report)
	}
	if opts.Logger.GitHub {
		printGitHubAnnotations(stdout, report)
	}
}

func writeAllReports(stderr io.Writer, opts *models.Options, report *models.Report, moduleRoot string) int {
	specs := []reportSpec{
		{
			enabled:  opts.Config.JSONOutput,
			write:    func() error { return reportmaker.MakeJSONReport(*report) },
			savedMsg: "Save report into %q", fileName: models.ReportFileName,
		},
		{
			enabled:  opts.Logger.SummaryJSON,
			write:    func() error { return reportmaker.MakeSummaryJSONReport(report.Stats) },
			savedMsg: "Save summary into %q", fileName: models.ReportSummaryJSONFileName,
		},
		{
			enabled:  opts.Logger.AgenticJSON,
			write:    func() error { return reportmaker.MakeAgenticJSONReport(*report, moduleRoot) },
			savedMsg: "Save agentic report into %q", fileName: models.ReportAgenticJSONFileName,
		},
		{
			enabled:  opts.Logger.GitLab,
			write:    func() error { return reportmaker.MakeGitLabReport(*report, moduleRoot) },
			savedMsg: "Save GitLab report into %q", fileName: models.ReportGitLabJSONFileName,
		},
		{
			enabled:  opts.Config.HTMLOutput || opts.General.HTMLOutput,
			write:    func() error { return reportmaker.MakeHTMLReport(*report) },
			savedMsg: "Save report into %q", fileName: models.ReportHTMLFileName,
		},
	}
	for _, s := range specs {
		if code := writeReport(stderr, opts, s); code != returnOk {
			return code
		}
	}
	return returnOk
}

type reportSpec struct {
	enabled  bool
	write    func() error
	savedMsg string
	fileName string
}

func writeReport(stderr io.Writer, opts *models.Options, s reportSpec) int {
	if !s.enabled {
		return returnOk
	}
	if err := s.write(); err != nil {
		_, _ = fmt.Fprintf(stderr, "%s\n", err.Error())
		return returnError
	}
	console.Verbose(opts, s.savedMsg, s.fileName)
	return returnOk
}

func printSummary(stdout io.Writer, report *models.Report) {
	msiPct := report.Stats.Msi * 100
	covMsiPct := report.Stats.CoveredCodeMsi * 100
	fmt.Fprintf(stdout,
		"The mutation score is %.2f%% (%d killed, %d escaped, %d errored, %d not covered, %d skipped, %d total)\n",
		msiPct,
		report.Stats.KilledCount,
		report.Stats.EscapedCount,
		report.Stats.ErrorCount,
		report.Stats.NotCoveredCount,
		report.Stats.SkippedCount,
		report.Stats.TotalMutantsCount,
	)
	if report.HasCoverage {
		fmt.Fprintf(stdout, "The covered-code mutation score is %.2f%%\n", covMsiPct)
	}

	if len(report.MutatorStats) > 0 {
		fmt.Fprintln(stdout, "\nPer-mutator breakdown:")
		sorted := make([]models.MutatorStats, len(report.MutatorStats))
		copy(sorted, report.MutatorStats)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
		for _, ms := range sorted {
			killRate := 0.0
			if ms.Total > 0 {
				killRate = float64(ms.Killed) / float64(ms.Total) * 100
			}
			fmt.Fprintf(stdout, "  %-35s  killed %3d / %-3d  (%.0f%%)\n", ms.Name, ms.Killed, ms.Total, killRate)
		}
	}
}

func printGitHubAnnotations(stdout io.Writer, report *models.Report) {
	repoRoot := ""
	if out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
		repoRoot = strings.TrimSpace(string(out))
	}

	for _, m := range report.Escaped {
		// GitHub resolves annotation paths against the repository root, which
		// differs from the module root when the module lives in a subdirectory.
		filePath := filepath.ToSlash(m.Mutator.OriginalFilePath)
		if repoRoot != "" {
			filePath = baseline.RelPath(m.Mutator.OriginalFilePath, repoRoot)
		}
		fmt.Fprintf(stdout, "::warning file=%s,line=%d,title=Mutant escaped (%s)::Escaped mutation at %s:%d — add a test to kill it\n",
			filePath,
			m.Mutator.OriginalStartLine,
			m.Mutator.MutatorName,
			filePath,
			m.Mutator.OriginalStartLine,
		)
	}
}

func checkQualityGates(stderr io.Writer, opts *models.Options, report *models.Report, bl *baseline.File) int {
	if opts.Score.IgnoreMsiWithNoMutations && report.Stats.TotalMutantsCount == 0 {
		return returnOk
	}

	minMsi := resolveThreshold(opts.Score.MinMsi, opts.Config.MinMsi)
	minCoveredMsi := resolveThreshold(opts.Score.MinCoveredMsi, opts.Config.MinCoveredMsi)

	escapedFail := checkEscapedGate(stderr, opts, report, bl)
	msiFail := checkMsiGate(stderr, report, minMsi)
	coveredFail := checkCoveredMsiGate(stderr, report, minCoveredMsi)

	if escapedFail || msiFail || coveredFail {
		return returnMsiThresholdNotMet
	}
	return returnOk
}

func resolveThreshold(cliValue, configValue float64) float64 {
	if cliValue < 0 {
		return configValue
	}
	return cliValue
}

func checkEscapedGate(stderr io.Writer, opts *models.Options, report *models.Report, bl *baseline.File) bool {
	if !opts.Score.FailOnEscaped {
		return false
	}
	newEscapes := bl.NewEscapes(report.Escaped)
	if len(newEscapes) == 0 {
		return false
	}
	qualifier := ""
	if bl != nil {
		qualifier = "new "
	}
	fmt.Fprintf(stderr, "%d %smutant(s) escaped — kill them or run --update-baseline to accept\n", len(newEscapes), qualifier)
	return true
}

const msiEpsilon = 1e-9

func checkMsiGate(stderr io.Writer, report *models.Report, minMsi float64) bool {
	msiPct := report.Stats.Msi * 100
	if minMsi >= 0 && (minMsi-msiPct) > msiEpsilon {
		fmt.Fprintf(stderr, "MSI %.2f%% is below minimum required %.2f%%\n", msiPct, minMsi)
		return true
	}
	return false
}

func checkCoveredMsiGate(stderr io.Writer, report *models.Report, minCoveredMsi float64) bool {
	if minCoveredMsi <= 0 {
		return false
	}
	if !report.HasCoverage {
		fmt.Fprintf(stderr, "Covered MSI cannot be checked: --coverage was not enabled (score is always 0 without a profile)\n")
		return true
	}
	covMsiPct := report.Stats.CoveredCodeMsi * 100
	if (minCoveredMsi - covMsiPct) > msiEpsilon {
		fmt.Fprintf(stderr, "Covered MSI %.2f%% is below minimum required %.2f%%\n", covMsiPct, minCoveredMsi)
		return true
	}
	return false
}

func saveAST(mutationBlackList map[string]struct{}, file string, fset *token.FileSet, node ast.Node, fmtOriginal []byte) (string, bool, error) {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return "", false, err
	}
	mutatedSrc, err := format.Source(buf.Bytes())
	if err != nil {
		return "", false, err
	}
	checksum := stableMutationKey(fmtOriginal, mutatedSrc)
	if _, ok := mutationBlackList[checksum]; ok {
		return checksum, true, nil
	}
	mutationBlackList[checksum] = struct{}{}
	return checksum, false, os.WriteFile(file, mutatedSrc, 0666)
}

type mutationEdit struct {
	start       int
	end         int
	replacement []byte
}

func captureMutationEdit(fset *token.FileSet, node ast.Node, startPos, endPos token.Pos, original []byte) (mutationEdit, error) {
	start := fset.PositionFor(startPos, false).Offset
	end := fset.PositionFor(endPos, false).Offset
	if start < 0 || end < start || end > len(original) {
		return mutationEdit{}, fmt.Errorf("invalid mutation range %d:%d for %d-byte source", start, end, len(original))
	}
	var replacement bytes.Buffer
	if err := printer.Fprint(&replacement, fset, node); err != nil {
		return mutationEdit{}, err
	}
	return mutationEdit{start: start, end: end, replacement: replacement.Bytes()}, nil
}

func (e mutationEdit) materialize(original []byte) ([]byte, error) {
	if e.start < 0 || e.end < e.start || e.end > len(original) {
		return nil, fmt.Errorf("invalid mutation range %d:%d for %d-byte source", e.start, e.end, len(original))
	}
	mutated := make([]byte, 0, len(original)-(e.end-e.start)+len(e.replacement))
	mutated = append(mutated, original[:e.start]...)
	mutated = append(mutated, e.replacement...)
	mutated = append(mutated, original[e.end:]...)
	return mutated, nil
}

// stableMutationEditKey hashes the file identity, the edit's byte offsets, the
// original text and the replacement, so two mutants are deduplicated only when
// they produce byte-identical edits at the same location in the same file
// (#103). Keys are 32-char MD5 hex strings to stay compatible with the
// --blacklist file format.
func stableMutationEditKey(file string, original []byte, edit mutationEdit) string {
	h := md5.New()
	fmt.Fprintf(h, "%s\x00%d\x00", file, edit.start)
	h.Write(original[edit.start:edit.end])
	h.Write([]byte{0})
	h.Write(edit.replacement)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func stableMutationKey(original, mutated []byte) string {
	h := md5.New()
	oLines := strings.Split(string(original), "\n")
	mLines := strings.Split(string(mutated), "\n")
	n := len(oLines)
	if len(mLines) > n {
		n = len(mLines)
	}
	for i := 0; i < n; i++ {
		o, m := "", ""
		if i < len(oLines) {
			o = oLines[i]
		}
		if i < len(mLines) {
			m = mLines[i]
		}
		if o != m {
			fmt.Fprintf(h, "-%s\n+%s\n", o, m)
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// mutationCovered reports whether the mutation at startLine is reached by any
// test, accounting for //line directives. The primary lookup uses the physical
// file path (the common case, where the adjusted line equals the raw line). For
// directive-shifted positions Go's coverage profile files the block under the
// directive's filename (or "." for a filename-less directive), so a second
// lookup uses the directive-adjusted filename. A third, line-only fallback
// handles the rare case where a single coverage block spans a //line filename
// change and is filed under a third filename; it is gated on directiveShifted
// so it never applies to ordinary positions (see #84).
func mutationCovered(job execJob, startLine int) bool {
	if job.coverProfile.IsCovered(job.source.absFile, startLine) {
		return true
	}
	if !job.directiveShifted || job.adjRelFile == "" {
		return false
	}
	if job.coverProfile.IsCoveredRelative(job.adjRelFile, startLine) {
		return true
	}
	return job.coverProfile.IsLineCoveredAnywhere(startLine)
}

// isPackageLevelDecl reports whether pos lies within a package-level
// GenDecl (const, var, type, import) in file. Declarations at package scope are
// not executable statements, so `go test` coverage profiles never record them as
// covered. The --coverage filter must therefore not skip mutations at such
// positions, even when the profile has no entry for the line (see #83).
func isPackageLevelDecl(file ast.Node, pos token.Pos) bool {
	f, ok := file.(*ast.File)
	if !ok {
		return false
	}
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if pos >= gen.Pos() && pos <= gen.End() {
			return true
		}
	}
	return false
}

func runExecJob(job execJob, stats *models.Report, mu *sync.Mutex) {
	opts := job.opts
	mutant := job.mutant

	startLine := mutant.Mutator.OriginalStartLine
	notCovered := !job.packageLevelDecl && job.coverProfile != nil && startLine > 0 && !mutationCovered(job, int(startLine))
	if notCovered {
		mu.Lock()
		defer mu.Unlock()
		recordNotCovered(opts, stats, mutant, mutantLocation(opts, mutant))
		return
	}

	mutatedSourceCode, err := job.source.edit.materialize(job.source.original)
	if err == nil {
		err = os.WriteFile(job.source.mutationFile, mutatedSourceCode, 0666)
	}
	if err != nil {
		out := fmt.Sprintf("INTERNAL ERROR %s\n", err.Error())
		fmt.Fprint(job.out.stdout, out)
		mutant.ProcessOutput = out
		mu.Lock()
		stats.Errored = append(stats.Errored, mutant)
		stats.Stats.ErrorCount++
		mu.Unlock()
		return
	}

	execExitCode := mutateExec(job, &mutant)
	console.Debug(opts, "Exited with %d", execExitCode)

	mu.Lock()
	defer mu.Unlock()
	recordMutantResult(job.out.stdout, opts, stats, mutant, execExitCode, mutantLocation(opts, mutant))
}

func mutantLocation(opts *models.Options, mutant models.Mutant) string {
	// Console locations are for the user at the terminal, so they stay
	// relative to the working directory rather than using the module-root
	// identity path from baseline.RelPath.
	loc := mutant.Mutator.OriginalFilePath
	if rel, err := filepath.Rel(".", loc); err == nil {
		loc = filepath.ToSlash(rel)
	}
	if mutant.Mutator.OriginalStartLine > 0 {
		loc = fmt.Sprintf("%s:%d", loc, mutant.Mutator.OriginalStartLine)
	}
	if (opts.General.Debug || opts.General.Verbose) && mutant.Checksum != "" {
		return fmt.Sprintf("%s (%s) [checksum: %s]", loc, mutant.Mutator.MutatorName, mutant.Checksum)
	}
	return fmt.Sprintf("%s (%s)", loc, mutant.Mutator.MutatorName)
}

func mutateExec(job execJob, mutant *models.Mutant) int {
	if len(job.execs) == 0 {
		return runBuiltinExec(job, mutant)
	}
	return runCustomExec(job, mutant)
}

func runBuiltinExec(job execJob, mutant *models.Mutant) int {
	opts := job.opts
	console.Debug(opts, "Execute built-in exec command for mutation")

	diff, code := computeDiff(job.ctx, job.out.stderr, job.source.originalFile, job.source.mutationFile, mutant)
	if code != 0 {
		return code
	}

	overlayName, code := prepareOverlay(job.out.stderr, job.tmpDir, job.source.originalFile, job.source.mutationFile)
	if code != 0 {
		return code
	}
	defer os.Remove(overlayName)

	execExitCode := runGoTest(job, overlayName, int(mutant.Mutator.OriginalStartLine))

	mutant.Diff = string(diff)
	return mapTestExitToResult(execExitCode)
}

func computeDiff(ctx context.Context, stderr io.Writer, file, mutationFile string, mutant *models.Mutant) ([]byte, int) {
	if ctx == nil {
		ctx = context.Background()
	}
	diff, err := exec.CommandContext(ctx, "diff", "--label=Original", "--label=New", "-u", file, mutationFile).CombinedOutput()
	if mutant.Mutator.OriginalStartLine <= 0 {
		mutant.Mutator.OriginalStartLine = parser.FindOriginalStartLine(diff)
	}

	diffExitCode, ok := commandExitCode(err)
	if !ok {
		fmt.Fprintf(stderr, "mutago: diff error: %v\n", err)
		return nil, 3
	}
	if diffExitCode != 0 && diffExitCode != 1 {
		fmt.Fprintf(stderr, "mutago: diff exited with code %d\n", diffExitCode)
		return nil, 3
	}
	return diff, 0
}

func prepareOverlay(stderr io.Writer, tmpDir, file, mutationFile string) (string, int) {
	absOrig, _ := filepath.Abs(file)
	absMut, _ := filepath.Abs(mutationFile)
	overlayName, err := writeOverlayFile(tmpDir, absOrig, absMut)
	if err != nil {
		fmt.Fprintf(stderr, "mutago: cannot create overlay file: %v\n", err)
		return "", 3
	}
	return overlayName, 0
}

func runGoTest(job execJob, overlayName string, startLine int) int {
	ctx, opts := job.ctx, job.opts
	if job.test.pkg == "" {
		fmt.Fprintf(job.out.stderr, "mutago: cannot resolve the package of %q to test\n", job.source.originalFile)
		return 3
	}
	inv := goTestInvocation{
		kind:           mutantRun,
		target:         job.test.pkg,
		dir:            job.test.dir,
		recursive:      opts.Test.Recursive,
		timeoutSeconds: opts.Exec.Timeout,
		testFlags:      job.extraTestFlags,
		overlay:        overlayName,
		runFilter:      perTestRunFilter(job.perTestProf, job.source.absFile, startLine),
	}

	if ctx == nil {
		ctx = context.Background()
	}
	stable := stableExecFrom(ctx)
	inv.execProgram = stable.wrapper
	goTestCmd := inv.command(ctx)
	if stable.wrapper != "" && stable.bin != "" && !hasTestFlag(job.extraTestFlags, "exec") {
		goTestCmd.Env = append(goTestCmd.Env, "MUTAGO_STABLE_DIR="+stable.bin)
	}
	test, err := goTestCmd.CombinedOutput()

	execExitCode, ok := commandExitCode(err)
	if !ok {
		fmt.Fprintf(job.out.stderr, "mutago: go test error: %v\n", err)
		return 3
	}

	if opts.General.Debug {
		fmt.Fprintf(job.out.stdout, "%s\n", test)
	}
	return classifyGoTestResult(execExitCode, test)
}

// classifyGoTestResult distinguishes a test failure from a package build or
// setup failure. All make `go test` exit 1, but a mutant that never compiled
// or whose package failed setup was not killed by a test and must be skipped
// instead.
func classifyGoTestResult(execExitCode int, output []byte) int {
	if execExitCode == 1 && (bytes.Contains(output, []byte("[build failed]")) || bytes.Contains(output, []byte("[setup failed]"))) {
		return 2
	}
	return execExitCode
}

func runCustomExec(job execJob, mutant *models.Mutant) int {
	ctx, opts := job.ctx, job.opts
	file, mutationFile := job.source.originalFile, job.source.mutationFile
	console.Debug(opts, "Execute %q for mutation", opts.Exec.Exec)

	extDiff, _ := exec.Command("diff", "--label=Original", "--label=New", "-u", file, mutationFile).CombinedOutput()
	if mutant.Mutator.OriginalStartLine <= 0 {
		mutant.Mutator.OriginalStartLine = parser.FindOriginalStartLine(extDiff)
	}
	mutant.Diff = string(extDiff)

	if ctx == nil {
		ctx = context.Background()
	}
	if opts.Exec.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(opts.Exec.Timeout)*time.Second)
		defer cancel()
	}
	execCommand := exec.CommandContext(ctx, job.execs[0], job.execs[1:]...)
	execCommand.Stderr = job.out.stderr
	execCommand.Stdout = job.out.stdout
	execCommand.Env = append(os.Environ(), []string{
		"MUTATE_CHANGED=" + mutationFile,
		fmt.Sprintf("MUTATE_DEBUG=%t", opts.General.Debug),
		"MUTATE_ORIGINAL=" + file,
		"MUTATE_PACKAGE=" + job.pkg.Path(),
		fmt.Sprintf("MUTATE_TIMEOUT=%d", opts.Exec.Timeout),
		fmt.Sprintf("MUTATE_VERBOSE=%t", opts.General.Verbose),
	}...)
	if opts.Test.Recursive {
		execCommand.Env = append(execCommand.Env, "TEST_RECURSIVE=true")
	}

	if err := execCommand.Start(); err != nil {
		fmt.Fprintf(job.out.stderr, "mutago: custom exec failed to start: %v\n", err)
		return 3
	}

	err := execCommand.Wait()
	if ctx.Err() != nil {
		fmt.Fprintf(job.out.stderr, "mutago: custom exec timed out or was cancelled: %v\n", ctx.Err())
		return 3
	}
	execExitCode, ok := commandExitCode(err)
	if !ok {
		fmt.Fprintf(job.out.stderr, "mutago: custom exec wait error: %v\n", err)
		return 3
	}
	return execExitCode
}

func commandExitCode(err error) (code int, ok bool) {
	if err == nil {
		return 0, true
	}
	if e, isExit := err.(*exec.ExitError); isExit {
		return e.Sys().(syscall.WaitStatus).ExitStatus(), true
	}
	return 0, false
}

func writeOverlayFile(tmpDir, absOrig, absMut string) (string, error) {
	overlayData, err := json.Marshal(struct {
		Replace map[string]string `json:"Replace"`
	}{Replace: map[string]string{absOrig: absMut}})
	if err != nil {
		return "", err
	}

	f, err := os.CreateTemp(tmpDir, "mutago-overlay-*.json")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(overlayData); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), f.Close()
}

func perTestRunFilter(perTestProf *coverage.PerTestProfile, absFile string, startLine int) string {
	if perTestProf == nil || startLine <= 0 {
		return ""
	}
	tests := perTestProf.CoveringTests(absFile, startLine)
	if len(tests) == 0 {
		return ""
	}
	return "^(" + strings.Join(tests, "|") + ")$"
}

func mapTestExitToResult(execExitCode int) int {
	switch execExitCode {
	case 0:
		return 1
	case 1:
		return 0
	}
	return execExitCode
}

func statusVisible(opts *models.Options, letter byte) bool {
	if opts.Config.SilentMode {
		return false
	}
	if opts.General.OutputStatuses != "" {
		return strings.IndexByte(opts.General.OutputStatuses, letter) >= 0
	}
	if opts.General.Quiet {
		return letter == 'e'
	}
	return true
}

func recordMutantResult(stdout io.Writer, opts *models.Options, stats *models.Report, mutant models.Mutant, execExitCode int, msg string) {
	switch execExitCode {
	case 0:
		recordKilled(opts, stats, mutant, msg)
	case 1:
		recordEscaped(opts, stats, mutant, msg)
	case 2:
		recordSkipped(stdout, opts, stats, mutant, msg)
	default:
		recordErrored(opts, stats, mutant, msg)
	}
}

func recordNotCovered(opts *models.Options, stats *models.Report, mutant models.Mutant, msg string) {
	out := fmt.Sprintf("NOT COVERED %s\n", msg)
	if statusVisible(opts, 'n') {
		console.PrintSkip(out)
	}
	mutant.ProcessOutput = out
	stats.NotCovered = append(stats.NotCovered, mutant)
	stats.Stats.NotCoveredCount++
}

func recordKilled(opts *models.Options, stats *models.Report, mutant models.Mutant, msg string) {
	out := fmt.Sprintf("%s %s\n", console.KILLED, msg)
	if statusVisible(opts, 'k') {
		console.PrintKilled(out)
	}
	if opts.General.Debug && !opts.General.NoDiffs && mutant.Diff != "" {
		console.PrintDiff([]byte(mutant.Diff))
	}
	mutant.ProcessOutput = out
	stats.Killed = append(stats.Killed, mutant)
	stats.Stats.KilledCount++
}

func recordEscaped(opts *models.Options, stats *models.Report, mutant models.Mutant, msg string) {
	out := fmt.Sprintf("%s %s\n", console.ESCAPED, msg)
	if statusVisible(opts, 'e') {
		console.PrintEscaped(out)
	}
	if !opts.General.NoDiffs && statusVisible(opts, 'e') && mutant.Diff != "" {
		console.PrintDiff([]byte(mutant.Diff))
	}
	mutant.ProcessOutput = out
	stats.Escaped = append(stats.Escaped, mutant)
	stats.Stats.EscapedCount++
}

func recordSkipped(stdout io.Writer, opts *models.Options, stats *models.Report, mutant models.Mutant, msg string) {
	out := fmt.Sprintf("SKIP %s\n", msg)
	if statusVisible(opts, 's') {
		console.PrintSkip(out)
	}
	if opts.General.Verbose {
		fmt.Fprintln(stdout, "Mutation did not compile")
	}
	if opts.General.Debug && !opts.General.NoDiffs && mutant.Diff != "" {
		console.PrintDiff([]byte(mutant.Diff))
	}
	mutant.ProcessOutput = out
	stats.Skipped = append(stats.Skipped, mutant)
	stats.Stats.SkippedCount++
}

func recordErrored(opts *models.Options, stats *models.Report, mutant models.Mutant, msg string) {
	out := fmt.Sprintf("UNKNOWN exit code for %s\n", msg)
	if statusVisible(opts, 'x') {
		console.PrintUnknown(out)
		if !opts.General.NoDiffs && mutant.Diff != "" {
			console.PrintDiff([]byte(mutant.Diff))
		}
	}
	mutant.ProcessOutput = out
	stats.Errored = append(stats.Errored, mutant)
	stats.Stats.ErrorCount++
}

func loadBlacklist(files []string) (map[string]struct{}, error) {
	bl := map[string]struct{}{}
	for _, f := range files {
		c, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("cannot read blacklist file %q: %w", f, err)
		}
		for _, line := range strings.Split(string(c), "\n") {
			if line == "" {
				continue
			}
			if len(line) != 32 {
				return nil, fmt.Errorf("%q is not a MD5 checksum", line)
			}
			bl[line] = struct{}{}
		}
	}
	return bl, nil
}
