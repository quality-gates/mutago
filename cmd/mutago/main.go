package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/printer"
	"go/token"
	"go/types"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jessevdk/go-flags"
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
	"github.com/zimmski/osutil"

	"github.com/quality-gates/mutago/v2"
	"github.com/quality-gates/mutago/v2/astutil"
	"github.com/quality-gates/mutago/v2/mutator"
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

const (
	returnOk = iota
	returnHelp
	returnBashCompletion
	returnError
	returnMsiThresholdNotMet // exit 4: quality gate failed
)

// isTerminal reports whether stderr is an interactive terminal.
func isTerminal() bool {
	fi, err := os.Stderr.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// statusVisible reports whether a given result status should be printed to the
// terminal.  letter is one of: k=killed e=escaped s=skipped n=not-covered
// x=errored.  --output-statuses takes precedence; --quiet falls back to showing
// only escaped mutants.  Silent mode is handled separately by the caller.
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

// matchesMutator reports whether pattern matches the mutator name.
// A trailing * is a prefix wildcard: "arithmetic/*" matches any name starting
// with "arithmetic/". A bare "*" matches everything. Exact names match literally.
func matchesMutator(pattern, name string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(name, strings.TrimSuffix(pattern, "*"))
	}
	return name == pattern
}

func checkArguments(args []string, opts *models.Options) (bool, int) {
	p := flags.NewNamedParser("mutago", flags.None)

	p.ShortDescription = "Mutation testing for Go source code"

	if _, err := p.AddGroup("mutago", "mutago arguments", opts); err != nil {
		return true, exitError(err.Error())
	}

	_, err := p.ParseArgs(args)

	// --help and --list-mutators print and exit before any parse error is reported.
	if handled, code := handleEarlyExitFlags(opts, p, args); handled {
		return true, code
	}

	if err != nil {
		return true, exitError(err.Error())
	}

	if isCompletion() {
		return true, returnBashCompletion
	}

	if opts.General.Debug {
		opts.General.Verbose = true
	}

	if handled, code := loadConfigFile(opts); handled {
		return true, code
	}

	return false, 0
}

// isCompletion reports whether mutago was invoked for shell completion.
func isCompletion() bool {
	return len(os.Getenv("GO_FLAGS_COMPLETION")) > 0
}

// handleEarlyExitFlags handles --help and --list-mutators, which print and exit
// before any mutation work. Returns (true, exitCode) when one applied.
func handleEarlyExitFlags(opts *models.Options, p *flags.Parser, args []string) (bool, int) {
	if (opts.General.Help || len(args) == 0) && !isCompletion() {
		p.WriteHelp(os.Stdout)

		return true, returnOk // exit 0 is conventional for --help
	}
	if opts.Mutator.ListMutators {
		for _, name := range mutator.List() {
			fmt.Println(name)
		}

		return true, returnOk
	}
	return false, 0
}

// loadConfigFile merges a YAML config file into opts when --config is set.
// Returns (true, exitCode) on failure, (false, 0) otherwise.
func loadConfigFile(opts *models.Options) (bool, int) {
	if opts.General.Config == "" {
		return false, 0
	}
	yamlFile, err := os.ReadFile(opts.General.Config)
	if err != nil {
		return true, exitError("Could not read config file: %q", opts.General.Config)
	}
	dec := yaml.NewDecoder(bytes.NewReader(yamlFile))
	dec.KnownFields(true)
	if err := dec.Decode(&opts.Config); err != nil {
		return true, exitError("Could not parse config file %q: %v", opts.General.Config, err)
	}
	return false, 0
}

func exitError(format string, args ...interface{}) int {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)

	return returnError
}

type mutatorItem struct {
	Name    string
	Mutator mutator.Mutator
}

// execJob carries everything a parallel worker needs to test one mutation.
type execJob struct {
	opts            *models.Options
	pkg             *types.Package
	originalFile    string
	mutationFile    string
	mutant          models.Mutant
	absFile         string
	coverProfile    *coverage.Profile
	gitChangedLines gitdiff.ChangedLines
	execs           []string
	perTestProf     *coverage.PerTestProfile
	extraTestFlags  []string
	runMutantID     string // when set, skip all mutants whose computed ID != this value
	moduleRoot      string // used to compute relative file path for ID matching
}

// mutationRun holds the run-wide configuration shared across every package and
// file mutated in a single mutago invocation.
type mutationRun struct {
	opts            *models.Options
	mutators        []mutatorItem
	blacklist       map[string]struct{}
	tmpDir          string
	numWorkers      int
	execs           []string
	extraTestFlags  []string
	report          *models.Report
	mu              *sync.Mutex
	modulePath      string
	moduleRoot      string
	gitChangedLines gitdiff.ChangedLines
	jobs            chan<- execJob
}

// fileContext holds the parsed state and per-file derived values for the file
// currently being mutated.
type fileContext struct {
	pkg          *types.Package
	info         *types.Info
	fset         *token.FileSet
	src          ast.Node
	sourceFile   string
	mutatedFile  string
	absFile      string
	coverProfile *coverage.Profile
	perTestProf  *coverage.PerTestProfile
	filters      []filter.NodeFilter
}

func mainCmd(args []string) int {
	opts := &models.Options{}

	if exit, exitCode := checkArguments(args, opts); exit {
		return exitCode
	}

	files := importing.FilesOfArgs(opts.Remaining.Targets, opts)
	if len(files) == 0 {
		return exitError("Could not find any suitable Go source files")
	}

	bl, err := baseline.Load(opts.Baseline.File)
	if err != nil {
		return exitError("Cannot load baseline: %v", err)
	}

	if handled, code := handleInfoFlags(opts, files); handled {
		return code
	}

	return runMutationTesting(opts, bl)
}

