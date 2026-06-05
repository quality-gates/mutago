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

var errDuplicate = fmt.Errorf("duplicate AST")

func checkArguments(args []string, opts *models.Options) (bool, int) {
	p := flags.NewNamedParser("mutago", flags.None)
	p.ShortDescription = "Mutation testing for Go source code"

	if _, err := p.AddGroup("mutago", "mutago arguments", opts); err != nil {
		return true, exitError(err.Error())
	}

	completion := len(os.Getenv("GO_FLAGS_COMPLETION")) > 0
	_, err := p.ParseArgs(args)

	if handled, code := handleHelpOrList(opts, args, p); handled {
		return true, code
	}

	if err != nil {
		return true, exitError(err.Error())
	}
	if completion {
		return true, returnBashCompletion
	}
	if opts.General.Debug {
		opts.General.Verbose = true
	}
	if exit, code := loadConfig(opts); exit {
		return true, code
	}
	return false, 0
}

func handleHelpOrList(opts *models.Options, args []string, p *flags.Parser) (bool, int) {
	completion := len(os.Getenv("GO_FLAGS_COMPLETION")) > 0
	if (opts.General.Help || len(args) == 0) && !completion {
		p.WriteHelp(os.Stdout)
		return true, returnOk
	}
	if opts.Mutator.ListMutators {
		for _, name := range mutator.List() {
			fmt.Println(name)
		}
		return true, returnOk
	}
	return false, 0
}

