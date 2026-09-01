package models

// Options Main config structure
type Options struct {
	General struct {
		Debug                bool   `long:"debug" description:"Debug log output"`
		DoNotRemoveTmpFolder bool   `long:"do-not-remove-tmp-folder" description:"Do not remove the tmp folder where all mutations are saved to"`
		Help                 bool   `long:"help" description:"Show this help message"`
		Noop                 bool   `long:"noop" description:"No-op: the baseline test check (run the suite once unmutated first, exit with an error if it fails) is now always on by default. This flag is kept for backward compatibility and has no effect."`
		DryRun               bool   `long:"dry-run" description:"Count mutations per file and mutator without generating files or running tests; prints a summary table and exits 0. The count is an upper bound — identical mutations across files are deduplicated in a real run."`
		NoDiffs              bool   `long:"no-diffs" description:"Suppress diff output for all mutation results (useful in CI where diffs are noisy and the JSON report is consumed instead)"`
		OutputStatuses       string `long:"output-statuses" description:"Show only these result statuses in the terminal: k=killed e=escaped s=skipped n=not-covered x=errored (e.g. --output-statuses=ke). Does not affect JSON reports. Overrides --quiet when set."`
		Quiet                bool   `long:"quiet" description:"Only print escaped mutants and the summary (suppress killed/skipped output). Combine with --no-diffs to also suppress escaped-mutant diffs."`
		Verbose              bool   `long:"verbose" description:"Verbose log output"`
		Workers              int    `long:"workers" description:"Number of parallel workers for mutation execution (0 = all CPUs). Forced to 1 when --exec is set." default:"0"`
		Config               string `long:"config" description:"Path to config file"`
		HTMLOutput           bool   `long:"html-output" description:"Generates a mutago-report.html file after testing is complete"`
	} `group:"General options"`

	Files struct {
		Blacklist []string `long:"blacklist" description:"List of files containing MD5 checksums (one per line) of mutations to ignore."`
		ListFiles bool     `long:"list-files" description:"List found files"`
		PrintAST  bool     `long:"print-ast" description:"Print the ASTs of all given files and exit"`
	} `group:"File options"`

	Mutator struct {
		DisableMutators []string `long:"disable" description:"Disable mutator by their name or using * as a suffix pattern (in order to check remaining enabled mutators use --verbose option)"`
		ListMutators    bool     `long:"list-mutators" description:"List all available mutators (including disabled)"`
	} `group:"Mutator options"`

	Filter struct {
		Match string `long:"match" description:"Only functions are mutated that confirm to the arguments regex"`
	} `group:"Filter options"`

	Exec struct {
		Exec               string  `long:"exec" description:"Execute this command for every mutation (by default the built-in exec command is used)"`
		NoExec             bool    `long:"no-exec" description:"Skip the built-in exec command and just generate the mutations"`
		Timeout            uint    `long:"exec-timeout" description:"Sets a timeout for the command execution (in seconds)" default:"10"`
		RunMutantID        string  `long:"run-mutant-id" description:"Run only the mutant with this stable ID. Copy the id field from mutago-agentic.json to target a specific survivor. Quality gates and the summary line are suppressed in this mode."`
		Coverage           bool    `long:"coverage" description:"Run go test -coverprofile before mutating to compute covered-code MSI and mark uncovered mutants"`
		PerTest            bool    `long:"per-test" description:"Build a per-test coverage map and run only covering tests for each mutation. Fastest on packages with slow tests; pairs well with --coverage."`
		TestFlags          string  `long:"test-flags" description:"Extra flags passed to each 'go test' invocation. Use the = form to pass flag values: --test-flags='-short'. Ignored when --exec is set."`
		TimeoutCoefficient float64 `long:"timeout-coefficient" description:"Set per-mutation timeout as a multiple of an uncached baseline test-suite run (e.g. 3 = 3× the clean run). Overrides --exec-timeout when > 0." default:"0"`
	} `group:"Exec options"`

	// GitDiff limits mutation to lines changed since a git base ref.
	// Pair with --ignore-msi-with-no-mutations for clean CI on unchanged packages.
	GitDiff struct {
		Lines bool   `long:"git-diff-lines" description:"Only mutate lines changed since the git diff base"`
		Base  string `long:"git-diff-base" description:"Git ref to diff against for --git-diff-lines (default: auto-detected from origin/HEAD, falling back to master)"`
	} `group:"Git diff options"`

	Logger struct {
		GitHub      bool `long:"logger-github" description:"Emit escaped mutants as GitHub Actions ::warning annotations"`
		GitLab      bool `long:"logger-gitlab" description:"Write mutago-gitlab.json in GitLab Code Quality format (escaped mutants as code-quality issues)"`
		SummaryJSON bool `long:"logger-summary-json" description:"Write a compact stats-only JSON to mutago-summary.json"`
		AgenticJSON bool `long:"logger-agentic-json" description:"Write mutago-agentic.json with enriched escaped-mutant data designed for LLM consumption"`
	} `group:"Logger options"`

	// Baseline tracks known-surviving mutants so CI only fails on new regressions.
	// When the baseline file does not exist, the check is skipped (opt-in).
	Baseline struct {
		File   string `long:"baseline" description:"Path to baseline file of known-surviving mutants" default:"mutago-baseline.json"`
		Update bool   `long:"update-baseline" description:"Write current escaped mutants to the baseline file then exit 0"`
	} `group:"Baseline options"`

	Test struct {
		Recursive bool `long:"test-recursive" description:"Defines if the executer should test recursively"`
	} `group:"Test options"`

	// Quality gates: fail with exit code 4 when metrics fall below thresholds.
	// -1 is the "not set" sentinel so that --min-msi 0 is distinguishable from
	// "flag not provided", and CLI always takes precedence over config file.
	Score struct {
		MinMsi                   float64 `long:"min-msi" description:"Minimum required MSI (0-100). Exit code 4 when not met." default:"-1"`
		MinCoveredMsi            float64 `long:"min-covered-msi" description:"Minimum required covered-MSI (0-100). Exit code 4 when not met." default:"-1"`
		IgnoreMsiWithNoMutations bool    `long:"ignore-msi-with-no-mutations" description:"Exit 0 even when MSI thresholds are not met if no mutations were generated (useful with --git-diff-lines)"`
		FailOnEscaped            bool    `long:"fail-on-escaped" description:"Exit code 4 if any mutant escapes, without requiring --min-msi"`
	} `group:"Score options"`

	Remaining struct {
		Targets []string `description:"Packages, directories and files even with patterns (by default the current directory)"`
	} `positional-args:"true" required:"true"`

	Config struct {
		SkipFileWithoutTest  bool     `yaml:"skip_without_test"`
		SkipFileWithBuildTag bool     `yaml:"skip_with_build_tags"`
		JSONOutput           bool     `yaml:"json_output"`
		HTMLOutput           bool     `yaml:"html_output"`
		SilentMode           bool     `yaml:"silent_mode"`
		ExcludeDirs          []string `yaml:"exclude_dirs"`
		MinMsi               float64  `yaml:"min_msi"`
		MinCoveredMsi        float64  `yaml:"min_covered_msi"`
		DisableMutators      []string `yaml:"disable_mutators"`
		EnableMutators       []string `yaml:"enable_mutators"`
		IgnoreSourceLines    []string `yaml:"ignore_source_lines"`
	}
}