// runMutationTesting performs the full mutation run: setup, worker pool,
// mutation dispatch, shutdown, and result finalization.
func runMutationTesting(opts *models.Options, bl *baseline.File) int {
	mutationBlackList, err := loadBlacklist(opts.Files.Blacklist)
	if err != nil {
		return exitError(err.Error())
	}

	tmpDir, err := os.MkdirTemp("", "mutago-")
	if err != nil {
		return exitError("Cannot create temp directory: %v", err)
	}
	console.Verbose(opts, "Save mutations into %q", tmpDir)

	report := &models.Report{}
	var reportMu sync.Mutex
	moduleRoot := detectModuleRoot()

	gitChangedLines, err := loadGitDiffLines(opts)
	if err != nil {
		return exitError("Cannot load git diff: %v", err)
	}

	execs, extraTestFlags := parseExecFlags(opts)
	pkgs := importing.PackagesWithFilesOfArgs(opts.Remaining.Targets, opts)

	if exitCode := runNoopChecks(opts, pkgs, execs, extraTestFlags); exitCode != 0 {
		return exitCode
	}

	applyAdaptiveTimeout(opts, pkgs, execs, extraTestFlags)

	numWorkers := calcNumWorkers(opts, execs)
	console.Verbose(opts, "Running with %d parallel worker(s)", numWorkers)

	jobs, jobWg := startWorkerPool(opts, numWorkers, report, &reportMu)
	stopProgress, progressWg := startProgressMonitor(opts, report, &reportMu)

	run := &mutationRun{
		opts:            opts,
		mutators:        buildActiveMutators(opts),
		blacklist:       mutationBlackList,
		tmpDir:          tmpDir,
		numWorkers:      numWorkers,
		execs:           execs,
		extraTestFlags:  extraTestFlags,
		report:          report,
		mu:              &reportMu,
		modulePath:      detectModulePath(),
		moduleRoot:      moduleRoot,
		gitChangedLines: gitChangedLines,
		jobs:            jobs,
	}

	dryRunTotal, dryRunMutatorTotals, loopCode := run.mutateAll(pkgs)

	shutdownAndCleanup(opts, jobs, jobWg, stopProgress, progressWg, tmpDir)

	if loopCode != returnOk {
		return loopCode
	}

	if opts.General.DryRun {
		printDryRunReport(dryRunTotal, dryRunMutatorTotals)
		return returnOk
	}

	report.Calculate()
	return finalizeResults(opts, report, bl, moduleRoot)
}

// finalizeResults writes reports, prints the summary, and evaluates the quality
// gates after a completed (non-dry) mutation run.
func finalizeResults(opts *models.Options, report *models.Report, bl *baseline.File, moduleRoot string) int {
	if handled, code := handleBaselineUpdate(opts, report, moduleRoot); handled {
		return code
	}

	printResultsIfNeeded(opts, report)

	if code := writeAllReports(opts, report, moduleRoot); code != returnOk {
		return code
	}

	if opts.Exec.RunMutantID != "" {
		return returnOk
	}
	return checkQualityGates(opts, report, bl, moduleRoot)
}

// handleInfoFlags handles --list-files and --print-ast early-exit flags.
func handleInfoFlags(opts *models.Options, files []string) (bool, int) {
	if opts.Files.ListFiles {
		for _, file := range files {
			fmt.Println(file)
		}
		return true, returnOk
	}
	if opts.Files.PrintAST {
		for _, file := range files {
			fmt.Println(file)
			src, _, err := parser.ParseFile(file)
			if err != nil {
				return true, exitError("Could not open file %q: %v", file, err)
			}
			mutago.PrintWalk(src)
			fmt.Println()
		}
		return true, returnOk
	}
	return false, 0
}

// parseExecFlags returns the exec command fields and any extra test flags.
func parseExecFlags(opts *models.Options) (execs []string, extraTestFlags []string) {
	if opts.Exec.Exec != "" {
		execs = strings.Fields(opts.Exec.Exec)
	}
	if opts.Exec.TestFlags != "" && len(execs) == 0 {
		extraTestFlags = strings.Fields(opts.Exec.TestFlags)
	}
	return
}

// loadBlacklist reads and parses blacklist files, returning the checksum set.
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

// buildActiveMutators filters the full mutator list per enable/disable config.
func buildActiveMutators(opts *models.Options) []mutatorItem {
	effectiveDisable := append(opts.Mutator.DisableMutators, opts.Config.DisableMutators...)
	var mutators []mutatorItem
MUTATOR:
	for _, name := range mutator.List() {
		if len(opts.Config.EnableMutators) > 0 {
			allowed := false
			for _, e := range opts.Config.EnableMutators {
				if matchesMutator(e, name) {
					allowed = true
					break
				}
			}
			if !allowed {
				continue MUTATOR
			}
		}
		for _, d := range effectiveDisable {
			if matchesMutator(d, name) {
				continue MUTATOR
			}
		}
		console.Verbose(opts, "Enable mutator %q", name)
		m, _ := mutator.New(name)
		mutators = append(mutators, mutatorItem{Name: name, Mutator: m})
	}
	return mutators
}

// loadGitDiffLines loads changed lines from git when --git-diff-lines is active.
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

// runNoopChecks runs the test suite once without mutations to confirm it passes.
func runNoopChecks(opts *models.Options, pkgs []importing.Package, execs []string, extraTestFlags []string) int {
	if !opts.General.Noop || opts.Exec.NoExec {
		return returnOk
	}
	if len(execs) > 0 {
		fmt.Fprintln(os.Stderr, "Warning: --noop is not supported with --exec; skipping initial test run")
		return returnOk
	}
	for _, importPkg := range pkgs {
		pkgPath := packageImportPath(importPkg.Files)
		if pkgPath == "" {
			continue
		}
		noopArgs := []string{"test", "-timeout", fmt.Sprintf("%ds", opts.Exec.Timeout)}
		noopArgs = append(noopArgs, extraTestFlags...)
		noopArgs = append(noopArgs, pkgPath)
		cmd := exec.Command("go", noopArgs...)
		cmd.Env = os.Environ()
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "Noop check failed for %q — fix your tests before running mutation testing:\n%s\n", pkgPath, out)
			return returnError
		}
	}
	console.Verbose(opts, "Noop check passed — all packages green before mutation")
	return returnOk
}