func loadConfig(opts *models.Options) (bool, int) {
	if opts.General.Config == "" {
		return false, 0
	}
	yamlFile, err := os.ReadFile(opts.General.Config)
	if err != nil {
		return true, exitError("Could not read config file: %q", opts.General.Config)
	}
	dec := yaml.NewDecoder(bytes.NewReader(yamlFile))
	dec.KnownFields(true)
	err = dec.Decode(&opts.Config)
	if err != nil {
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

type runContext struct {
	bl                *baseline.File
	mutationBlackList map[string]struct{}
	mutators          []mutatorItem
	execs             []string
	extraTestFlags    []string
	tmpDir            string
	modulePath        string
	moduleRoot        string
	gitChangedLines   gitdiff.ChangedLines
	pkgs              []importing.Package
}

func initRunContext(opts *models.Options) (*runContext, int) {
	files := importing.FilesOfArgs(opts.Remaining.Targets, opts)
	if len(files) == 0 {
		return nil, exitError("Could not find any suitable Go source files")
	}

	bl, err := baseline.Load(opts.Baseline.File)
	if err != nil {
		return nil, exitError("Cannot load baseline: %v", err)
	}

	if handled, code := handleInfoFlags(opts, files); handled {
		return nil, code
	}

	mutationBlackList, err := loadBlacklist(opts.Files.Blacklist)
	if err != nil {
		return nil, exitError(err.Error())
	}

	mutators := buildActiveMutators(opts)
	execs, extraTestFlags := parseExecFlags(opts)

	tmpDir, err := os.MkdirTemp("", "mutago-")
	if err != nil {
		return nil, exitError("Cannot create temp directory: %v", err)
	}
	console.Verbose(opts, "Save mutations into %q", tmpDir)

	modulePath := detectModulePath()
	moduleRoot := detectModuleRoot()

	gitChangedLines, err := loadGitDiffLines(opts)
	if err != nil {
		return nil, exitError("Cannot load git diff: %v", err)
	}

	pkgs := importing.PackagesWithFilesOfArgs(opts.Remaining.Targets, opts)

	if exitCode := runNoopChecks(opts, pkgs, execs, extraTestFlags); exitCode != 0 {
		return nil, exitCode
	}

	applyAdaptiveTimeout(opts, pkgs, execs, extraTestFlags)

	return &runContext{
		bl:                bl,
		mutationBlackList: mutationBlackList,
		mutators:          mutators,
		execs:             execs,
		extraTestFlags:    extraTestFlags,
		tmpDir:            tmpDir,
		modulePath:        modulePath,
		moduleRoot:        moduleRoot,
		gitChangedLines:   gitChangedLines,
		pkgs:              pkgs,
	}, 0
}

func mainCmd(args []string) int {
	var opts = &models.Options{}

	if exit, exitCode := checkArguments(args, opts); exit {
		return exitCode
	}

	ctx, exitCode := initRunContext(opts)
	if exitCode != 0 {
		return exitCode
	}

	report := &models.Report{}
	var reportMu sync.Mutex

	numWorkers := calcNumWorkers(opts, ctx.execs)
	console.Verbose(opts, "Running with %d parallel worker(s)", numWorkers)

	jobs, jobWg := startWorkerPool(opts, numWorkers, report, &reportMu)
	stopProgress, progressWg := startProgressMonitor(opts, report, &reportMu)

	runner := &mutationRunner{
		opts:                opts,
		mutators:            ctx.mutators,
		blacklist:           ctx.mutationBlackList,
		tmpDir:              ctx.tmpDir,
		numWorkers:          numWorkers,
		execs:               ctx.execs,
		extraTestFlags:      ctx.extraTestFlags,
		report:              report,
		mu:                  &reportMu,
		modulePath:          ctx.modulePath,
		moduleRoot:          ctx.moduleRoot,
		gitChangedLines:     ctx.gitChangedLines,
		jobs:                jobs,
		dryRunMutatorTotals: make(map[string]int),
	}

	dryRunTotal, loopCode := runner.mutateAllPackages(ctx.pkgs)

	shutdownAndCleanup(opts, jobs, jobWg, stopProgress, progressWg, ctx.tmpDir)

	if loopCode != returnOk {
		return loopCode
	}

	if opts.General.DryRun {
		printDryRunReport(dryRunTotal, runner.dryRunMutatorTotals)
		return returnOk
	}

	report.Calculate()

	if handled, code := handleBaselineUpdate(opts, report, ctx.moduleRoot); handled {
		return code
	}

	printResultsIfNeeded(opts, report)

	if code := writeAllReports(opts, report, ctx.moduleRoot); code != returnOk {
		return code
	}

	if opts.Exec.RunMutantID != "" {
		return returnOk
	}
	return checkQualityGates(opts, report, ctx.bl, ctx.moduleRoot)
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

func shouldSkipProgressMonitor(opts *models.Options) bool {
	if !isTerminal() {
		return true
	}
	if opts.General.Verbose || opts.General.Debug {
		return true
	}
	return opts.Config.SilentMode || opts.Exec.NoExec || opts.General.DryRun
}

// startProgressMonitor launches a goroutine printing live kill/escape/skip counts.
// Returns nil, nil when conditions are not met (non-terminal, verbose, silent, etc.).
func startProgressMonitor(opts *models.Options, report *models.Report, mu *sync.Mutex) (chan struct{}, *sync.WaitGroup) {
	if shouldSkipProgressMonitor(opts) {
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

type astTarget struct {
	pkg     *types.Package
	info    *types.Info
	file    string
	fset    *token.FileSet
	src     ast.Node
	node    ast.Node
	tmpFile string
	absFile string
	filters []filter.NodeFilter
}

type mutationRunner struct {
	opts                *models.Options
	mutators            []mutatorItem
	blacklist           map[string]struct{}
	tmpDir              string
	numWorkers          int
	execs               []string
	extraTestFlags      []string
	report              *models.Report
	mu                  *sync.Mutex
	modulePath          string
	moduleRoot          string
	gitChangedLines     gitdiff.ChangedLines
	jobs                chan<- execJob
	dryRunMutatorTotals map[string]int
}

func (r *mutationRunner) mutateAllPackages(pkgs []importing.Package) (dryRunTotal int, exitCode int) {
	for _, importPkg := range pkgs {
		coverProfile, perTestProf := r.loadCoverageProfiles(importPkg)
		for _, file := range importPkg.Files {
			count, code := r.processMutationFile(file, coverProfile, perTestProf)
			if code != returnOk {
				return dryRunTotal, code
			}
			dryRunTotal += count
		}
	}
	return dryRunTotal, returnOk
}

func (r *mutationRunner) loadCoverageProfiles(importPkg importing.Package) (*coverage.Profile, *coverage.PerTestProfile) {
	var coverProfile *coverage.Profile
	if !r.opts.General.DryRun {
		coverProfile = buildCoverageProfile(r.opts, importPkg.Files, r.tmpDir, r.modulePath)
		if coverProfile != nil {
			r.mu.Lock()
			r.report.HasCoverage = true
			r.mu.Unlock()
		}
	}
	var perTestProf *coverage.PerTestProfile
	if r.opts.Exec.PerTest && !r.opts.Exec.NoExec && !r.opts.General.DryRun && len(r.execs) == 0 {
		perTestProf = buildPerTestCoverageProfile(r.opts, importPkg.Files, r.modulePath, r.tmpDir, r.numWorkers, r.extraTestFlags)
	}
	return coverProfile, perTestProf
}

func (r *mutationRunner) processMutationFile(
	file string,
	coverProfile *coverage.Profile,
	perTestProf *coverage.PerTestProfile,
) (int, int) {
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

	err = os.MkdirAll(r.tmpDir+"/"+filepath.Dir(file), 0755)
	if err != nil {
		return 0, exitError("Cannot create mutation directory: %v", err)
	}

	tmpFile := r.tmpDir + "/" + file
	originalFile := fmt.Sprintf("%s.original", tmpFile)
	err = osutil.CopyFile(file, originalFile)
	if err != nil {
		return 0, exitError("Cannot copy original file: %v", err)
	}
	console.Debug(r.opts, "Save original into %q", originalFile)

	absFile, _ := filepath.Abs(file)
	mutationID := 0

	target := &astTarget{
		pkg:     pkg,
		info:    info,
		file:    file,
		fset:    fset,
		src:     src,
		node:    src,
		tmpFile: tmpFile,
		absFile: absFile,
		filters: nodeFilters,
	}

	if r.opts.Filter.Match != "" {
		m, err := regexp.Compile(r.opts.Filter.Match)
		if err != nil {
			return 0, exitError("Match regex is not valid: %v", err)
		}
		for _, f := range astutil.Functions(src) {
			if m.MatchString(f.Name.Name) {
				target.node = f
				mutationID = r.mutate(mutationID, target, coverProfile, perTestProf)
			}
		}
		return mutationID, returnOk
	}

	mutationID = r.mutate(mutationID, target, coverProfile, perTestProf)
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
	if !opts.General.DoNotRemoveTmpFolder {
		if err := os.RemoveAll(tmpDir); err != nil {
			fmt.Fprintf(os.Stderr, "mutago: cannot remove %s: %v\n", tmpDir, err)
			return
		}
		console.Debug(opts, "Remove %q", tmpDir)
	}
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

type reportWriter struct {
	name      string
	shouldRun func(opts *models.Options) bool
	write     func(opts *models.Options, report *models.Report, moduleRoot string) error
}

var reportWriters = []reportWriter{
	{
		name: "JSON",
		shouldRun: func(opts *models.Options) bool {
			return opts.General.Config == "" || opts.Config.JSONOutput
		},
		write: func(opts *models.Options, report *models.Report, _ string) error {
			if err := reportmaker.MakeJSONReport(*report); err != nil {
				return err
			}
			console.Verbose(opts, "Save report into %q", models.ReportFileName)
			return nil
		},
	},
	{
		name: "Summary JSON",
		shouldRun: func(opts *models.Options) bool {
			return opts.Logger.SummaryJSON
		},
		write: func(opts *models.Options, report *models.Report, _ string) error {
			if err := reportmaker.MakeSummaryJSONReport(report.Stats); err != nil {
				return err
			}
			console.Verbose(opts, "Save summary into %q", models.ReportSummaryJSONFileName)
			return nil
		},
	},
	{
		name: "Agentic JSON",
		shouldRun: func(opts *models.Options) bool {
			return opts.Logger.AgenticJSON
		},
		write: func(opts *models.Options, report *models.Report, moduleRoot string) error {
			if err := reportmaker.MakeAgenticJSONReport(*report, moduleRoot); err != nil {
				return err
			}
			console.Verbose(opts, "Save agentic report into %q", models.ReportAgenticJSONFileName)
			return nil
		},
	},
	{
		name: "GitLab",
		shouldRun: func(opts *models.Options) bool {
			return opts.Logger.GitLab
		},
		write: func(opts *models.Options, report *models.Report, moduleRoot string) error {
			if err := reportmaker.MakeGitLabReport(*report, moduleRoot); err != nil {
				return err
			}
			console.Verbose(opts, "Save GitLab report into %q", models.ReportGitLabJSONFileName)
			return nil
		},
	},
	{
		name: "HTML",
		shouldRun: func(opts *models.Options) bool {
			return opts.Config.HTMLOutput || opts.General.HTMLOutput
		},
		write: func(opts *models.Options, report *models.Report, _ string) error {
			if err := reportmaker.MakeHTMLReport(*report); err != nil {
				return err
			}
			console.Verbose(opts, "Save report into %q", models.ReportHTMLFileName)
			return nil
		},
	},
}

// writeAllReports writes every enabled report format; returns first error exit code.
func writeAllReports(opts *models.Options, report *models.Report, moduleRoot string) int {
	for _, rw := range reportWriters {
		if rw.shouldRun(opts) {
			if err := rw.write(opts, report, moduleRoot); err != nil {
				return exitError(err.Error())
			}
		}
	}
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

func checkEscapedMutants(opts *models.Options, report *models.Report, bl *baseline.File, moduleRoot string) bool {
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

func checkMsiThresholds(report *models.Report, minMsi, minCoveredMsi float64) bool {
	failed := false
	msiPct := report.Stats.Msi * 100
	covMsiPct := report.Stats.CoveredCodeMsi * 100

	if minMsi >= 0 && msiPct < minMsi {
		fmt.Fprintf(os.Stderr, "MSI %.2f%% is below minimum required %.2f%%\n", msiPct, minMsi)
		failed = true
	}
	if minCoveredMsi > 0 {
		if !report.HasCoverage {
			fmt.Fprintf(os.Stderr, "Covered MSI cannot be checked: --coverage was not enabled (score is always 0 without a profile)\n")
			failed = true
		} else if covMsiPct < minCoveredMsi {
			fmt.Fprintf(os.Stderr, "Covered MSI %.2f%% is below minimum required %.2f%%\n", covMsiPct, minCoveredMsi)
			failed = true
		}
	}
	return failed
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
	minMsi := opts.Score.MinMsi
	if minMsi < 0 {
		minMsi = opts.Config.MinMsi
	}
	minCoveredMsi := opts.Score.MinCoveredMsi
	if minCoveredMsi < 0 {
		minCoveredMsi = opts.Config.MinCoveredMsi
	}

	failed := checkEscapedMutants(opts, report, bl, moduleRoot)
	if checkMsiThresholds(report, minMsi, minCoveredMsi) {
		failed = true
	}

	if failed {
		return returnMsiThresholdNotMet
	}
	return returnOk
}

func (r *mutationRunner) mutate(
	mutationID int,
	target *astTarget,
	coverProfile *coverage.Profile,
	perTestProf *coverage.PerTestProfile,
) int {
	// Read the original source once per file — it never changes during mutation.
	originalSourceCode, err := os.ReadFile(target.file)
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
		console.Debug(r.opts, "Mutator %s", m.Name)

		mutatorAnnotated := annotation.DecoratorFilter(m.Mutator, m.Name, target.filters...)

		changed := mutago.MutateWalk(target.pkg, target.info, target.node, mutatorAnnotated)

		for {
			_, ok := <-changed
			if !ok {
				break
			}

			if r.opts.General.DryRun {
				// Count only — no file writes, no job submission.
				dryRunCounts[m.Name]++
				if r.dryRunMutatorTotals != nil {
					r.dryRunMutatorTotals[m.Name]++
				}
				changed <- true
				<-changed
				changed <- true
				mutationID++
				continue
			}

			mutant := models.Mutant{}
			mutant.Mutator.MutatorName = m.Name
			mutant.Mutator.OriginalFilePath = target.file
			mutant.Mutator.OriginalSourceCode = string(originalSourceCode)

			mutationFile := fmt.Sprintf("%s.%d", target.tmpFile, mutationID)
			checksum, err := saveAST(r.blacklist, mutationFile, target.fset, target.src, fmtOriginal)
			
			r.recordASTResult(mutationFile, checksum, err, &mutant, target, coverProfile, perTestProf)

			// Release the MutateWalk goroutine to reset the AST and advance.
			changed <- true
			<-changed
			changed <- true

			mutationID++
		}
	}

	printDryRunFileSummary(target.file, dryRunCounts)

	return mutationID
}

func (r *mutationRunner) recordASTResult(
	mutationFile string,
	checksum string,
	err error,
	mutant *models.Mutant,
	target *astTarget,
	coverProfile *coverage.Profile,
	perTestProf *coverage.PerTestProfile,
) {
	if err == errDuplicate {
		console.Debug(r.opts, "%q is a duplicate, we ignore it", mutationFile)
		r.mu.Lock()
		r.report.Stats.DuplicatedCount++
		r.mu.Unlock()
		return
	}
	if err != nil {
		out := fmt.Sprintf("INTERNAL ERROR %s\n", err.Error())
		fmt.Printf("%s", out)
		mutant.ProcessOutput = out
		r.mu.Lock()
		r.report.Errored = append(r.report.Errored, *mutant)
		r.report.Stats.ErrorCount++
		r.mu.Unlock()
		return
	}
	console.Debug(r.opts, "Save mutation into %q with checksum %s", mutationFile, checksum)
	if r.jobs != nil {
		r.jobs <- execJob{
			opts:            r.opts,
			pkg:             target.pkg,
			originalFile:    target.file,
			mutationFile:    mutationFile,
			mutant:          *mutant,
			absFile:         target.absFile,
			coverProfile:    coverProfile,
			gitChangedLines: r.gitChangedLines,
			execs:           r.execs,
			perTestProf:     perTestProf,
			extraTestFlags:  r.extraTestFlags,
			runMutantID:     r.opts.Exec.RunMutantID,
			moduleRoot:      r.moduleRoot,
		}
	}
}

// printDryRunFileSummary prints per-mutator dry-run counts for a single file.
func printDryRunFileSummary(originalFile string, counts map[string]int) {
	if len(counts) == 0 {
		return
	}
	loc := originalFile
	if rel, err := filepath.Rel(".", originalFile); err == nil {
		loc = filepath.ToSlash(rel)
	}
	fmt.Printf("%s\n", loc)
	mutatorNames := make([]string, 0, len(counts))
	for name := range counts {
		mutatorNames = append(mutatorNames, name)
	}
	sort.Strings(mutatorNames)
	for _, name := range mutatorNames {
		fmt.Printf("  %-40s %d\n", name, counts[name])
	}
}

// shouldSkipGitDiff checks if the mutant should be skipped based on git diff.
func shouldSkipGitDiff(job execJob) bool {
	if job.gitChangedLines == nil {
		return false
	}
	diffOut, _ := exec.Command("diff", "--label=Original", "--label=New", "-u", job.originalFile, job.mutationFile).CombinedOutput()
	lineNum := int(parser.FindOriginalStartLine(diffOut))
	if !gitdiff.IsLineChanged(job.gitChangedLines, job.absFile, lineNum) {
		console.Debug(job.opts, "Skip %q at line %d (not in git diff)", job.mutationFile, lineNum)
		return true
	}
	return false
}

// shouldSkipMutantID checks if the mutant matches the runMutantID filter.
func shouldSkipMutantID(job execJob, mutant *models.Mutant) bool {
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
	id := baseline.MutantID(relFile, mutant.Mutator.MutatorName, string(diffOut))
	return id != job.runMutantID
}

// getMutantLocationMsg formats the location message for a mutant.
func getMutantLocationMsg(mutant *models.Mutant) string {
	loc := mutant.Mutator.OriginalFilePath
	if rel, err := filepath.Rel(".", loc); err == nil {
		loc = filepath.ToSlash(rel)
	}
	if mutant.Mutator.OriginalStartLine > 0 {
		loc = fmt.Sprintf("%s:%d", loc, mutant.Mutator.OriginalStartLine)
	}
	return fmt.Sprintf("%s (%s)", loc, mutant.Mutator.MutatorName)
}

// runExecJob executes a single mutation job in a worker goroutine.
// It applies the git-diff filter, runs go test via overlay (or --exec),
// checks coverage, and records the result under mu.
func runExecJob(job execJob, stats *models.Report, mu *sync.Mutex) {
	opts := job.opts
	mutant := job.mutant

	if shouldSkipGitDiff(job) {
		return
	}

	if shouldSkipMutantID(job, &mutant) {
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

	msg := getMutantLocationMsg(&mutant)

	mu.Lock()
	defer mu.Unlock()
	if notCovered {
		recordNotCoveredMutant(opts, stats, mutant, msg)
		return
	}
	recordMutantResult(opts, stats, mutant, execExitCode, msg)
}

// recordNotCoveredMutant records a mutant that is not covered by tests.
func recordNotCoveredMutant(opts *models.Options, stats *models.Report, mutant models.Mutant, msg string) {
	out := fmt.Sprintf("NOT COVERED %s\n", msg)
	if statusVisible(opts, 'n') {
		console.PrintSkip(out)
	}
	mutant.ProcessOutput = out
	stats.NotCovered = append(stats.NotCovered, mutant)
	stats.Stats.NotCoveredCount++
}

// recordKilledMutant records a mutant that was successfully killed.
func recordKilledMutant(opts *models.Options, stats *models.Report, mutant models.Mutant, msg string) {
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

// recordEscapedMutant records a mutant that escaped.
func recordEscapedMutant(opts *models.Options, stats *models.Report, mutant models.Mutant, msg string) {
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

// recordSkippedMutant records a mutant that was skipped (failed compilation).
func recordSkippedMutant(opts *models.Options, stats *models.Report, mutant models.Mutant, msg string) {
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

// recordErrorMutant records a mutant that encountered an error during execution.
func recordErrorMutant(opts *models.Options, stats *models.Report, mutant models.Mutant, msg string) {
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

// recordMutantResult appends the mutant to the appropriate stats bucket and prints status.
func recordMutantResult(opts *models.Options, stats *models.Report, mutant models.Mutant, execExitCode int, msg string) {
	switch execExitCode {
	case 0:
		recordKilledMutant(opts, stats, mutant, msg)
	case 1:
		recordEscapedMutant(opts, stats, mutant, msg)
	case 2:
		recordSkippedMutant(opts, stats, mutant, msg)
	default:
		recordErrorMutant(opts, stats, mutant, msg)
	}
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

// getDiffAndStartLine runs diff on file and mutationFile, sets startLine on mutant, and returns the diff or an error.
func getDiffAndStartLine(file, mutationFile string, mutant *models.Mutant) ([]byte, int64, error) {
	diff, err := exec.Command("diff", "--label=Original", "--label=New", "-u", file, mutationFile).CombinedOutput()
	startLine := parser.FindOriginalStartLine(diff)
	mutant.Mutator.OriginalStartLine = startLine

	if err == nil {
		return diff, startLine, nil
	}

	e, ok := err.(*exec.ExitError)
	if !ok {
		return nil, 0, fmt.Errorf("diff error: %w", err)
	}

	diffExitCode := e.Sys().(syscall.WaitStatus).ExitStatus()
	if diffExitCode != 0 && diffExitCode != 1 {
		return nil, 0, fmt.Errorf("diff exited with code %d", diffExitCode)
	}
	return diff, startLine, nil
}

// createOverlayFile creates a temporary JSON overlay file mapping file to mutationFile.
func createOverlayFile(file, mutationFile string) (string, error) {
	absOrig, _ := filepath.Abs(file)
	absMut, _ := filepath.Abs(mutationFile)
	overlayData, _ := json.Marshal(struct {
		Replace map[string]string `json:"Replace"`
	}{Replace: map[string]string{absOrig: absMut}})

	overlayFile, err := os.CreateTemp("", "mutago-overlay-*.json")
	if err != nil {
		return "", fmt.Errorf("cannot create overlay file: %w", err)
	}
	if _, err := overlayFile.Write(overlayData); err != nil {
		overlayFile.Close()
		os.Remove(overlayFile.Name())
		return "", fmt.Errorf("cannot write overlay file: %w", err)
	}
	overlayFile.Close()
	return overlayFile.Name(), nil
}

// getRunFilter computes the -run filter for a mutation based on covering tests.
func getRunFilter(perTestProf *coverage.PerTestProfile, absFile string, startLine int64) string {
	if perTestProf != nil && startLine > 0 {
		if tests := perTestProf.CoveringTests(absFile, int(startLine)); len(tests) > 0 {
			return "^(" + strings.Join(tests, "|") + ")$"
		}
	}
	return ""
}

// runGoTestCmd runs the go test command using the overlay file and timeout.
func runGoTestCmd(opts *models.Options, overlayName string, extraTestFlags []string, runFilter string, pkgName string) ([]byte, int, error) {
	goTestArgs := []string{"test", "-overlay=" + overlayName, "-timeout", fmt.Sprintf("%ds", opts.Exec.Timeout)}
	goTestArgs = append(goTestArgs, extraTestFlags...)
	if runFilter != "" {
		goTestArgs = append(goTestArgs, "-run", runFilter)
	}
	goTestArgs = append(goTestArgs, pkgName)

	goTestCmd := exec.Command("go", goTestArgs...)
	goTestCmd.Env = os.Environ()
	test, err := goTestCmd.CombinedOutput()

	if err == nil {
		return test, 0, nil
	}

	e, ok := err.(*exec.ExitError)
	if !ok {
		return nil, 0, fmt.Errorf("go test error: %w", err)
	}

	execExitCode := e.Sys().(syscall.WaitStatus).ExitStatus()
	return test, execExitCode, nil
}

// mapGoTestExitCode maps go test exit codes (0 means escaped, 1 means killed).
func mapGoTestExitCode(execExitCode int) int {
	if execExitCode == 0 {
		return 1
	}
	if execExitCode == 1 {
		return 0
	}
	return execExitCode
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

	diff, startLine, err := getDiffAndStartLine(file, mutationFile, mutant)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mutago: %v\n", err)
		return 3
	}

	overlayName, err := createOverlayFile(file, mutationFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mutago: %v\n", err)
		return 3
	}
	defer os.Remove(overlayName)

	pkgName := pkg.Path()
	if opts.Test.Recursive {
		pkgName += "/..."
	}

	runFilter := getRunFilter(perTestProf, absFile, startLine)

	test, execExitCode, err := runGoTestCmd(opts, overlayName, extraTestFlags, runFilter, pkgName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mutago: %v\n", err)
		return 3
	}

	if opts.General.Debug {
		fmt.Printf("%s\n", test)
	}

	mutant.Diff = string(diff)
	return mapGoTestExitCode(execExitCode)
}

// runCustomExec runs the user-provided --exec command for a mutation.
func runCustomExec(opts *models.Options, pkg *types.Package, file string, mutationFile string, execs []string, mutant *models.Mutant) int {
	console.Debug(opts, "Execute %q for mutation", opts.Exec.Exec)

	extDiff, _ := exec.Command("diff", "--label=Original", "--label=New", "-u", file, mutationFile).CombinedOutput()
	mutant.Mutator.OriginalStartLine = parser.FindOriginalStartLine(extDiff)
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

	if err == nil {
		return 0
	}

	e, ok := err.(*exec.ExitError)
	if !ok {
		fmt.Fprintf(os.Stderr, "mutago: exec wait error: %v\n", err)
		return 3
	}

	return e.Sys().(syscall.WaitStatus).ExitStatus()
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
func saveAST(mutationBlackList map[string]struct{}, file string, fset *token.FileSet, node ast.Node, fmtOriginal []byte) (string, error) {
	var buf bytes.Buffer

	if err := printer.Fprint(&buf, fset, node); err != nil {
		return "", err
	}

	mutatedSrc, err := format.Source(buf.Bytes())
	if err != nil {
		return "", err
	}

	checksum := stableMutationKey(fmtOriginal, mutatedSrc)

	if _, ok := mutationBlackList[checksum]; ok {
		return checksum, errDuplicate
	}

	mutationBlackList[checksum] = struct{}{}

	return checksum, os.WriteFile(file, mutatedSrc, 0666)
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
