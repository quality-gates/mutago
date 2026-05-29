# CLI Reference

Run `mutago --help` for the authoritative flag list. The most commonly used flags are documented here.

## Targets

```
mutago [flags] <pkg|file|dir> ...
```

Targets can be Go source files, directories, or import paths. The `...` wildcard searches recursively. Test files (`_test.go`) are excluded automatically.

## Core flags

| Flag | Default | Description |
| :--- | :------ | :---------- |
| `--exec` | (built-in) | Custom exec command for testing each mutation |
| `--exec-timeout` | `10` | Seconds to wait before killing the test process |
| `--timeout-coefficient` | `0` (disabled) | Scale per-mutation timeout as a multiple of the baseline test-suite run time (e.g. `3` = 3× the clean run). Overrides `--exec-timeout` when set. |
| `--workers` | all CPUs | Number of parallel mutation workers |
| `--config` | — | Path to YAML config file |

## Output flags

| Flag | Description |
| :--- | :---------- |
| `--dry-run` | Count mutations per file and mutator without generating files or running tests; prints a summary table and exits 0 |
| `--noop` | Run the test suite once without any mutations first; exits immediately if the clean suite fails |
| `--no-diffs` | Suppress diff output for all mutation results (useful in CI where diffs are noisy and the JSON report is consumed instead) |
| `--output-statuses` | Show only listed result statuses in the terminal: `k`=killed `e`=escaped `s`=skipped `n`=not-covered `x`=errored (e.g. `--output-statuses=ke`). Does not affect JSON reports. Overrides `--quiet` when set. |
| `--quiet` | Suppress killed/skipped lines; show only escaped mutants and summary (equivalent to `--output-statuses=e`) |
| `--verbose` | Print full test output for each mutation |
| `--debug` | Print internal debug information |
| `--logger-github` | Emit escaped mutants as `::warning` GitHub Actions annotations |
| `--logger-gitlab` | Write `mutago-gitlab.json` in GitLab Code Quality format |
| `--logger-summary-json` | Write compact stats to `mutago-summary.json` |
| `--logger-agentic-json` | Write LLM-ready report to `mutago-agentic.json` |
| `--run-mutant-id` | Run only the mutant with this stable ID (copy the `id` field from `mutago-agentic.json`) |

## Quality gates

| Flag | Default | Description |
| :--- | :------ | :---------- |
| `--min-msi` | `-1` (disabled) | Fail (exit 4) if overall MSI is below this value (0–100) |
| `--min-covered-msi` | `-1` (disabled) | Fail (exit 4) if covered-code MSI is below this value |
| `--fail-on-escaped` | `false` | Fail (exit 4) if any mutant survives, without requiring `--min-msi` |
| `--ignore-msi-with-no-mutations` | `false` | Exit 0 when no mutations are generated (useful in PR mode) |

## Coverage

| Flag | Description |
| :--- | :---------- |
| `--coverage` | Run `go test -coverprofile` first; exclude uncovered lines from covered-MSI |
| `--per-test` | Build a per-test coverage map and run only the tests that cover each mutation. Best for packages with slow tests. Pairs well with `--coverage`. |
| `--test-flags` | Extra flags passed to every `go test` call (e.g. `--test-flags=-short`). Use the `=` form for values starting with a dash. Ignored when `--exec` is set. |

## Filtering

| Flag | Description |
| :--- | :---------- |
| `--blacklist <file>` | File of MD5 checksums to skip |
| `--disable <mutator>` | Disable a mutator by name (repeatable). Supports trailing-`*` wildcard (e.g. `'arithmetic/*'`). **Quote wildcard patterns** to prevent shell glob expansion. Config file equivalents: `disable_mutators` (denylist) and `enable_mutators` (allowlist) — see [config reference](config.md). |
| `--git-diff-lines` | Only mutate lines changed since `--git-diff-base` |
| `--git-diff-base` | Git ref to diff against (default: `HEAD`) |

## Baseline

| Flag | Description |
| :--- | :---------- |
| `--baseline <file>` | Known-survivors file; only fail on new escapes |
| `--update-baseline` | Record current survivors and exit 0 |

## Exit codes

| Code | Meaning |
| :--- | :------ |
| 0 | All mutations tested; all quality gates passed |
| 1 | Internal error |
| 4 | A quality gate was not met (`--min-msi`, `--min-covered-msi`, or `--fail-on-escaped`) |