// applyAdaptiveTimeout measures baseline run times and updates opts.Exec.Timeout.
func applyAdaptiveTimeout(opts *models.Options, pkgs []importing.Package, execs []string, extraTestFlags []string) {
	if opts.Exec.TimeoutCoefficient <= 0 || opts.Exec.NoExec || len(execs) > 0 {
		return
	}
	var maxBaseline time.Duration
	for _, importPkg := range pkgs {
		pkgPath := packageImportPath(importPkg.Files)
		if pkgPath == "" {
			continue
		}
		baseArgs := []string{"test", "-timeout", "300s"}
		baseArgs = append(baseArgs, extraTestFlags...)
		baseArgs = append(baseArgs, pkgPath)
		cmd := exec.Command("go", baseArgs...)
		cmd.Env = os.Environ()
		start := time.Now()
		_ = cmd.Run()
		elapsed := time.Since(start)
		if elapsed > maxBaseline {
			maxBaseline = elapsed
		}
	}
	if maxBaseline > 0 {
		derived := uint(math.Ceil(opts.Exec.TimeoutCoefficient * maxBaseline.Seconds()))
		if derived < 1 {
			derived = 1
		}
		opts.Exec.Timeout = derived
		console.Verbose(opts, "Adaptive timeout: baseline %.2fs × %.1f = %ds",
			maxBaseline.Seconds(), opts.Exec.TimeoutCoefficient, derived)
	}
}

// buildCoverageProfile runs go test -coverprofile for pkgFiles and returns the parsed profile.
// Returns nil when coverage is disabled or unavailable (soft failure).
func buildCoverageProfile(opts *models.Options, pkgFiles []string, tmpDir string, modulePath string) *coverage.Profile {
	if opts.Exec.NoExec || !opts.Exec.Coverage {
		return nil
	}
	pkgPath := packageImportPath(pkgFiles)
	if pkgPath == "" {
		return nil
	}
	profileDir := filepath.Join(tmpDir, "coverage", filepath.FromSlash(pkgPath))
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		console.Verbose(opts, "Cannot create coverage dir for %q: %v", pkgPath, err)
		return nil
	}
	profilePath := filepath.Join(profileDir, "coverage.out")
	if err := runCoverageProfile(pkgPath, profilePath); err != nil {
		console.Verbose(opts, "Coverage unavailable for %q: %v", pkgPath, err)
		return nil
	}
	prof, err := coverage.ParseProfile(profilePath, modulePath)
	if err != nil {
		console.Verbose(opts, "Coverage parse failed for %q: %v", pkgPath, err)
		return nil
	}
	return prof
}

// buildPerTestCoverageProfile builds a per-test coverage map for the package containing pkgFiles.
func buildPerTestCoverageProfile(opts *models.Options, pkgFiles []string, modulePath string, tmpDir string, numWorkers int, extraTestFlags []string) *coverage.PerTestProfile {
	pkgPath := packageImportPath(pkgFiles)
	if pkgPath == "" {
		return nil
	}
	testCount := coverage.CountTests(pkgPath)
	if testCount > 0 {
		fmt.Printf("Building per-test coverage map for %q (%d tests)...\n", pkgPath, testCount)
	}
	prof, err := coverage.BuildPerTestProfile(pkgPath, modulePath, tmpDir, opts.Exec.Timeout, numWorkers, extraTestFlags)
	if err != nil {
		console.Verbose(opts, "Per-test coverage unavailable for %q: %v", pkgPath, err)
		return nil
	}
	if prof != nil {
		console.Verbose(opts, "Per-test coverage map built for %q", pkgPath)
	}
	return prof
}

// calcNumWorkers returns the effective worker count (1 when --exec is set).
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

// startWorkerPool launches numWorkers goroutines draining the jobs channel.
// Returns nil, nil in no-exec or dry-run mode.
func startWorkerPool(opts *models.Options, numWorkers int, report *models.Report, mu *sync.Mutex) (chan execJob, *sync.WaitGroup) {
	if opts.Exec.NoExec || opts.General.DryRun {
		return nil, nil
	}
	jobs := make(chan execJob, numWorkers*2)
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				runExecJob(job, report, mu)
			}
		}()
	}
	return jobs, &wg
}

// progressMonitorEnabled reports whether the live progress monitor should run:
// only on an interactive terminal and outside verbose, debug, silent, no-exec,
// and dry-run modes.
func progressMonitorEnabled(opts *models.Options) bool {
	return isTerminal() && !opts.General.Verbose && !opts.General.Debug &&
		!opts.Config.SilentMode && !opts.Exec.NoExec && !opts.General.DryRun
}

// startProgressMonitor launches a goroutine printing live kill/escape/skip counts.
// Returns nil, nil when conditions are not met (non-terminal, verbose, silent, etc.).
func startProgressMonitor(opts *models.Options, report *models.Report, mu *sync.Mutex) (chan struct{}, *sync.WaitGroup) {
	if !progressMonitorEnabled(opts) {
		return nil, nil
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				mu.Lock()
				k := report.Stats.KilledCount
				e := report.Stats.EscapedCount
				s := report.Stats.SkippedCount
				n := report.Stats.NotCoveredCount
				mu.Unlock()
				fmt.Fprintf(os.Stderr, "\rMutating: killed=%-4d escaped=%-4d skip=%-4d not-covered=%-4d",
					k, e, s, n)
			case <-stop:
				fmt.Fprintf(os.Stderr, "\r\033[K")
				return
			}
		}
	}()
	return stop, &wg
}

// mutateAll iterates all packages and files, dispatching mutation jobs.
// Returns dry-run totals and any file-parse error exit code.
func (r *mutationRun) mutateAll(pkgs []importing.Package) (dryRunTotal int, dryRunMutatorTotals map[string]int, exitCode int) {
	if r.opts.General.DryRun {
		dryRunMutatorTotals = make(map[string]int)
	}
	for _, importPkg := range pkgs {
		coverProfile := r.coverageForPackage(importPkg)
		perTestProf := r.perTestForPackage(importPkg)
		for _, file := range importPkg.Files {
			count, code := r.processFile(file, coverProfile, perTestProf, dryRunMutatorTotals)
			if code != returnOk {
				return dryRunTotal, dryRunMutatorTotals, code
			}
			dryRunTotal += count
		}
	}
	return dryRunTotal, dryRunMutatorTotals, returnOk
}

// coverageForPackage builds the coverage profile for a package, marking the
// report as having coverage. It returns nil during dry runs.
func (r *mutationRun) coverageForPackage(importPkg importing.Package) *coverage.Profile {
	if r.opts.General.DryRun {
		return nil
	}
	coverProfile := buildCoverageProfile(r.opts, importPkg.Files, r.tmpDir, r.modulePath)
	if coverProfile != nil {
		r.report.HasCoverage = true
	}
	return coverProfile
}

