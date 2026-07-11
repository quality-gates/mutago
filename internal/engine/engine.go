package engine

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/printer"
	"go/token"
	"go/types"
	"io"
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
	returnOk                 = 0
	returnError              = 3
	returnMsiThresholdNotMet = 4
)

// Engine orchestrates the mutation testing lifecycle.
type Engine struct {
	Stdout io.Writer
	Stderr io.Writer
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

type mutationRun struct {
	opts           *models.Options
	mutators       []mutatorItem
	blacklist      map[string]struct{}
	tmpDir         string
	numWorkers     int
	execs          []string
	extraTestFlags []string
	report         *models.Report
	mu             *sync.Mutex
	modulePath     string
	moduleRoot     string
	jobs           chan<- execJob
	stdout         io.Writer
}

type execJob struct {
	opts           *models.Options
	pkg            *types.Package
	originalFile   string
	mutationFile   string
	mutant         models.Mutant
	absFile        string
	coverProfile   *coverage.Profile
	execs          []string
	perTestProf    *coverage.PerTestProfile
	extraTestFlags []string
	runMutantID    string
	moduleRoot     string
}

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

// Run executes the mutation testing lifecycle based on options and baseline.
func (e *Engine) Run(ctx context.Context, opts *models.Options, bl *baseline.File) (Result, error) {
	e.initDefaults()

	run, pkgs, jobs, jobWg, stopProgress, progressWg, _, err := e.initRun(opts)
	if err != nil {
		return Result{ExitCode: returnError}, err
	}
	if run == nil {
		// initRun returns a nil run with a non-nil result for early exits.
		return Result{ExitCode: returnError}, nil
	}

	report := run.report
	if exitCode := runNoopChecks(opts, pkgs, run.execs, run.extraTestFlags); exitCode != 0 {
		return Result{Report: report, ExitCode: exitCode}, nil
	}

	applyAdaptiveTimeout(opts, pkgs, run.execs, run.extraTestFlags)

	dryRunTotal, dryRunMutatorTotals, loopCode := run.mutateAll(pkgs)

	if !opts.General.DryRun {
		shutdownAndCleanup(opts, jobs, jobWg, stopProgress, progressWg, run.tmpDir)
	}

	if loopCode != returnOk {
		return Result{Report: report, ExitCode: loopCode}, nil
	}

	if opts.General.DryRun {
		report.Stats.TotalMutantsCount = int64(dryRunTotal)
		printDryRunReport(run.stdout, dryRunTotal, dryRunMutatorTotals)
		return Result{Report: report, ExitCode: returnOk}, nil
	}

	report.Calculate()
	exitCode := finalizeResults(e.Stdout, e.Stderr, opts, report, bl, run.moduleRoot)
	return Result{Report: report, ExitCode: exitCode}, nil
}

func (e *Engine) initDefaults() {
	if e.Stdout == nil {
		e.Stdout = os.Stdout
	}
	if e.Stderr == nil {
		e.Stderr = os.Stderr
	}
}

func (e *Engine) initRun(opts *models.Options) (*mutationRun, []importing.Package, chan execJob, *sync.WaitGroup, chan struct{}, *sync.WaitGroup, gitdiff.ChangedLines, error) {
	files := importing.FilesOfArgs(opts.Remaining.Targets, opts)
	if len(files) == 0 {
		return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("Could not find any suitable Go source files")
	}

	mutationBlackList, err := loadBlacklist(opts.Files.Blacklist)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}

	gitChangedLines, err := loadGitDiffLines(opts)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("Cannot load git diff: %w", err)
	}

	pkgs := importing.PackagesWithFilesOfArgs(opts.Remaining.Targets, opts)

	report := &models.Report{}
	var reportMu sync.Mutex

	execs, extraTestFlags := parseExecFlags(opts)

	numWorkers := calcNumWorkers(opts, execs)
	console.Verbose(opts, "Running with %d parallel worker(s)", numWorkers)

	tmpDir, err := createTmpDir(opts)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}

	var jobs chan execJob
	var jobWg *sync.WaitGroup
	if !opts.General.DryRun && !opts.Exec.NoExec {
		jobs, jobWg = startWorkerPool(opts, numWorkers, report, &reportMu, e.Stdout, gitChangedLines)
	}

	var stopProgress chan struct{}
	var progressWg *sync.WaitGroup
	if progressMonitorEnabled(opts) {
		stopProgress, progressWg = startProgressMonitor(opts, report, &reportMu, e.Stderr)
	}

	run := &mutationRun{
		opts:           opts,
		mutators:       buildActiveMutators(opts),
		blacklist:      mutationBlackList,
		tmpDir:         tmpDir,
		numWorkers:     numWorkers,
		execs:          execs,
		extraTestFlags: extraTestFlags,
		report:         report,
		mu:             &reportMu,
		modulePath:     detectModulePath(),
		moduleRoot:     detectModuleRoot(),
		jobs:           jobs,
		stdout:         e.Stdout,
	}

	return run, pkgs, jobs, jobWg, stopProgress, progressWg, gitChangedLines, nil
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

