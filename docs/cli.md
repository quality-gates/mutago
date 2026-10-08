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
| `--timeout-coefficient` | `0` (disabled) | Scale per-mutation timeout as a multiple of an uncached baseline test-suite run (e.g. `3` = 3× the clean run). The clean pre-flight/coverage run uses a generous cap so a suite slower than `--exec-timeout` can still be measured. Overrides `--exec-timeout` when set. |
| `--workers` | all CPUs | Number of parallel mutation workers |
| `--config` | — | Path to YAML config file |

## Output flags

| Flag | Description |
| :--- | :---------- |
| `--dry-run` | Count mutations per file and mutator without generating files or running tests; prints a summary table and exits 0. The count uses the same scope as a real run (`--git-diff-lines`, `--blacklist`, duplicate edits, and `--run-mutant-id`) and does not apply coverage |
| `--noop` | No-op (backward compatibility). The baseline pre-flight check — run the suite once unmutated, exit with a tool error if it fails — is now always on by default. Skipped under `--coverage`, `--no-exec`, `--dry-run`, or a custom `--exec` |
| `--no-diffs` | Suppress diff output for all mutation results (useful in CI where diffs are noisy and the JSON report is consumed instead) |
| `--output-statuses` | Show only listed result statuses in the terminal: `k`=killed `e`=escaped `s`=skipped `n`=not-covered `x`=errored (e.g. `--output-statuses=ke`). Does not affect JSON reports. Overrides `--quiet` when set. |
| `--quiet` | Suppress killed/skipped lines; show only escaped mutants and summary (equivalent to `--output-statuses=e`) |
| `--verbose` | Print full test output for each mutation |
| `--debug` | Print internal debug information, including each mutation's 32-character blacklist checksum |
| `--logger-github` | Emit escaped mutants as `::warning` GitHub Actions annotations |
| `--logger-gitlab` | Write `mutago-gitlab.json` in GitLab Code Quality format |
| `--logger-summary-json` | Write compact stats to `mutago-summary.json` |
| `--logger-agentic-json` | Write LLM-ready report to `mutago-agentic.json` |
| `--run-mutant-id` | Run only the mutant with this stable ID (copy the `id` field from `mutago-agentic.json`). Other mutants are not classified or counted. `--dry-run` counts only this mutant. Exits 3 if no matching mutant is found. Valid-ID runs suppress the summary and quality gates. |
| `--version`, `-v` | Print version and exit 0 |

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
| `--coverage` | Run `go test -coverprofile` first; stop with exit 3 if it fails, otherwise skip test execution for uncovered lines and exclude them from covered-MSI |
| `--per-test` | Build a per-test coverage map and run only the tests that cover each mutation. Best for packages with slow tests. Pairs well with `--coverage`. With `--test-recursive`, the map also covers subpackage tests. |
| `--test-flags` | Extra flags passed to every `go test` call (e.g. `--test-flags=-short`). Use the `=` form for values starting with a dash. Adaptive timeout preserves a positive `-count=N`, adds `-count=1` when absent, and rejects `-count=0`. Ignored when `--exec` is set. |

## Vet

Every built-in `go test` run (baseline pre-flight, coverage, and mutant runs) passes `-vet=off` by default. A mutant is not meant to be lint-clean, and a `go vet` diagnostic on mutated code would fail the run and be miscounted as KILLED. The baseline and coverage runs use the same setting so they judge tests the same way the mutant runs do. Pass your own `-vet` via `--test-flags` to re-enable it (e.g. `--test-flags=-vet=all`); your flag wins in every run and no duplicate is added.

With `--test-recursive`, every run targets `<package>/...`, so the baseline and coverage runs cover the same tests as the mutant runs. The coverage run also passes `-coverpkg=<package>`, so tests in subpackages count towards the package's coverage.

Mutant test runs also pass `-failfast`. One failing test is enough to kill a mutant, so `go test` starts no new tests after the first failure. Pass `--test-flags=-failfast=false` to run the whole suite for every mutant; your flag wins and no duplicate is added.

`--per-test` builds its map the same way. Listing packages and tests gets only your build flags (`-tags`, `-race`, `-gcflags`, `-ldflags`, `-mod`, and so on), so tests behind a build tag are mapped. The coverage test binary is compiled with all of your flags, and each test runs with your runner flags (`-short`, `-count`, `-v`, ...) in `-test.*` form. Flags may be written `-name`, `--name`, or `-name=value`.

If a generated mutant does not compile, it is skipped rather than counted as killed by a test.

## Filtering

| Flag | Description |
| :--- | :---------- |
| `--blacklist <file>` | File of MD5 checksums to skip |
| `--disable <mutator>` | Disable a mutator by name (repeatable). Supports trailing-`*` wildcard (e.g. `'arithmetic/*'`). **Quote wildcard patterns** to prevent shell glob expansion. A name that matches no mutator prints a warning on stderr. Config file equivalents: `disable_mutators` (denylist) and `enable_mutators` (allowlist) — see [config reference](config.md). |
| `--git-diff-lines` | Only mutate lines changed since `--git-diff-base` (compared against the merge-base, so it matches the PR diff even when the branch is behind its target) |
| `--git-diff-base` | Git ref to diff against (default: the remote-tracking ref named by `origin/HEAD`, such as `origin/main`; falls back to `master` if `origin/HEAD` cannot be resolved) |

## Baseline

| Flag | Description |
| :--- | :---------- |
| `--baseline <file>` | Known-survivors file; only fail on new escapes |
| `--update-baseline` | Record current survivors and exit 0 |

## Exit codes

| Code | Meaning |
| :--- | :------ |
| 0 | All mutations tested; all quality gates passed |
| 3 | Invalid input or tool failure, including a failed clean coverage run |
| 4 | A quality gate was not met (`--min-msi`, `--min-covered-msi`, or `--fail-on-escaped`) |

## Exploratory testing

The dated [CLI exploratory testing reports](exploratory-testing/README.md) record end-to-end journeys, results, and supporting evidence.