// perTestForPackage builds the per-test coverage profile for a package, or nil
// when per-test coverage is not applicable.
func (r *mutationRun) perTestForPackage(importPkg importing.Package) *coverage.PerTestProfile {
	if !r.opts.Exec.PerTest || r.opts.Exec.NoExec || r.opts.General.DryRun || len(r.execs) != 0 {
		return nil
	}
	return buildPerTestCoverageProfile(r.opts, importPkg.Files, r.modulePath, r.tmpDir, r.numWorkers, r.extraTestFlags)
}

// processFile parses one source file, applies all mutators, and enqueues jobs.
// Returns the mutation count for this file and an exit code.
func (r *mutationRun) processFile(file string, coverProfile *coverage.Profile, perTestProf *coverage.PerTestProfile, dryRunMutatorTotals map[string]int) (int, int) {
	console.Verbose(r.opts, "Mutate %q", file)

	annotationProcessor := annotation.NewProcessor()
	skipFilterProcessor := filter.NewSkipMakeArgsFilter()
	sourceLineFilter := filter.NewSourceLineRegexFilter(r.opts.Config.IgnoreSourceLines)

	collectors := []filter.NodeCollector{annotationProcessor, skipFilterProcessor, sourceLineFilter}
	nodeFilters := []filter.NodeFilter{annotationProcessor, skipFilterProcessor, sourceLineFilter}

	src, fset, pkg, info, err := parser.ParseAndTypeCheckFile(file, collectors)
	if err != nil {
		return 0, exitError(err.Error())
	}

	if err := os.MkdirAll(r.tmpDir+"/"+filepath.Dir(file), 0755); err != nil {
		return 0, exitError("Cannot create mutation directory: %v", err)
	}

	tmpFile := r.tmpDir + "/" + file
	originalFile := fmt.Sprintf("%s.original", tmpFile)
	if err := osutil.CopyFile(file, originalFile); err != nil {
		return 0, exitError("Cannot copy original file: %v", err)
	}
	console.Debug(r.opts, "Save original into %q", originalFile)

	absFile, _ := filepath.Abs(file)

	fc := &fileContext{
		pkg:          pkg,
		info:         info,
		fset:         fset,
		src:          src,
		sourceFile:   file,
		mutatedFile:  tmpFile,
		absFile:      absFile,
		coverProfile: coverProfile,
		perTestProf:  perTestProf,
		filters:      nodeFilters,
	}

	return r.mutateFile(fc, dryRunMutatorTotals)
}

// mutateFile applies the configured mutators to one parsed file, honoring the
// --match function filter. Returns the mutation count and an exit code.
func (r *mutationRun) mutateFile(fc *fileContext, dryRunMutatorTotals map[string]int) (int, int) {
	mutationID := 0

	if r.opts.Filter.Match == "" {
		mutationID = r.mutate(fc, fc.src, mutationID, dryRunMutatorTotals)
		return mutationID, returnOk
	}

	m, err := regexp.Compile(r.opts.Filter.Match)
	if err != nil {
		return 0, exitError("Match regex is not valid: %v", err)
	}
	for _, f := range astutil.Functions(fc.src) {
		if m.MatchString(f.Name.Name) {
			mutationID = r.mutate(fc, f, mutationID, dryRunMutatorTotals)
		}
	}
	return mutationID, returnOk
}

// shutdownAndCleanup closes channels, drains workers, stops progress, and removes tmpDir.
func shutdownAndCleanup(opts *models.Options, jobs chan execJob, jobWg *sync.WaitGroup, stopProgress chan struct{}, progressWg *sync.WaitGroup, tmpDir string) {
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
		fmt.Fprintf(os.Stderr, "mutago: cannot remove %s: %v\n", tmpDir, err)
		return
	}
	console.Debug(opts, "Remove %q", tmpDir)
}

// printDryRunReport prints per-mutator counts and total for a dry run.
func printDryRunReport(total int, totals map[string]int) {
	if len(totals) > 0 {
		names := make([]string, 0, len(totals))
		for name := range totals {
			names = append(names, name)
		}
		sort.Strings(names)
		fmt.Println("\nPer-mutator totals across all files:")
		for _, name := range names {
			fmt.Printf("  %-40s %d\n", name, totals[name])
		}
	}
	fmt.Printf("\nTotal: %d mutation(s) would be generated. No files written, no tests run.\n", total)
	fmt.Println("Note: this count is an upper bound. Identical mutations across files are deduplicated during an actual run.")

}

// handleBaselineUpdate writes the current escaped mutants to the baseline file.
// Returns (true, exitCode) when handled, (false, 0) to continue.
func handleBaselineUpdate(opts *models.Options, report *models.Report, moduleRoot string) (bool, int) {
	if !opts.Baseline.Update {
		return false, 0
	}
	if err := baseline.Write(opts.Baseline.File, report.Escaped, moduleRoot); err != nil {
		return true, exitError("Cannot write baseline: %v", err)
	}
	fmt.Printf("Baseline written to %q (%d surviving mutant(s))\n", opts.Baseline.File, len(report.Escaped))
	return true, returnOk
}

// printResultsIfNeeded prints the summary line and GitHub annotations when appropriate.
func printResultsIfNeeded(opts *models.Options, report *models.Report) {
	if opts.Exec.NoExec {
		fmt.Println("Cannot do a mutation testing summary since no exec command was executed.")
		return
	}
	if opts.Exec.RunMutantID == "" {
		printSummary(report)
	}
	if opts.Logger.GitHub {
		printGitHubAnnotations(report)
	}
}

