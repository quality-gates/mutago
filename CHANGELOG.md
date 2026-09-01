# Changelog

All notable changes to this project will be documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/). This project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Fixed
- Adaptive timeout baselines now bypass Go's test-result cache, preventing cached coverage runs from producing false killed mutants. Failed coverage runs now stop with a tool error instead of using partial profiles.

## [v2.9.2] — 2026-08-31

### Fixed
- `// mutator-disable-regexp` directives now match the final line of a source file even when the file has no trailing newline. Previously `bufio.Reader.ReadString` returned that line together with `io.EOF` and the loop discarded it, so regex annotations on the last line were silently ignored.
- The `std` target pattern now resolves standard-library packages again. The importer walked `$GOROOT/src/pkg`, a directory removed in Go 1.4 (the standard library has lived under `$GOROOT/src` since then), so `std` silently returned only `cmd/*` packages. The walk now targets `$GOROOT/src`.
- Coverage resolution (`Profile.IsCovered`, `PerTestProfile.CoveringTests`) is now deterministic. It previously matched an absolute file path against module-relative keys with a path-suffix test while iterating a Go map, so when one file path was a suffix of another (e.g. `foo/bar.go` and `bar.go`) the matching key was chosen at random and the wrong result was cached. The longest — most specific — matching key now wins, and that correct result is cached.
- `gitdiff.IsLineChanged` (the `--git-diff-lines` absolute-path fallback) is now deterministic. It had the same path-suffix map-iteration collision as the coverage resolver: when one relative path was a suffix of another, the matching diff entry was chosen at random, so changed lines could be reported unchanged (mutations wrongly skipped) and vice-versa. The longest — most specific — matching key now wins. The primary `--git-diff-lines` path already used the deterministic `IsRelativeLineChanged` direct lookup.

## [v2.9.1] — 2026-08-27

### Fixed
- Package preparation now batches target directories into one typed package load instead of launching one `go list` process per source file, restoring fast mutation discovery for multi-file packages.

## [v2.9.0] — 2026-08-27

### Changed
- Mutation discovery now indexes annotation, coverage, git-diff, package, and identifier lookups; defers source rendering until a mutant is executed; and reuses scratch storage in high-cardinality mutators.
- Per-test coverage now compiles one covered test binary per package and reuses it for every individual test instead of invoking the Go toolchain for each test.
- Full JSON reports store source text once per file in `sources`; legacy per-mutant source fields are omitted from newly generated reports.
- Agentic report context, CLI target resolution, AST diagnostic printing, and select/error-wrap/composite mutation generation now avoid repeated work and unnecessary allocations.

### Fixed
- Custom `--exec` commands now honor `--exec-timeout` and cancellation.
- Per-test discovery excludes benchmarks, which cannot be selected reliably with the test `-run` filter.

## [v2.8.6] — 2026-08-27

### Changed
- Package parsing no longer loads full syntax and type information for dependencies. Full-tree dry runs complete substantially faster while retaining target-package type information.

## [v2.8.5] — 2026-08-27

### Changed
- `--coverage` with `--timeout-coefficient` now derives its adaptive timeout from the coverage run, avoiding a duplicate clean test run for every target package.

## [v2.8.4] — 2026-08-27

### Fixed
- `numbers/decrementer` now parenthesizes negative integer and float mutations, preventing invalid expressions such as `value--0.5`.

## [v2.8.3] — 2026-08-26

### Fixed
- `--coverage` now skips test execution for mutants on uncovered lines. Previously every mutant ran `go test` (or `--exec`) before the coverage check, so uncovered mutants paid the full suite cost and then were discarded.

## [v2.8.2] — 2026-08-20

### Fixed
- Agentic JSON report generation no longer panics when an escaped mutant's start line falls outside its captured source text (`extractContextLines` slice bounds).

## [v2.8.1] — 2026-08-05

### Fixed
- `--test-flags` now reaches the initial `--coverage` profile collection step (`runCoverageProfile`). Previously `-short` and similar flags were forwarded to per-test profiling and per-mutant runs but omitted from coverage collection, so failing non-short tests could leave an empty profile and mark all mutants NOT COVERED.

### Changed
- Shipping step 10 now requires a push-access check on `jonbaldie/go-mutesting` before attempting the downstream sync; skip the sync and note it when push is unavailable.