func matchesMutator(pattern, name string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(name, strings.TrimSuffix(pattern, "*"))
	}
	return name == pattern
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
		if idx := strings.LastIndex(ref, "/"); idx >= 0 {
			return ref[idx+1:]
		}
	}
	return "master"
}

func detectModulePath() string {
	cmd := exec.Command("go", "list", "-m")
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

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

func (r *mutationRun) mutateAll(pkgs []importing.Package) (dryRunTotal int, dryRunMutatorTotals map[string]int, exitCode int) {
	if r.opts.General.DryRun {
		dryRunMutatorTotals = make(map[string]int)
	}
	for _, importPkg := range pkgs {
		coverProfile := r.coverageForPackage(importPkg)
		perTestProf := r.perTestForPackage(importPkg)
		for _, file := range importPkg.Files {
			count, code := r.processFile(file, coverProfile, perTestProf, dryRunMutatorTotals)
			if code != 0 {
				return dryRunTotal, dryRunMutatorTotals, code
			}
			dryRunTotal += count
		}
	}
	return dryRunTotal, dryRunMutatorTotals, 0
}

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

func (r *mutationRun) perTestForPackage(importPkg importing.Package) *coverage.PerTestProfile {
	if !r.opts.Exec.PerTest || r.opts.Exec.NoExec || r.opts.General.DryRun || len(r.execs) != 0 {
		return nil
	}
	return buildPerTestCoverageProfile(r.opts, importPkg.Files, r.modulePath, r.tmpDir, r.numWorkers, r.extraTestFlags)
}

func (r *mutationRun) processFile(file string, coverProfile *coverage.Profile, perTestProf *coverage.PerTestProfile, dryRunMutatorTotals map[string]int) (int, int) {
	console.Verbose(r.opts, "Mutate %q", file)

	annotationProcessor := annotation.NewProcessor()
	skipFilterProcessor := filter.NewSkipMakeArgsFilter()
	sourceLineFilter := filter.NewSourceLineRegexFilter(r.opts.Config.IgnoreSourceLines)

	collectors := []filter.NodeCollector{annotationProcessor, skipFilterProcessor, sourceLineFilter}
	nodeFilters := []filter.NodeFilter{annotationProcessor, skipFilterProcessor, sourceLineFilter}

	src, fset, pkg, info, err := parser.ParseAndTypeCheckFile(file, collectors)
	if err != nil {
		return 0, returnError
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

func (r *mutationRun) mutateFile(fc *fileContext, dryRunMutatorTotals map[string]int) (int, int) {
	mutationID := 0

	if r.opts.Filter.Match == "" {
		mutationID = r.mutate(fc, fc.src, mutationID, dryRunMutatorTotals)
		return mutationID, 0
	}

	m, err := regexp.Compile(r.opts.Filter.Match)
	if err != nil {
		return 0, returnError
	}
	for _, f := range astutil.Functions(fc.src) {
		if m.MatchString(f.Name.Name) {
			mutationID = r.mutate(fc, f, mutationID, dryRunMutatorTotals)
		}
	}
	return mutationID, 0
}

func (r *mutationRun) mutate(fc *fileContext, node ast.Node, mutationID int, dryRunGlobalTotals map[string]int) int {
	originalSourceCode, err := os.ReadFile(fc.sourceFile)
	if err != nil {
		log.Fatal(err)
	}
	fmtOriginal, fmtErr := format.Source(originalSourceCode)
	if fmtErr != nil {
		fmtOriginal = originalSourceCode
	}

	var dryRunCounts map[string]int
	if r.opts.General.DryRun {
		dryRunCounts = make(map[string]int)
	}

	for _, m := range r.mutators {
		mutationID = r.applyMutator(m, fc, node, mutationID, originalSourceCode, fmtOriginal, dryRunCounts, dryRunGlobalTotals)
	}

	printDryRunFileSummary(r.opts, r.stdout, fc.sourceFile, dryRunCounts)

	return mutationID
}

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

		changed <- mutago.PositionedMutation{}
		<-changed
		changed <- mutago.PositionedMutation{}

		mutationID++
	}
	return mutationID
}

func (r *mutationRun) recordOneMutation(m mutatorItem, fc *fileContext, mutationID int, originalStartLine int64, originalSourceCode, fmtOriginal []byte, dryRunCounts, dryRunGlobalTotals map[string]int) {
	if r.opts.General.DryRun {
		countDryRunMutation(m.Name, dryRunCounts, dryRunGlobalTotals)
		return
	}
	r.processMutation(m, fc, mutationID, originalStartLine, originalSourceCode, fmtOriginal)
}

func (r *mutationRun) processMutation(m mutatorItem, fc *fileContext, mutationID int, originalStartLine int64, originalSourceCode, fmtOriginal []byte) {
	mutant := models.Mutant{}
	mutant.Mutator.MutatorName = m.Name
	mutant.Mutator.OriginalFilePath = fc.sourceFile
	mutant.Mutator.OriginalSourceCode = string(originalSourceCode)
	mutant.Mutator.OriginalStartLine = originalStartLine

	mutationFile := fmt.Sprintf("%s.%d", fc.mutatedFile, mutationID)
	_, duplicate, err := saveAST(r.blacklist, mutationFile, fc.fset, fc.src, fmtOriginal)
	if err != nil {
		out := fmt.Sprintf("INTERNAL ERROR %s\n", err.Error())
		fmt.Fprintf(r.stdout, "%s", out)
		mutant.ProcessOutput = out
		r.mu.Lock()
		r.report.Errored = append(r.report.Errored, mutant)
		r.report.Stats.ErrorCount++
		r.mu.Unlock()
		return
	}

	if duplicate {
		r.mu.Lock()
		r.report.Stats.DuplicatedCount++
		r.mu.Unlock()
		return
	}

	job := execJob{
		opts:           r.opts,
		pkg:            fc.pkg,
		originalFile:   fc.sourceFile,
		mutationFile:   mutationFile,
		mutant:         mutant,
		absFile:        fc.absFile,
		coverProfile:   fc.coverProfile,
		execs:          r.execs,
		perTestProf:    fc.perTestProf,
		extraTestFlags: r.extraTestFlags,
		runMutantID:    r.opts.Exec.RunMutantID,
		moduleRoot:     r.moduleRoot,
	}

	if r.jobs != nil {
		r.jobs <- job
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
	fmt.Fprintln(stdout, "Note: this count is an upper bound. Identical mutations across files are deduplicated during an actual run.")
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

func runNoopChecks(opts *models.Options, pkgs []importing.Package, execs []string, extraTestFlags []string) int {
	if !opts.General.Noop || opts.Exec.NoExec {
		return 0 // returnOk
	}
	if len(execs) > 0 {
		fmt.Fprintln(os.Stderr, "Warning: --noop is not supported with --exec; skipping initial test run")
		return 0 // returnOk
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
			return 3 // returnError
		}
	}
	console.Verbose(opts, "Noop check passed — all packages green before mutation")
	return 0 // returnOk
}

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

func runCoverageProfile(pkg, profilePath string) error {
	cmd := exec.Command("go", "test", "-coverprofile="+profilePath, pkg)
	cmd.Env = os.Environ()
	_ = cmd.Run()
	if _, err := os.Stat(profilePath); err != nil {
		return fmt.Errorf("coverage profile not created for %q", pkg)
	}
	return nil
}

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

func startWorkerPool(opts *models.Options, numWorkers int, report *models.Report, mu *sync.Mutex, stdout io.Writer, gitChangedLines gitdiff.ChangedLines) (chan execJob, *sync.WaitGroup) {
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
				runExecJob(job, report, mu, stdout, gitChangedLines)
			}
		}()
	}
	return jobs, &wg
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

