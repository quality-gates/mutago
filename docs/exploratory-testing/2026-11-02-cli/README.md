# Exploratory testing report: CLI, 2026-11-02

## Setup

Built commit [`3f240bf`](https://github.com/quality-gates/mutago/commit/3f240bf14f0d3c7608b55a64c625276d518ed88a) on Linux amd64 with Go 1.26.8. The [setup record](evidence/setup.txt) records the build. The isolated [module](evidence/go.mod), [source](evidence/calc.go), and [test](evidence/calc_test.go) were in the worktree. The clean test passed. Every mutation run used one worker, `GOMAXPROCS=1`, and a 20-second timeout. Go caches, temporary files, and dependencies were confined to the worktree. The binary was built after `go mod download` with `GOFLAGS=-mod=readonly`.

## Findings

[Issue #356](https://github.com/quality-gates/mutago/issues/356) records one confirmed bug in the bundled custom runner. The first runner anomaly below was a test-driver mistake. Existing issues [#340](https://github.com/quality-gates/mutago/issues/340), [#351](https://github.com/quality-gates/mutago/issues/351), [#352](https://github.com/quality-gates/mutago/issues/352), and [#353](https://github.com/quality-gates/mutago/issues/353) cover known baseline, changed-line, and workspace hazards; this pass did not re-test those scenarios.

## Journeys

### Classify mutants and enforce a gate

The goal was to identify tested and untested code, then fail a CI gate for escaped mutants. Without coverage, [the run](evidence/normal.txt) reported two killed and four escaped mutants, and the [summary JSON](evidence/normal-summary.json) matched the terminal totals. With coverage, [the variation](evidence/coverage.txt) classified the four mutants in the uncalled function as not covered. Its [summary JSON](evidence/coverage-summary.json) matched the terminal totals. `--fail-on-escaped` [exited 4 without coverage](evidence/gate-escapes.txt) and [exited 0 with coverage](evidence/gate.txt), where no mutants escaped. The latter does not mean all source lines were tested.

### Replay one reported survivor

The goal was to inspect one survivor without rerunning the full gate. The [agentic report](evidence/normal-agentic.json) provided the ID and diff for an arithmetic mutation. [Replaying that ID](evidence/single.txt) produced one escape and exit 0, despite `--fail-on-escaped`, as documented for single-mutant runs. A [nonexistent ID](evidence/unknown.txt) exited 3. The source hashes [before](evidence/source-before.sha256) and [after](evidence/source-final.sha256) matched, so no mutation remained in the fixture.

### Run a custom test command

The goal was to use the supplied custom runner and observe unsupported-option warnings. A [wrapper](evidence/runner.sh) recorded `MUTATE_TIMEOUT` and delegated to `scripts/exec/test-mutated-package.sh` from the tested commit. The [ordinary run](evidence/custom.txt) had the same killed and escaped results as the built-in runner. With `--per-test --timeout-coefficient=3`, [the variation](evidence/custom-options.txt) warned that both options were ignored and produced the same results. The [recorded calls](evidence/custom-options-calls.txt) show the explicit timeout was passed to the script. Source hashes matched after the runs.

**Confirmed bug, [#356](https://github.com/quality-gates/mutago/issues/356).** The CLI promises that `--no-diffs` suppresses mutation diffs, but the bundled custom runner printed a diff for each escape. The [first replay](evidence/no-diffs-replay-1.txt) and [second replay](evidence/no-diffs-replay-2.txt) used the unchanged fixture and the bundled script directly. Both exited 0 and printed four diffs. The built-in [normal run](evidence/normal.txt) printed none. The bundled script emits its own diff, which appears in the terminal. This increases CI log noise despite the flag.

The first [driver](evidence/invalid-runner.sh) called `go test` directly, did not install the mutant, and returned Go's exit codes rather than the custom runner's codes. Its [100% score](evidence/invalid-driver.txt) was invalid evidence, not a mutago defect. Replacing it with the supplied script corrected the observation. This illustrates that custom runner authors must implement both the mutant swap and the documented exit-code contract.

## Verification and limits

`go build ./...` passed. The [changed-code mutation gate](evidence/change-gate.txt) passed with no changed Go lines and tested no mutants. [The full `go test ./...` run](evidence/checks.txt) failed two tests that require a temporary directory outside any Go module. `TMPDIR` was inside this worktree, so the fixtures inherited the repository's `go.mod`. This is the likely cause, but a run with a temporary directory outside the module was not attempted because this pass required worktree-local build outputs. Other packages in the log passed. The test command restored `example/example.go` afterwards.

Without `--coverage`, [summary JSON](evidence/normal-summary.json) reports `coveredCodeMsi` as zero. The CLI describes that score as computed by `--coverage`, but the JSON reference does not explain the zero in a non-coverage run. Its intended interpretation is unresolved. The first four run headers omit the worker and timeout arguments even though the driver supplied them.

The pass did not cover HTML reports, Windows, macOS, or a workspace. The tests used a small single-package fixture and do not establish behavior for larger projects. No background process, browser, or container was started.