---

## [v2.8.0] — 2026-08-03

### Changed
- Upgraded module to `go 1.26.5`, clearing GO-2026-5856 / CVE-2026-42505 (`crypto/tls`), GO-2026-5039 / CVE-2026-42507 (`net/textproto`), GO-2026-5037 / CVE-2026-27145 (`crypto/x509`), GO-2026-5038 / CVE-2026-42504 (`mime`), and GO-2026-4970 / CVE-2026-39822 (`os`).

---

## [v2.7.7] — 2026-07-13

### Changed
- Mutation CI now tests only lines changed by a pull request, while retaining the full-tree gate for code-related pushes to `main`. Pull requests with no mutable changes pass without running mutants.

---

## [v2.7.6] — 2026-07-13

### Added
- Added committed Serena project configuration and onboarding memories for symbol-aware navigation, project conventions, common commands, and completion checks. Expanded the agent instructions with skill-use rules and the required downstream `jonbaldie/go-mutesting` sync step.
- Vendored 21 engineering/productivity agent skills from [mattpocock/skills](https://github.com/mattpocock/skills) (MIT license) under `.claude/skills/`, with attribution in `.claude/skills/THIRD-PARTY-NOTICES.md`. Configured the `triage` skill's issue-tracker and label vocabulary for this repo in `docs/agents/`, documented in `CLAUDE.md` under a new "Agent skills" section.
- Committed git hook scripts under `githooks/` (`pre-commit`, `pre-push`) that mirror the checks in `.github/workflows/`: `pre-commit` runs the fast whole-tree checks (gofmt, go vet, gocyclo, ineffassign, messgo, build, unit tests), hard-failing on any finding or missing tool, while `pre-push` runs mutation testing scoped to the diff against `origin/main` via `--git-diff-lines`. Not activated on clone — opt in with `git config core.hooksPath githooks`. Documented under a new "Definition of Ready" section in `CLAUDE.md`.

### Changed
- Reduced the `mutationRun` struct's CouplingBetweenObjects (CBO) metric from 13 to 12 types by threading the `gitdiff.ChangedLines` parameter through the call chain instead of storing it as a field. This passes the messgo CBO quality gate (< 13) without changing external behavior or API surface.
- Extracted the git/write/line-matching helpers in `TestParseChangedLines_StaleBranchExcludesTargetChanges` into top-level test functions, dropping its cyclomatic complexity from 17 to under the goreportcard gocyclo threshold of 15.

### Fixed
- Mutation locations used by reports, coverage, per-test selection, and `--git-diff-lines` now come from the original AST token position instead of the first line changed by `go/printer`. This keeps PR filtering and reported lines accurate for leading comments, multiline syntax, and insertion-style mutators. `statement/remove-self-assign` also anchors its empty replacement at the removed statement so comments remain in place.
- `FindOriginalStartLine` now reports the line after a zero-length original range for pure additions, including line 1 for additions to an empty file. It also validates hunk ranges and body prefixes, returning the fallback line instead of accepting malformed diffs or overflowing `int64` line coordinates.
- Noop-based statement and branch mutators no longer emit invalid Go when removed code declares local variables, uses struct field keys, or contains selectors rooted in calls. Generated references are position-normalized so interior comments stay outside noop assignments, and empty branch bodies are skipped instead of producing identical mutants.
- `TestParseChangedLines_StaleBranchExcludesTargetChanges` now forces its scratch repo's initial branch name (`git init -b main`) instead of relying on the user's `init.defaultBranch` config, which defaults to `master` in many environments.
- Fixed `gofmt -s` findings (redundant blank lines, misaligned struct tags) in `cmd/mutago/main_test.go`, `internal/coverage/coverage_test.go`, `internal/engine/engine.go`, and `internal/models/report.go`.

---

## [v2.7.5] — 2026-06-16

### Fixed
- `--git-diff-lines` now diffs against the **merge-base** of the diff base and the current branch instead of the tip of the base branch. Previously, when a feature branch was behind its target, a plain two-dot `git diff <base>` attributed commits that had landed on the base after the branch point to the feature branch, so mutago mutated lines the developer never touched. The filter now matches exactly what the pull request shows, while still including uncommitted working-tree changes.

---

## [v2.7.4] — 2026-06-10

### Changed
- Bumped the `messgo` CI quality gate from `v0.1.1` to `v0.1.9`. The newer release was verified locally with the workflow's `go,codesize` rules and the LCOM rule; both are clean for production Go code.
---

## [v2.7.3] — 2026-06-09

### Fixed
- `statement/remove` (and the other noop-based mutators) no longer mangle a comment that sits directly above the mutated code. The synthesized noop (`_, _ = a, b`) carried no source position, so `go/printer` floated the leading comment into the middle of the assignment (e.g. `_, _ =\n// comment\na, b`) and the resulting diff's first hunk line pointed at the comment instead of the code. The noop is now anchored at the original line of code, so the comment stays above it and the diff reports the correct line. For branch mutators the anchor descends into the replaced block so the noop still renders on the code line rather than at the opening brace.
- `FindOriginalStartLine` (`internal/parser`) now walks the first hunk body to find the first changed line instead of assuming a fixed three lines of context (`header + 3`). The old heuristic reported the wrong line whenever a hunk carried fewer than three context lines — for example a change near the top of a function, or one preceded by a comment.

### Changed
- Renamed the diff fuzz target `FuzzParseDiffOutput` to `FuzzFindOriginalStartLine` and removed the now-unused `ParseDiffOutput` helper, which the new line-walking logic replaces.

---

## [v2.7.2] — 2026-06-05

### Added
- Coverage-guided fuzz tests (`FuzzParse`, `FuzzParseDiffOutput`, `FuzzParseProfile`, `FuzzMutantID`, `FuzzLoad`) with seed corpora for the input-parsing surface in `internal/gitdiff`, `internal/parser`, `internal/coverage`, and `internal/baseline`, plus a saved regression seed for the overflow above.
- [messgo](https://github.com/quality-gates/messgo) (a PHPMD-style mess detector for Go) as a CI quality gate. A new `messgo` workflow runs the recommended `go,codesize` rulesets over the source with `--ignore-tests` (test fixtures excluded) and fails the build on any violation.

### Changed
- Refactored the codebase to pass the new messgo gate without lowering any threshold. High-complexity functions were split into focused helpers, `else` branches were flattened into early returns, the long `mutate` / `processMutationFile` / `mutateAllPackages` parameter lists were grouped into `mutationRun` and `fileContext` types, and the not-covered branch of `recordMutantResult` was lifted into its own helper to drop a boolean flag argument. Behaviour is unchanged; all tests and the self-mutation quality gates (MSI ≥ 75%, covered-code MSI ≥ 80%) still pass.

### Fixed
- The `--git-diff-lines` hunk-header parser (`internal/gitdiff`) no longer ignores `strconv.Atoi` errors when reading a hunk's start line and count. An out-of-range numeric field (more digits than fit in an `int64`) previously made `Atoi` return `MaxInt64` while the error was discarded; the subsequent `Start + count - 1` then overflowed to a negative number, producing an inverted `LineRange` that silently corrupted the diff-line filter. Malformed or out-of-range hunk headers are now skipped cleanly. Found by coverage-guided fuzzing.

---

## [v2.7.1] — 2026-06-02

### Changed
- Per-mutant console output now uses mutation-testing terminology instead of test-runner terminology. A caught mutant is labelled `KILLED` (was `PASS`) and a surviving one is labelled `ESCAPED` (was `FAIL`). The old `PASS`/`FAIL` wording was confusing: `PASS` looked like a good result for the line it sat on, when it actually meant the mutation had been killed, and `FAIL` meant the mutation escaped. The labels now match the wording already used in the summary line and the `KILLED`/`ESCAPED` colour coding (green/red) is unchanged. The same relabelling is applied to the `--exec` script comments and the documented example output.

---

## [v2.7.0] — 2026-05-31

### Added
- Three Go-idiomatic mutators aimed at the kinds of subtly-untested code that machine-generated Go tends to produce:
  - `composite/field-clear` drops one keyed field from a struct, map, or keyed array/slice literal, leaving it at its zero value. It targets fields set to a meaningful value that no test asserts — e.g. a fully-populated config or options struct where only a couple of fields matter to the suite. Fields already at a zero value (`0`, `""`, `false`, `nil`) and positional elements are skipped to avoid no-op mutations.
  - `expression/errorf-wrap` downgrades the error-wrapping verb in `Errorf`-style calls from `%w` to `%v`. The message is byte-for-byte identical, but the returned error no longer wraps its cause, so `errors.Is` / `errors.As` stop matching. It finds error wrapping that no test ever unwraps.
  - `expression/recover-clear` neutralises a `recover()` call by rewriting it to `any(nil)`, so the recovered value is always nil and a panic propagates instead of being recovered. It finds deferred recovery blocks that no test exercises.
- The self-mutation quality gate (`mutation.yml`) now also covers the new `mutator/composite` package.

---

## [v2.6.16] — 2026-05-29

### Fixed
- The `cmd/mutago` config integration tests no longer fail on a fresh checkout. Their fixture files (`testdata/configs/*.yml.test`) matched the `*.test` `.gitignore` rule (intended for compiled Go test binaries) and were silently dropped from the initial commit, so a clean clone had a red test suite. The fixtures are now committed and a `.gitignore` negation keeps them tracked.

---

## [v2.6.15] — 2026-05-26

### Fixed
- Worker goroutines no longer crash with a raw Go stack trace when `--exec` is given a bad path or encounters an I/O error; the error is now printed cleanly to stderr and the mutant is recorded as errored.
- Diff output is now printed under the report mutex, eliminating interleaved diff/PASS/FAIL lines when running with multiple workers.
- `--dry-run` output now notes that the count is an upper bound (identical mutations across files are deduplicated in a real run); the `--dry-run` help text says the same.
- README blacklist section corrected: checksums are only printed with `--debug`, not during a normal run.
- README `--exec` example path no longer contains a spurious `/v2/` segment that caused a not-found panic.
- Dead struct fields `TimeOutCount`, `MutationCodeCoverage`, and `Timeouted` (which were never populated) removed from `Stats` and `Report`; the README no longer documents them as meaningful.
- `--logger-agentic-json` now writes `msi` in the same 0–1 ratio scale as `--logger-summary-json`, eliminating the prior inconsistency where agentic JSON used 0–100.
- Skipped mutants (mutations that did not compile) are now tracked in `Report.Skipped` and included in the per-mutator breakdown table, making the breakdown consistent with the headline MSI.

---

## [v2.6.14] — 2026-05-26

### Fixed
- `arithmetic/bitwise` mutator no longer produces `INTERNAL ERROR … expected ')', found '|'` when run against packages that contain generics type union constraints (e.g. `type T interface { *A | *B | *C }`). The `|` in such constraints is a type-level operator and is now correctly skipped.

---

## [v2.6.13] — 2026-05-21

### Added
- `.github/workflows/goreportcard.yml`: CI gate that enforces A+ grade (>90%) using goreportcard's exact weights from source — gofmt (0.30), go vet (0.30), gocyclo (0.10), license (0.05), ineffassign (0.10).
- Two `BuildPerTestProfile` tests targeting previously escaped mutants; covered-code MSI on `internal/coverage` rises from 81.76% to 89.19%.

### Fixed
- Typo: `duting` → `during` in `internal/annotation/regex.go`.

### Changed
- All functions brought to cyclomatic complexity ≤15: helpers extracted in `cmd/mutago/main.go`, `internal/coverage`, `internal/importing`, and `mutator/statement`.
- `gofmt -s` applied to all source and testdata files.

[v2.6.13]: https://github.com/quality-gates/mutago/compare/v2.6.12...v2.6.13

---

## [v2.6.12] — 2026-05-20

### Fixed
- `concurrency/goroutine-remove`, `select/case-remove`, and `select/default-remove` were registered with underscores but documented with hyphens. `--disable concurrency/goroutine-remove` (as the docs instructed) silently did nothing. All three names now use hyphens consistently. **Breaking for anyone who referenced the old underscore names directly in `--disable` flags or `disable_mutators` config.**
- `--run-mutant-id` no longer prints a misleading "mutation score is 0.00%" summary line when run in single-mutant debug mode.

### Changed
- CI example workflows in README and `docs/ci.md` updated from `actions/checkout@v4` / `actions/setup-go@v5` to `@v6`.
- `docs/install.md` Go requirement updated from 1.25 to 1.26.
- `docs/json-outputs.md`: `context_start_line` and `reminder` fields added to the agentic JSON schema table; MSI scale difference (0–1 in summary JSON vs 0–100 in agentic JSON) called out explicitly.
- `docs/cli.md` and `docs/config.md`: added shell-quoting note for wildcard patterns (`--disable 'arithmetic/*'`).

---

## [v2.6.11] — 2026-05-20

### Added
- `.github/dependabot.yml`: weekly grouped Dependabot PRs for Go modules and GitHub Actions.
- `.github/workflows/security.yml`: `govulncheck` runs on push/PR and weekly Monday cron via `golang/govulncheck-action`; now fully enforcing (no `|| true`).

### Changed
- Upgraded module to `go 1.26.3`, clearing GO-2026-4980 and GO-2026-4982 (XSS in `html/template`).
- Moved vulnerability scanning out of `mutation.yml` into dedicated `security.yml`.
- Updated all GitHub Action pins from v4/v5 to current v6 SHAs.

---

## [v2.6.10] — 2026-05-20

### Performance
- `packages.Load` is now cached per directory in `ParseAndTypeCheckFile`. For a package with N files, the type-checker runs once instead of N times — reducing dry-run time by 66% (4.8 s → 1.6 s for a 4-package run) and overall user CPU by ~31%.
- `format.Source` on the original file is now computed once per source file in `mutate()` and passed pre-formatted into `saveAST`, eliminating a redundant `gofmt` call for every mutation. The improvement scales with worker count: ~7.6% wall-time reduction with default workers, ~4.3% with `--workers 1`.

### Fixed
- Integration tests now call `parser.ClearPackageCache()` between runs to prevent a stale cached AST from leaking into subsequent tests when the `--exec` script writes to the original file on disk.

---

## [v2.6.9] — 2026-05-20

### Added
- `conditional/bool-literal`, `conditional/not`, `expression/string-literal`, and `statement/defer-remove` mutators documented in README and docs site (were present in code since v2.6.7 but missing from docs).
- `arithmetic/assign_invert` table completed with six bitwise compound assignments (`&=`, `|=`, `^=`, `<<=`, `>>=`, `&^=`).
- `ignore_source_lines` config key documented in README and docs/config.md.
- `--logger-gitlab`, `--noop`, `--fail-on-escaped`, `--run-mutant-id`, `--timeout-coefficient` added to README features table and docs/cli.md.
- Full documentation link (`https://quality-gates.github.io/mutago/`) added to README.
- Example YAML config file added to README.

### Fixed
- Removed dead content: linked-list `removeNode` example (linked to inactive `avito-tech/container` repo) and Tavor framework reference.
- Corrected avito-tech description: inactive since late 2025, not an active alternative.
- Typos: `origianl`, `generate`/`generates`, `possible`/`possibly`, `Negotiation`/`Negation` (×4).
- Non-native phrasing inherited from upstream forks; broken `#this-fork` and `#expression-mutators` anchors.
- ToC entry "Other mutation testing projects and their flaws" now matches the actual heading.

[v2.6.9]: https://github.com/quality-gates/mutago/releases/tag/v2.6.9

---

## [v2.6.8] — 2026-05-19

### Added
- Registration assertions added to all mutator test files; every mutator's `init()` registration is now directly tested.
- `loop/condition` testdata extended with a `!=` operator loop, killing an escaped mutation where the operator assignment was a no-op.
- `statement/return` tests extended: pointer/slice, imported struct, already-zero, and `unsafe.Pointer` return types now covered by golden-file tests.
- `internal/coverage` tests strengthened: zero-line and negative-line guard, workers-normalisation (`workers=0→1`), and data-presence assertions for `BuildPerTestProfile`. All target files now above 80% covered MSI.
- CLAUDE.md convention: edit files one at a time using Read then Edit; no bulk scripts.

[v2.6.8]: https://github.com/quality-gates/mutago/releases/tag/v2.6.8

---

## [v2.6.7] — 2026-05-19

### Added
- `conditional/bool-literal` mutator: swaps `true`↔`false` in assignments and function call arguments.
- `conditional/not` mutator: removes the `!` operator from negated conditions in `if`, `for`, and `&&`/`||` operands.
- `expression/string-literal` mutator: replaces non-empty string literals in `==`/`!=` comparisons with `""`.
- `statement/defer-remove` mutator: turns `defer f()` into an immediate call, testing whether deferred execution timing matters. Covers `defer` inside `select` case branches.
- `arithmetic/assign_invert` expanded to cover bitwise compound assignments (`&=`, `|=`, `^=`, `<<=`, `>>=`, `&^=`).
- `--logger-gitlab` flag: emits a GitLab Code Quality artifact JSON (`mutago-gitlab.json`).
- `--timeout-coefficient` flag: scales per-mutation timeout by a multiplier of the baseline test-suite run time.
- `--run-mutant-id` flag: runs only the mutant with a given stable ID (copy the `id` field from `mutago-agentic.json`).
- `ignore_source_lines` config key: list of regexes; mutations on matching source lines are suppressed.
- `SourceLineRegexFilter` in `internal/filter`: programmatic filter for skipping mutations on regex-matched lines.
- New tests for `internal/coverage` (`CountTests`, `BuildPerTestProfile`) and `internal/filter` (`SourceLineRegexFilter`, `ShouldSkip`), raising quality-gate MSI to 77.75% / 83.78% covered.

### Fixed
- `statement/return` mutator now zeroes struct-typed return values using an empty composite literal (`T{}`).
- Guard conditions in `expression/context-nil`, `expression/string-literal`, and `statement/remove` refactored from `==` string comparisons to `switch`, eliminating equivalent mutations.
- GitLab report fingerprint now uses the stable `baseline.MutantID` hash, preventing deduplication of distinct mutations on the same line.

---

## [v2.6.6] — 2026-05-19

### Added
- `--dry-run` now prints per-mutator counts as a summary table before the grand total.
- `--per-test` startup message: prints the package name and test count before building the per-test coverage map.
- Agentic JSON `context_start_line` field: anchors `context_lines[0]` to a 1-based source line so LLMs can navigate without guessing.

### Fixed
- `--git-diff-base` now auto-detects the default branch via `git symbolic-ref origin/HEAD`; falls back to `master`.
- Agentic JSON `description` for simple one-line mutations now shows the exact change (e.g. `` `return a, b` → `return a, nil` ``).
- `--quiet` help text now mentions `--no-diffs`.

[v2.6.6]: https://github.com/quality-gates/mutago/releases/tag/v2.6.6

---

## [v2.6.5] — 2026-05-19

### Fixed
- `silent_mode: true` now prints only the final summary line; previously it suppressed the summary as well.
- `--logger-agentic-json` descriptions and kill hints were missing for 14 mutators; all 27 current mutators are now covered.
- `statement/remove` no longer generates false escapes on blank-assign statements (`_, _ = a, b`).
- `.gitignore` extended to cover `mutago-report.html`, `mutago-summary.json`, and `mutago-agentic.json`.

[v2.6.5]: https://github.com/quality-gates/mutago/releases/tag/v2.6.5

---

## [v2.6.4] — 2026-05-19

### Added
- `disable_mutators` and `enable_mutators` config keys: commit per-mutator control to YAML instead of threading flags through every CLI call. Trailing-`*` wildcards work for whole categories (`arithmetic/*`).
- Config JSON Schema updated for editor autocomplete.

### Fixed
- Panic when `*` was passed as a bare wildcard pattern to `--disable`.

[v2.6.4]: https://github.com/quality-gates/mutago/releases/tag/v2.6.4

---

## [v2.6.3] — 2026-05-19

### Added
- `--dry-run` flag: count mutations without writing files or running tests.
- `--no-diffs` flag: suppress diff output for all results (good for CI pipelines that consume the JSON report).
- `--output-statuses` flag: filter terminal output to specific result types (e.g. `e` for escaped, `ke` for killed + escaped).
- Config JSON Schema at `schema/config-schema.json`; add a comment to your config file for editor validation and autocomplete.

### Fixed
- Diffs for escaped and errored mutants now respect `--output-statuses`; previously they leaked through when other statuses were suppressed.

[v2.6.3]: https://github.com/quality-gates/mutago/releases/tag/v2.6.3

---

## [v2.6.2] — 2026-05-18

### Added
- `--per-test` flag: builds a per-test coverage map and runs only the tests that cover each mutation.
- `--test-flags` flag: passes extra flags to every `go test` call (e.g. `--test-flags=-short`).

### Fixed
- `--per-test` worker used `return` instead of `continue` on parse errors, causing a deadlock with one worker when any test binary failed to compile.
- `--test-flags` values were not forwarded to the per-test profile-building phase, causing inconsistent behaviour.
- `--per-test` help text incorrectly stated it requires `--coverage`.

[v2.6.2]: https://github.com/quality-gates/mutago/releases/tag/v2.6.2

---

## [v2.6.1] — 2026-05-18

### Added
- `numbers/float-negate` mutator: replaces non-zero float literals with `0.0`.
- `arithmetic/negate` mutator: inverts unary negation (`-x → +x`), closing the gap with gremlins' INVERT_NEGATIVES.
- `statement/remove-self-assign` mutator: removes self-assignment statements (`a = a`), closing the gap with gremlins' REMOVE_SELF_ASSIGNMENTS.
- `expression/context-nil` mutator: replaces `context.Context` arguments at call sites with `nil`.
- `expression/error-guard` mutator: replaces `if err != nil` with `if false` and `if err == nil` with `if true`.
- Public mutator extension API: `mutator.Register` / `mutator.New` so third-party packages can add custom operators without forking.
- MkDocs documentation site (Install, Quick Start, CLI reference, per-mutator pages, CI integration guide, JSON output schemas). Deployed to GitHub Pages.
- This CHANGELOG.

[v2.6.1]: https://github.com/quality-gates/mutago/releases/tag/v2.6.1

---

## [v2.6.0] — 2026-05-17

### Added
- `statement/return` mutator: replaces return values with the zero value for their type (`false`, `0`, `""`, `nil`). Uses `go/types` for type resolution. Kills 91% of its own mutants on first run.
- Copyright 2026 Jonathan Baldie to LICENSE.

### Fixed
- Progress goroutine is now joined (via `sync.WaitGroup`) before the summary line prints, eliminating a race on terminals.
- Config file parsing now rejects unknown YAML keys (`KnownFields(true)`) instead of silently ignoring them.
- `internal/parser`: migrated `ParseAndTypeCheckFile` from the deprecated `golang.org/x/tools/go/loader` to `golang.org/x/tools/go/packages`. Build-constrained files (e.g. testdata fixtures) fall back to direct parsing via `go/types.Config`.
- Vulnerability scanning via `govulncheck` added to CI (advisory-only until go1.26.3 releases).

[v2.6.0]: https://github.com/quality-gates/mutago/releases/tag/v2.6.0

---

## [v2.5.2] — 2026-05-17

### Fixed
- Updated module import paths to use `/v2` suffix consistently.
- `govulncheck` step made advisory-only in CI (`|| true`) to avoid blocking on unfixable CVEs in go1.25.x.
- Five bughunting fixes: exec path splitting (`strings.Fields`), covered-MSI no-coverage sentinel, JSON report conditional, `--quiet` suppressing NOT COVERED lines, progress goroutine race.

[v2.5.2]: https://github.com/quality-gates/mutago/releases/tag/v2.5.2

---

## [v2.5.1] — 2026-05-17

### Fixed
- CI workflow package paths updated to use `/v2` module suffix.

[v2.5.1]: https://github.com/quality-gates/mutago/releases/tag/v2.5.1

---

## [v2.5.0] — 2026-05-17

### Added
- Live progress display on terminals (shows running kill/escape/skip counts at 200 ms intervals; suppressed in verbose/debug/silent mode).
- `--baseline` / `--update-baseline` flags: track known-surviving mutants in a file; CI only fails on *new* regressions.
- `--logger-agentic-json`: writes `mutago-agentic.json` with enriched survived-mutant data designed for LLM consumption.

[v2.5.0]: https://github.com/quality-gates/mutago/releases/tag/v2.5.0

---

## [v2.4.3] — 2026-05-17

### Added
- Parallel mutation execution via `go test -overlay` (all CPUs by default, configurable with `--workers`).
- `--noop` pre-flight check: runs the test suite once without mutations; exits with an error if it already fails.
- `--logger-summary-json`: writes compact stats JSON to `mutago-summary.json`.
- `select/case-remove` and `select/default-remove` mutators (Go-specific channel select mutations).
- `concurrency/goroutine-remove` mutator (removes the `go` keyword from goroutine calls).

### Fixed
- `saveAST` errors now tracked in stats instead of silently dropped.
- Git diff line calculation uses the first hunk line for multi-hunk diffs.

[v2.4.3]: https://github.com/quality-gates/mutago/releases/tag/v2.4.3

---

## [v2.4.2] — 2026-05-17

### Fixed
- Internal test and documentation fixes; no behaviour change.

[v2.4.2]: https://github.com/quality-gates/mutago/releases/tag/v2.4.2

---

## [v2.4.1] — 2026-05-17

### Added
- `conditional/negated` mutator.
- `--quiet` flag: suppresses killed/skipped output, shows only escaped mutants and the final summary.
- `--fail-on-escaped`: exits with code 4 if any mutant survives, without requiring `--min-msi`.
- Vulnerability scan via `govulncheck` wired into CI.

[v2.4.1]: https://github.com/quality-gates/mutago/releases/tag/v2.4.1

---

## [v2.4.0] — 2026-05-17

### Changed
- Module path renamed from `github.com/avito-tech/mutago` to `github.com/quality-gates/mutago/v2`.

[v2.4.0]: https://github.com/quality-gates/mutago/releases/tag/v2.4.0

---

[v2.6.7]: https://github.com/quality-gates/mutago/releases/tag/v2.6.7
[v2.6.10]: https://github.com/quality-gates/mutago/compare/v2.6.9...v2.6.10
[v2.6.11]: https://github.com/quality-gates/mutago/compare/v2.6.10...v2.6.11
[v2.6.12]: https://github.com/quality-gates/mutago/compare/v2.6.11...v2.6.12
[v2.6.13]: https://github.com/quality-gates/mutago/compare/v2.6.12...v2.6.13
[v2.6.14]: https://github.com/quality-gates/mutago/compare/v2.6.13...v2.6.14
[v2.6.15]: https://github.com/quality-gates/mutago/compare/v2.6.14...v2.6.15
[v2.6.16]: https://github.com/quality-gates/mutago/compare/v2.6.15...v2.6.16
[v2.7.0]: https://github.com/quality-gates/mutago/compare/v2.6.16...v2.7.0
[v2.7.1]: https://github.com/quality-gates/mutago/compare/v2.7.0...v2.7.1
[v2.7.2]: https://github.com/quality-gates/mutago/compare/v2.7.1...v2.7.2
[v2.7.3]: https://github.com/quality-gates/mutago/compare/v2.7.2...v2.7.3
[v2.7.4]: https://github.com/quality-gates/mutago/compare/v2.7.3...v2.7.4
[v2.7.5]: https://github.com/quality-gates/mutago/compare/v2.7.4...v2.7.5
[v2.7.6]: https://github.com/quality-gates/mutago/compare/v2.7.5...v2.7.6
[v2.7.7]: https://github.com/quality-gates/mutago/compare/v2.7.6...v2.7.7
[v2.8.0]: https://github.com/quality-gates/mutago/compare/v2.7.7...v2.8.0
[v2.8.1]: https://github.com/quality-gates/mutago/compare/v2.8.0...v2.8.1
[v2.8.2]: https://github.com/quality-gates/mutago/compare/v2.8.1...v2.8.2
[v2.8.3]: https://github.com/quality-gates/mutago/compare/v2.8.2...v2.8.3
[v2.8.4]: https://github.com/quality-gates/mutago/compare/v2.8.3...v2.8.4
[v2.8.5]: https://github.com/quality-gates/mutago/compare/v2.8.4...v2.8.5
[v2.8.6]: https://github.com/quality-gates/mutago/compare/v2.8.5...v2.8.6
[v2.9.0]: https://github.com/quality-gates/mutago/compare/v2.8.6...v2.9.0
[v2.9.1]: https://github.com/quality-gates/mutago/compare/v2.9.0...v2.9.1
[v2.9.2]: https://github.com/quality-gates/mutago/compare/v2.9.1...v2.9.2
[Unreleased]: https://github.com/quality-gates/mutago/compare/v2.9.2...HEAD