// writeAllReports writes every enabled report format; returns first error exit code.
func writeAllReports(opts *models.Options, report *models.Report, moduleRoot string) int {
	specs := []reportSpec{
		{
			enabled:  opts.General.Config == "" || opts.Config.JSONOutput,
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
		if code := writeReport(opts, s); code != returnOk {
			return code
		}
	}
	return returnOk
}

// reportSpec describes one optional report format and how to write it.
type reportSpec struct {
	enabled  bool
	write    func() error
	savedMsg string
	fileName string
}

// writeReport writes one report when it is enabled, returning an error exit code
// on failure and logging a verbose confirmation on success.
func writeReport(opts *models.Options, s reportSpec) int {
	if !s.enabled {
		return returnOk
	}
	if err := s.write(); err != nil {
		return exitError(err.Error())
	}
	console.Verbose(opts, s.savedMsg, s.fileName)
	return returnOk
}

// printSummary prints the final mutation testing summary including per-mutator breakdown.
func printSummary(report *models.Report) {
	msiPct := report.Stats.Msi * 100
	covMsiPct := report.Stats.CoveredCodeMsi * 100
	fmt.Printf(
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
		fmt.Printf("The covered-code mutation score is %.2f%%\n", covMsiPct)
	}

	if len(report.MutatorStats) > 0 {
		fmt.Println("\nPer-mutator breakdown:")
		// Sort by name for stable output.
		sorted := make([]models.MutatorStats, len(report.MutatorStats))
		copy(sorted, report.MutatorStats)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
		for _, ms := range sorted {
			killRate := 0.0
			if ms.Total > 0 {
				killRate = float64(ms.Killed) / float64(ms.Total) * 100
			}
			fmt.Printf("  %-35s  killed %3d / %-3d  (%.0f%%)\n", ms.Name, ms.Killed, ms.Total, killRate)
		}
	}
}

// printGitHubAnnotations writes escaped mutants as GitHub Actions ::warning
// annotations so they appear inline in PR diffs. File paths are made relative
// to the repo root so GitHub can match them against the diff.
func printGitHubAnnotations(report *models.Report) {
	repoRoot := ""
	if out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
		repoRoot = strings.TrimSpace(string(out))
	}

	for _, m := range report.Escaped {
		filePath := filepath.ToSlash(m.Mutator.OriginalFilePath)
		if repoRoot != "" {
			if rel, err := filepath.Rel(repoRoot, m.Mutator.OriginalFilePath); err == nil {
				filePath = filepath.ToSlash(rel)
			}
		}
		fmt.Printf("::warning file=%s,line=%d,title=Mutant escaped (%s)::Escaped mutation at %s:%d — add a test to kill it\n",
			filePath,
			m.Mutator.OriginalStartLine,
			m.Mutator.MutatorName,
			filePath,
			m.Mutator.OriginalStartLine,
		)
	}
}

// checkQualityGates returns returnMsiThresholdNotMet if configured thresholds
// are not met, otherwise returnOk.
func checkQualityGates(opts *models.Options, report *models.Report, bl *baseline.File, moduleRoot string) int {
	// When no mutations were generated (e.g. --git-diff-lines on an unchanged
	// package), skip threshold checks rather than failing with 0% MSI.
	if opts.Score.IgnoreMsiWithNoMutations && report.Stats.TotalMutantsCount == 0 {
		return returnOk
	}

	// CLI flag is -1 when not provided; config file defaults to 0 when not set.
	// CLI always wins when explicitly set (>= 0); fall back to config otherwise.
	minMsi := resolveThreshold(opts.Score.MinMsi, opts.Config.MinMsi)
	minCoveredMsi := resolveThreshold(opts.Score.MinCoveredMsi, opts.Config.MinCoveredMsi)

	// Evaluate every gate so all failures are reported, not just the first.
	escapedFail := checkEscapedGate(opts, report, bl, moduleRoot)
	msiFail := checkMsiGate(report, minMsi)
	coveredFail := checkCoveredMsiGate(report, minCoveredMsi)

	if escapedFail || msiFail || coveredFail {
		return returnMsiThresholdNotMet
	}
	return returnOk
}

// resolveThreshold returns the CLI value when it was explicitly set (>= 0),
// otherwise the config fallback.
func resolveThreshold(cliValue, configValue float64) float64 {
	if cliValue < 0 {
		return configValue
	}
	return cliValue
}

// checkEscapedGate reports whether --fail-on-escaped should fail the run. With a
// baseline active, only escapes not already in the baseline count.
func checkEscapedGate(opts *models.Options, report *models.Report, bl *baseline.File, moduleRoot string) bool {
	if !opts.Score.FailOnEscaped {
		return false
	}
	newEscapes := bl.NewEscapes(report.Escaped, moduleRoot)
	if len(newEscapes) == 0 {
		return false
	}
	qualifier := ""
	if bl != nil {
		qualifier = "new "
	}
	fmt.Fprintf(os.Stderr, "%d %smutant(s) escaped — kill them or run --update-baseline to accept\n", len(newEscapes), qualifier)
	return true
}

// checkMsiGate reports whether the overall MSI is below the required minimum.
func checkMsiGate(report *models.Report, minMsi float64) bool {
	msiPct := report.Stats.Msi * 100
	if minMsi >= 0 && msiPct < minMsi {
		fmt.Fprintf(os.Stderr, "MSI %.2f%% is below minimum required %.2f%%\n", msiPct, minMsi)
		return true
	}
	return false
}

// checkCoveredMsiGate reports whether the covered-code MSI gate fails, treating
// a missing coverage profile as a failure when the gate is active.
func checkCoveredMsiGate(report *models.Report, minCoveredMsi float64) bool {
	if minCoveredMsi <= 0 {
		return false
	}
	if !report.HasCoverage {
		fmt.Fprintf(os.Stderr, "Covered MSI cannot be checked: --coverage was not enabled (score is always 0 without a profile)\n")
		return true
	}
	covMsiPct := report.Stats.CoveredCodeMsi * 100
	if covMsiPct < minCoveredMsi {
		fmt.Fprintf(os.Stderr, "Covered MSI %.2f%% is below minimum required %.2f%%\n", covMsiPct, minCoveredMsi)
		return true
	}
	return false
}

// mutate applies every active mutator to node, writing mutations and enqueuing
// exec jobs (or counting them in dry-run mode). Returns the next mutation ID.
func (r *mutationRun) mutate(fc *fileContext, node ast.Node, mutationID int, dryRunGlobalTotals map[string]int) int {
	// Read the original source once per file — it never changes during mutation.
	originalSourceCode, err := os.ReadFile(fc.sourceFile)
	if err != nil {
		log.Fatal(err)
	}
	// Pre-format the original once; saveAST needs the formatted version for
	// duplicate detection and would otherwise re-format it for every mutation.
	fmtOriginal, fmtErr := format.Source(originalSourceCode)
	if fmtErr != nil {
		fmtOriginal = originalSourceCode
	}

	// In dry-run mode collect per-mutator counts and print after the file is done.
	var dryRunCounts map[string]int
	if r.opts.General.DryRun {
		dryRunCounts = make(map[string]int)
	}

	for _, m := range r.mutators {
		mutationID = r.applyMutator(m, fc, node, mutationID, originalSourceCode, fmtOriginal, dryRunCounts, dryRunGlobalTotals)
	}

	printDryRunFileSummary(fc.sourceFile, dryRunCounts)

	return mutationID
}

