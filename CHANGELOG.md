# Changelog

All notable changes to this project will be documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/). This project uses [Semantic Versioning](https://semver.org/).

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
