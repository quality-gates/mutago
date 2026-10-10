# Exploratory testing pass — mutago CLI, 2026-10-10

Scope: AFK pass. Three journeys not covered by earlier reports: adding a
custom mutator, mutating a multi-package module (`./...`, `--test-recursive`,
build tags), and CI reporting with a baseline (GitHub/GitLab loggers,
same-text mutants).

## Build and setup

- Built `mutago dev` from [`f47b7ae`](https://github.com/quality-gates/mutago/commit/f47b7ae9416fb96b7c1efbdc8ab34e1208996175) (v2.10.26). See [setup](evidence/00-setup.txt).
- Go 1.26.6 on macOS arm64.
- Disposable Go modules under `$TMPDIR`. Copies are under [`evidence/`](evidence/): [`fixture-custom`](evidence/fixture-custom/), [`fixture-neg`](evidence/fixture-neg/), [`fixture-multi`](evidence/fixture-multi/), [`fixture-ci`](evidence/fixture-ci/).
- Every mutation run used `GOMAXPROCS=1`, `--workers=1`, `--exec-timeout=10`, and a disposable `GOCACHE`.
- In the evidence, the binary path is shortened to `mutago` and the scratch path to `$TMPDIR`.

## Confirmed bugs

| Issue | Summary |
| :---- | :------ |
| [#300](https://github.com/quality-gates/mutago/issues/300) | The custom mutator guide says to fork `cmd/mutago/main.go`. That copy cannot build outside the mutago module, because it imports `internal/` packages |

### #300 — custom mutator wiring does not build

**Impact:** a user who follows [custom-mutators.md](../../custom-mutators.md) cannot build a binary with their mutator from their own module.

**Replay:** in an empty module, add the guide's `flip` package. Copy `cmd/mutago/main.go` from v2.10.26, add a blank import of `flip`, run `go get github.com/quality-gates/mutago/v2@v2.10.26`, then `go build ./cmd/mutago`.

**Expected:** a binary that also runs `mypkg/flip-sign`.

**Actual:** `use of internal package github.com/quality-gates/mutago/v2/internal/baseline not allowed`. Seen twice from empty directories: [first](evidence/01-external-fork-build.txt), [replay](evidence/03-external-fork-replay.txt).

**Workaround:** clone the whole mutago repo and add the import there. That works, and `mypkg/flip-sign` killed its mutant: [runs](evidence/02-repo-fork-runs.txt).

## Journeys

### 1. Add a custom mutator

**Goal:** register `mypkg/flip-sign` as the guide shows and see it in a run.

- External module: build fails (#300).
- Whole-repo fork: builds. `--debug` lists `mypkg/flip-sign` as enabled.
- The guide's example does the same edit as `arithmetic/negate` (`-x` → `+x`). Duplicate edits are dropped, so the dry-run shows no `mypkg/flip-sign` mutant. With `--disable arithmetic/negate`, it shows up and is killed: [runs](evidence/02-repo-fork-runs.txt).

### 2. Multi-package module

**Goal:** run `./...` on a module with a tested package (`a`), a package tested only from a subpackage (`b`, tested by `b/c`), and a package whose test needs `-tags=integration` (`tagged`). Expected: every package is mutated, and each mutant is judged by the tests that cover it.

- With default settings, `./...` mutates only `a`. Targeting `./b`, `./b/b.go`, or `./b` with `--test-recursive` exits 3 with "Could not find any suitable Go source files": [output](evidence/10-default-skips.txt). This is the documented default: `skip_without_test` and `skip_with_build_tags` are both `true`. Not a bug.
- With both turned off ([`all.yml`](evidence/fixture-multi/all.yml)):
  - `./b` alone: 5 escaped (no tests in `b`). With `--test-recursive`: 4 killed, 1 escaped (`>` → `>=`, which is equivalent here): [output](evidence/11-config-all.txt).
  - `./tagged`: 4 escaped. With `--test-flags=-tags=integration`: 4 killed (same file).
  - `--coverage` and `--per-test` give the same results for `b` (with `--test-recursive`) and `tagged` (with `-tags`). Without those flags, mutants are reported as not covered: [output](evidence/12-coverage-pertest.txt), [`./...` with `--test-recursive`](evidence/13-recursive-all.txt).

No bugs found.

### 3. CI reporting and baseline

**Goal:** a CI user gets escapes as GitHub annotations and a GitLab report, records a baseline, and fails only on new escapes. This includes identical lines, which were fixed in #274.

- [Run](evidence/20-ci-run.txt) with all loggers and `--coverage`: 17 mutants. There are 7 `::warning` lines, one per escape. [GitLab JSON](evidence/20-gitlab.json) has 7 unique fingerprints. The summary JSON totals add up.
- Monorepo variation: the module is in `svc/` and mutago runs from there. Annotation and GitLab paths are `svc/pkg/calc.go`, relative to the git root: [output](evidence/21-monorepo-paths.txt).
- [`--update-baseline`](evidence/22-update-baseline.txt) recorded 11 survivors. `Inc` and `IncAgain` have the same line text, and each gets its own ID: [baseline](evidence/22-mutago-baseline.json).
- Added `IncThird` with the same text and no assertion ([patch](evidence/23-new-code.patch)). The gate exits 4 with "4 new mutant(s) escaped": [output](evidence/23-baseline-new-same-text.txt).
- Variation: added the same function between `Inc` and `IncAgain` instead ([patch](evidence/24-insert-before.patch)). The gate still says 4 new and exits 4: [output](evidence/24-baseline-insert-before.txt).

## Unresolved

- Same-text IDs seem to follow order within the file. In the insert-before variation, the agentic JSON gives the old `IncAgain` IDs to the new function on line 8. The unchanged `IncAgain` (now line 12) gets new IDs. The count and exit code are correct, so this affects only which site is called "new". The tool does not say which sites are new, so this is not user-visible from the terminal. Not filed.

## Usability observations

- "Could not find any suitable Go source files" does not say that `skip_without_test` or `skip_with_build_tags` removed the files. A user who passes `--test-recursive` or `-tags` is likely to hit this, because the defaults skip exactly the files those flags exist for. *Suggestion:* name the skip setting in the message, or warn when an explicit file target is skipped.
- `skip_without_test` matches by file name (`x.go` needs `x_test.go`). With `./...`, a file tested from `pkg_test.go` is dropped silently, and the score does not show it.
- When the baseline gate fails, it gives a count of new escapes but does not list them. The terminal shows all escapes, old and new, the same way. *Suggestion:* mark or list the new ones.
- The custom mutator guide's example duplicates `arithmetic/negate`, so it never appears in a default run (noted in #300).

## Not explored

- HTML report contents, `--exec` scripts, and `--timeout-coefficient`.
- Windows and Linux.