// applyMutator drives a single mutator over node, processing each generated
// mutation and advancing the MutateWalk goroutine. Returns the next mutation ID.
func (r *mutationRun) applyMutator(m mutatorItem, fc *fileContext, node ast.Node, mutationID int, originalSourceCode, fmtOriginal []byte, dryRunCounts, dryRunGlobalTotals map[string]int) int {
	console.Debug(r.opts, "Mutator %s", m.Name)

	mutatorAnnotated := annotation.DecoratorFilter(m.Mutator, m.Name, fc.filters...)
	changed := mutago.MutateWalkWithPositions(fc.pkg, fc.info, node, mutatorAnnotated)

	for {
		mutation, ok := <-changed
		if !ok {
			break
		}

		originalStartLine := int64(fc.fset.Position(mutation.Position).Line)
		r.recordOneMutation(m, fc, mutationID, originalStartLine, originalSourceCode, fmtOriginal, dryRunCounts, dryRunGlobalTotals)

		// Release the MutateWalk goroutine to reset the AST and advance.
		changed <- mutago.PositionedMutation{}
		<-changed
		changed <- mutago.PositionedMutation{}

		mutationID++
	}
	return mutationID
}

// recordOneMutation tallies (dry run) or writes and enqueues a single generated
// mutation.
func (r *mutationRun) recordOneMutation(m mutatorItem, fc *fileContext, mutationID int, originalStartLine int64, originalSourceCode, fmtOriginal []byte, dryRunCounts, dryRunGlobalTotals map[string]int) {
	if r.opts.General.DryRun {
		countDryRunMutation(m.Name, dryRunCounts, dryRunGlobalTotals)
		return
	}
	r.processMutation(m, fc, mutationID, originalStartLine, originalSourceCode, fmtOriginal)
}

// countDryRunMutation records a would-be mutation in the per-file and global
// dry-run tallies.
func countDryRunMutation(name string, dryRunCounts, dryRunGlobalTotals map[string]int) {
	dryRunCounts[name]++
	if dryRunGlobalTotals != nil {
		dryRunGlobalTotals[name]++
	}
}

// processMutation writes one mutation to disk and either records a duplicate, an
// internal error, or enqueues an exec job. Must not be called in dry-run mode.
func (r *mutationRun) processMutation(m mutatorItem, fc *fileContext, mutationID int, originalStartLine int64, originalSourceCode, fmtOriginal []byte) {
	mutant := models.Mutant{}
	mutant.Mutator.MutatorName = m.Name
	mutant.Mutator.OriginalFilePath = fc.sourceFile
	mutant.Mutator.OriginalSourceCode = string(originalSourceCode)
	mutant.Mutator.OriginalStartLine = originalStartLine

	mutationFile := fmt.Sprintf("%s.%d", fc.mutatedFile, mutationID)
	checksum, duplicate, err := saveAST(r.blacklist, mutationFile, fc.fset, fc.src, fmtOriginal)
	if err != nil {
		out := fmt.Sprintf("INTERNAL ERROR %s\n", err.Error())
		fmt.Printf("%s", out)
		mutant.ProcessOutput = out
		r.mu.Lock()
		r.report.Errored = append(r.report.Errored, mutant)
		r.report.Stats.ErrorCount++
		r.mu.Unlock()
		return
	}
	if duplicate {
		console.Debug(r.opts, "%q is a duplicate, we ignore it", mutationFile)
		r.mu.Lock()
		r.report.Stats.DuplicatedCount++
		r.mu.Unlock()
		return
	}
	if r.jobs == nil {
		return
	}

	console.Debug(r.opts, "Save mutation into %q with checksum %s", mutationFile, checksum)
	r.jobs <- execJob{
		opts:            r.opts,
		pkg:             fc.pkg,
		originalFile:    fc.sourceFile,
		mutationFile:    mutationFile,
		mutant:          mutant,
		absFile:         fc.absFile,
		coverProfile:    fc.coverProfile,
		gitChangedLines: r.gitChangedLines,
		execs:           r.execs,
		perTestProf:     fc.perTestProf,
		extraTestFlags:  r.extraTestFlags,
		runMutantID:     r.opts.Exec.RunMutantID,
		moduleRoot:      r.moduleRoot,
	}
}

// printDryRunFileSummary prints per-mutator dry-run counts for a single file.
func printDryRunFileSummary(originalFile string, counts map[string]int) {
	if len(counts) == 0 {
		return
	}
	display := originalFile
	if rel, err := filepath.Rel(".", originalFile); err == nil {
		display = filepath.ToSlash(rel)
	}
	fmt.Printf("%s\n", display)
	mutatorNames := make([]string, 0, len(counts))
	for name := range counts {
		mutatorNames = append(mutatorNames, name)
	}
	sort.Strings(mutatorNames)
	for _, name := range mutatorNames {
		fmt.Printf("  %-40s %d\n", name, counts[name])
	}
}

// skipForGitDiff reports whether the mutation falls outside the --git-diff-lines
// filter and should not be tested.
func skipForGitDiff(job execJob) bool {
	if job.gitChangedLines == nil {
		return false
	}
	lineNum := int(job.mutant.Mutator.OriginalStartLine)
	if gitdiff.IsLineChanged(job.gitChangedLines, job.absFile, lineNum) {
		return false
	}
	console.Debug(job.opts, "Skip %q at line %d (not in git diff)", job.mutationFile, lineNum)
	return true
}

// skipForMutantID reports whether the job's computed mutant ID does not match
// the requested --run-mutant-id value.
func skipForMutantID(job execJob) bool {
	if job.runMutantID == "" {
		return false
	}
	diffOut, _ := exec.Command("diff", "--label=Original", "--label=New", "-u", job.originalFile, job.mutationFile).CombinedOutput()
	relFile := job.absFile
	if job.moduleRoot != "" {
		if rel, err := filepath.Rel(job.moduleRoot, job.absFile); err == nil {
			relFile = filepath.ToSlash(rel)
		}
	}
	id := baseline.MutantID(relFile, job.mutant.Mutator.MutatorName, string(diffOut))
	return id != job.runMutantID
}