func finalizeResults(stdout, stderr io.Writer, opts *models.Options, report *models.Report, bl *baseline.File, moduleRoot string) int {
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
	return checkQualityGates(opts, report, bl, moduleRoot)
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
		printGitHubAnnotations(report)
	}
}

func writeAllReports(stderr io.Writer, opts *models.Options, report *models.Report, moduleRoot string) int {
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

func checkQualityGates(opts *models.Options, report *models.Report, bl *baseline.File, moduleRoot string) int {
	if opts.Score.IgnoreMsiWithNoMutations && report.Stats.TotalMutantsCount == 0 {
		return returnOk
	}

	minMsi := resolveThreshold(opts.Score.MinMsi, opts.Config.MinMsi)
	minCoveredMsi := resolveThreshold(opts.Score.MinCoveredMsi, opts.Config.MinCoveredMsi)

	escapedFail := checkEscapedGate(opts, report, bl, moduleRoot)
	msiFail := checkMsiGate(report, minMsi)
	coveredFail := checkCoveredMsiGate(report, minCoveredMsi)

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

func checkMsiGate(report *models.Report, minMsi float64) bool {
	msiPct := report.Stats.Msi * 100
	if minMsi >= 0 && msiPct < minMsi {
		fmt.Fprintf(os.Stderr, "MSI %.2f%% is below minimum required %.2f%%\n", msiPct, minMsi)
		return true
	}
	return false
}

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

func runExecJob(job execJob, stats *models.Report, mu *sync.Mutex, stdout io.Writer, gitChangedLines gitdiff.ChangedLines) {
	opts := job.opts
	mutant := job.mutant

	if skipForGitDiff(job, gitChangedLines) || skipForMutantID(job) {
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

func skipForGitDiff(job execJob, gitChangedLines gitdiff.ChangedLines) bool {
	if gitChangedLines == nil {
		return false
	}
	lineNum := int(job.mutant.Mutator.OriginalStartLine)
	if gitdiff.IsLineChanged(gitChangedLines, job.absFile, lineNum) {
		return false
	}
	console.Debug(job.opts, "Skip %q at line %d (not in git diff)", job.mutationFile, lineNum)
	return true
}

func toRelPath(absOrRel, moduleRoot string) string {
	rel, err := filepath.Rel(moduleRoot, absOrRel)
	if err != nil {
		return filepath.ToSlash(absOrRel)
	}
	return filepath.ToSlash(rel)
}

func skipForMutantID(job execJob) bool {
	if job.runMutantID == "" {
		return false
	}
	relFile := toRelPath(job.mutant.Mutator.OriginalFilePath, job.moduleRoot)
	diffOut, _ := exec.Command("diff", "--label=Original", "--label=New", "-u", job.originalFile, job.mutationFile).CombinedOutput()
	id := baseline.MutantID(relFile, job.mutant.Mutator.MutatorName, string(diffOut))
	return id != job.runMutantID
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

	diff, code := computeDiff(file, mutationFile, mutant)
	if code != 0 {
		return code
	}

	overlayName, code := prepareOverlay(file, mutationFile)
	if code != 0 {
		return code
	}
	defer os.Remove(overlayName)

	execExitCode := runGoTest(opts, pkg, overlayName, perTestProf, absFile, int(mutant.Mutator.OriginalStartLine), extraTestFlags)

	mutant.Diff = string(diff)
	return mapTestExitToResult(execExitCode)
}

func computeDiff(file, mutationFile string, mutant *models.Mutant) ([]byte, int) {
	diff, err := exec.Command("diff", "--label=Original", "--label=New", "-u", file, mutationFile).CombinedOutput()
	if mutant.Mutator.OriginalStartLine <= 0 {
		mutant.Mutator.OriginalStartLine = parser.FindOriginalStartLine(diff)
	}

	diffExitCode, ok := commandExitCode(err)
	if !ok {
		fmt.Fprintf(os.Stderr, "mutago: diff error: %v\n", err)
		return nil, 3
	}
	if diffExitCode != 0 && diffExitCode != 1 {
		fmt.Fprintf(os.Stderr, "mutago: diff exited with code %d\n", diffExitCode)
		return nil, 3
	}
	return diff, 0
}

func prepareOverlay(file, mutationFile string) (string, int) {
	absOrig, _ := filepath.Abs(file)
	absMut, _ := filepath.Abs(mutationFile)
	overlayName, err := writeOverlayFile(absOrig, absMut)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mutago: cannot create overlay file: %v\n", err)
		return "", 3
	}
	return overlayName, 0
}

func runGoTest(opts *models.Options, pkg *types.Package, overlayName string, perTestProf *coverage.PerTestProfile, absFile string, startLine int, extraTestFlags []string) int {
	pkgName := pkg.Path()
	if opts.Test.Recursive {
		pkgName += "/..."
	}

	runFilter := perTestRunFilter(perTestProf, absFile, startLine)

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
	return execExitCode
}

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
		fmt.Fprintf(os.Stderr, "mutago: custom exec failed to start: %v\n", err)
		return 3
	}

	err := execCommand.Wait()
	execExitCode, ok := commandExitCode(err)
	if !ok {
		fmt.Fprintf(os.Stderr, "mutago: custom exec wait error: %v\n", err)
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

func recordMutantResult(opts *models.Options, stats *models.Report, mutant models.Mutant, execExitCode int, msg string) {
	switch execExitCode {
	case 0:
		recordKilled(opts, stats, mutant, msg)
	case 1:
		recordEscaped(opts, stats, mutant, msg)
	case 2:
		recordSkipped(opts, stats, mutant, msg)
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