// runExecJob executes a single mutation job in a worker goroutine.
// It applies the git-diff filter, runs go test via overlay (or --exec),
// checks coverage, and records the result under mu.
func runExecJob(job execJob, stats *models.Report, mu *sync.Mutex) {
	opts := job.opts
	mutant := job.mutant

	if skipForGitDiff(job) || skipForMutantID(job) {
		return
	}

	execExitCode := mutateExec(opts, job.pkg, job.originalFile, job.mutationFile, job.execs, job.perTestProf, job.absFile, job.extraTestFlags, &mutant)
	console.Debug(opts, "Exited with %d", execExitCode)

	mutatedSourceCode, err := os.ReadFile(job.mutationFile)
	if err != nil {
		log.Fatal(err)
	}
	mutant.Mutator.MutatedSourceCode = string(mutatedSourceCode)

	startLine := mutant.Mutator.OriginalStartLine
	notCovered := job.coverProfile != nil && startLine > 0 && !job.coverProfile.IsCovered(job.absFile, int(startLine))

	loc := mutant.Mutator.OriginalFilePath
	if rel, err := filepath.Rel(".", loc); err == nil {
		loc = filepath.ToSlash(rel)
	}
	if mutant.Mutator.OriginalStartLine > 0 {
		loc = fmt.Sprintf("%s:%d", loc, mutant.Mutator.OriginalStartLine)
	}
	msg := fmt.Sprintf("%s (%s)", loc, mutant.Mutator.MutatorName)

	mu.Lock()
	defer mu.Unlock()
	if notCovered {
		recordNotCovered(opts, stats, mutant, msg)
		return
	}
	recordMutantResult(opts, stats, mutant, execExitCode, msg)
}

// recordMutantResult appends the mutant to the appropriate stats bucket and prints status.
// Must be called with mu held.
func recordMutantResult(opts *models.Options, stats *models.Report, mutant models.Mutant, execExitCode int, msg string) {
	switch execExitCode {
	case 0: // Tests failed → mutation killed
		recordKilled(opts, stats, mutant, msg)
	case 1: // Tests passed → mutation escaped
		recordEscaped(opts, stats, mutant, msg)
	case 2: // Did not compile → skip
		recordSkipped(opts, stats, mutant, msg)
	default:
		recordErrored(opts, stats, mutant, msg)
	}
}

// recordNotCovered records a mutant on a line not reached by any test.
// Must be called with mu held.
func recordNotCovered(opts *models.Options, stats *models.Report, mutant models.Mutant, msg string) {
	out := fmt.Sprintf("NOT COVERED %s\n", msg)
	if statusVisible(opts, 'n') {
		console.PrintSkip(out)
	}
	mutant.ProcessOutput = out
	stats.NotCovered = append(stats.NotCovered, mutant)
	stats.Stats.NotCoveredCount++
}

// recordKilled records a mutant whose tests failed. Must be called with mu held.
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

// recordEscaped records a mutant whose tests passed. Must be called with mu held.
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

// recordSkipped records a mutant that did not compile. Must be called with mu held.
func recordSkipped(opts *models.Options, stats *models.Report, mutant models.Mutant, msg string) {
	out := fmt.Sprintf("SKIP %s\n", msg)
	if statusVisible(opts, 's') {
		console.PrintSkip(out)
	}
	if opts.General.Verbose {
		fmt.Println("Mutation did not compile")
	}
	if opts.General.Debug && !opts.General.NoDiffs && mutant.Diff != "" {
		console.PrintDiff([]byte(mutant.Diff))
	}
	mutant.ProcessOutput = out
	stats.Skipped = append(stats.Skipped, mutant)
	stats.Stats.SkippedCount++
}

// recordErrored records a mutant with an unexpected exit code. Must be called with mu held.
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

func mutateExec(
	opts *models.Options,
	pkg *types.Package,
	file string,
	mutationFile string,
	execs []string,
	perTestProf *coverage.PerTestProfile,
	absFile string,
	extraTestFlags []string,
	mutant *models.Mutant,
) int {
	if len(execs) == 0 {
		return runBuiltinExec(opts, pkg, file, mutationFile, perTestProf, absFile, extraTestFlags, mutant)
	}
	return runCustomExec(opts, pkg, file, mutationFile, execs, mutant)
}

// runBuiltinExec runs go test with an overlay file to test the mutation in-process.
func runBuiltinExec(
	opts *models.Options,
	pkg *types.Package,
	file string,
	mutationFile string,
	perTestProf *coverage.PerTestProfile,
	absFile string,
	extraTestFlags []string,
	mutant *models.Mutant,
) int {
	console.Debug(opts, "Execute built-in exec command for mutation")

	diff, err := exec.Command("diff", "--label=Original", "--label=New", "-u", file, mutationFile).CombinedOutput()
	startLine := mutant.Mutator.OriginalStartLine
	if startLine <= 0 {
		startLine = parser.FindOriginalStartLine(diff)
		mutant.Mutator.OriginalStartLine = startLine
	}

	diffExitCode, ok := commandExitCode(err)
	if !ok {
		fmt.Fprintf(os.Stderr, "mutago: diff error: %v\n", err)
		return 3
	}
	if diffExitCode != 0 && diffExitCode != 1 {
		fmt.Fprintf(os.Stderr, "mutago: diff exited with code %d\n", diffExitCode)
		return 3
	}

	absOrig, _ := filepath.Abs(file)
	absMut, _ := filepath.Abs(mutationFile)
	overlayName, err := writeOverlayFile(absOrig, absMut)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mutago: cannot create overlay file: %v\n", err)
		return 3
	}
	defer os.Remove(overlayName)

	pkgName := pkg.Path()
	if opts.Test.Recursive {
		pkgName += "/..."
	}

	runFilter := perTestRunFilter(perTestProf, absFile, int(startLine))

	goTestArgs := []string{"test", "-overlay=" + overlayName, "-timeout", fmt.Sprintf("%ds", opts.Exec.Timeout)}
	goTestArgs = append(goTestArgs, extraTestFlags...)
	if runFilter != "" {
		goTestArgs = append(goTestArgs, "-run", runFilter)
	}
	goTestArgs = append(goTestArgs, pkgName)

	goTestCmd := exec.Command("go", goTestArgs...)
	goTestCmd.Env = os.Environ()
	test, err := goTestCmd.CombinedOutput()

	execExitCode, ok := commandExitCode(err)
	if !ok {
		fmt.Fprintf(os.Stderr, "mutago: go test error: %v\n", err)
		return 3
	}

	if opts.General.Debug {
		fmt.Printf("%s\n", test)
	}

	mutant.Diff = string(diff)
	return mapTestExitToResult(execExitCode)
}

// commandExitCode maps an exec error to a process exit code. ok is false when
// the command could not be run at all, as opposed to running and exiting
// non-zero.
func commandExitCode(err error) (code int, ok bool) {
	if err == nil {
		return 0, true
	}
	if e, isExit := err.(*exec.ExitError); isExit {
		return e.Sys().(syscall.WaitStatus).ExitStatus(), true
	}
	return 0, false
}

// writeOverlayFile writes a `go build` overlay mapping absOrig→absMut and
// returns the overlay file path. The caller is responsible for removing it.
func writeOverlayFile(absOrig, absMut string) (string, error) {
	overlayData, err := json.Marshal(struct {
		Replace map[string]string `json:"Replace"`
	}{Replace: map[string]string{absOrig: absMut}})
	if err != nil {
		return "", err
	}

	f, err := os.CreateTemp("", "mutago-overlay-*.json")
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

// perTestRunFilter returns a -run regexp restricting the run to the tests that
// cover absFile at startLine, or "" when per-test data is unavailable.
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

// mapTestExitToResult maps a go test exit code to mutago's result code: 0 (tests
// passed → mutation escaped) becomes 1, 1 (tests failed → killed) becomes 0,
// and any other code is passed through unchanged.
func mapTestExitToResult(execExitCode int) int {
	switch execExitCode {
	case 0:
		return 1
	case 1:
		return 0
	}
	return execExitCode
}

// runCustomExec runs the user-provided --exec command for a mutation.
func runCustomExec(opts *models.Options, pkg *types.Package, file string, mutationFile string, execs []string, mutant *models.Mutant) int {
	console.Debug(opts, "Execute %q for mutation", opts.Exec.Exec)

	extDiff, _ := exec.Command("diff", "--label=Original", "--label=New", "-u", file, mutationFile).CombinedOutput()
	if mutant.Mutator.OriginalStartLine <= 0 {
		mutant.Mutator.OriginalStartLine = parser.FindOriginalStartLine(extDiff)
	}
	mutant.Diff = string(extDiff)

	execCommand := exec.Command(execs[0], execs[1:]...)
	execCommand.Stderr = os.Stderr
	execCommand.Stdout = os.Stdout
	execCommand.Env = append(os.Environ(), []string{
		"MUTATE_CHANGED=" + mutationFile,
		fmt.Sprintf("MUTATE_DEBUG=%t", opts.General.Debug),
		"MUTATE_ORIGINAL=" + file,
		"MUTATE_PACKAGE=" + pkg.Path(),
		fmt.Sprintf("MUTATE_TIMEOUT=%d", opts.Exec.Timeout),
		fmt.Sprintf("MUTATE_VERBOSE=%t", opts.General.Verbose),
	}...)
	if opts.Test.Recursive {
		execCommand.Env = append(execCommand.Env, "TEST_RECURSIVE=true")
	}

	if err := execCommand.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "mutago: cannot start %q: %v\n", execs[0], err)
		return 3
	}
	err := execCommand.Wait()

	execExitCode, ok := commandExitCode(err)
	if !ok {
		fmt.Fprintf(os.Stderr, "mutago: exec wait error: %v\n", err)
		return 3
	}
	return execExitCode
}

func main() {
	os.Exit(mainCmd(os.Args[1:]))
}

// saveAST writes the mutated AST to file and returns a stable checksum.
// The checksum is derived from only the lines that differ between original
// and mutated source, so it survives edits to surrounding code that would
// otherwise invalidate a blacklist entry.
// saveAST writes the mutated AST to file and returns a dedup checksum. fmtOriginal
// must already be gofmt-formatted (call format.Source once per file, not per mutation).
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

// stableMutationKey returns an MD5 hash of only the lines that differ between
// original and mutated source. This is stable when unrelated surrounding code
// changes, unlike hashing the entire file.
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

// detectDefaultBranch returns the default remote branch name by inspecting
// refs/remotes/origin/HEAD. Falls back to "master" when unavailable.
func detectDefaultBranch() string {
	out, err := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD").Output()
	if err == nil {
		// Output is "refs/remotes/origin/main\n" — extract the last segment.
		ref := strings.TrimSpace(string(out))
		if idx := strings.LastIndex(ref, "/"); idx >= 0 {
			return ref[idx+1:]
		}
	}
	return "master"
}

// detectModulePath returns the current module path via `go list -m`.
// This works regardless of where go.mod lives relative to the working directory.
func detectModulePath() string {
	cmd := exec.Command("go", "list", "-m")
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// detectModuleRoot returns the directory containing the module's go.mod file.
// Used to compute relative file paths for baseline and agentic JSON output.
func detectModuleRoot() string {
	cmd := exec.Command("go", "env", "GOMOD")
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	gomod := strings.TrimSpace(string(out))
	if gomod == "" || gomod == os.DevNull {
		return ""
	}
	return filepath.Dir(gomod)
}

// runCoverageProfile runs go test -coverprofile for pkg and writes output to profilePath.
// Test failures are tolerated — the profile may still be written.
func runCoverageProfile(pkg, profilePath string) error {
	cmd := exec.Command("go", "test", "-coverprofile="+profilePath, pkg)
	cmd.Env = os.Environ()
	// We intentionally ignore test failures: a package with failing tests should
	// still produce a (partial) coverage profile so we can identify covered lines.
	_ = cmd.Run()
	if _, err := os.Stat(profilePath); err != nil {
		return fmt.Errorf("coverage profile not created for %q", pkg)
	}
	return nil
}

// packageImportPath returns the import path for the package containing files,
// by parsing the first file's package declaration.
func packageImportPath(files []string) string {
	if len(files) == 0 {
		return ""
	}
	f, err := filepath.Abs(files[0])
	if err != nil {
		return ""
	}
	dir := filepath.Dir(f)
	cmd := exec.Command("go", "list", dir)
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
