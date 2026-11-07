---
name: verify-custom-exec-restore
description: Verify source preservation when a bundled custom exec finishes, is cancelled, or times out. Use when changing runCustomExec, its runner, context or timeout, or the scripts in scripts/exec.
---

# Verify custom exec source preservation

**Property:** Either bundled script leaves the source file unchanged while it runs and after `runCustomExec` returns.

**Harm:** A mutant left in a user's source can be mistaken for their own code and committed.

## Coverage and sync

Last matched against the code in PR #362:

| Code | What to match to the model |
| --- | --- |
| `internal/engine/engine.go`: `RunResolved`, `validateAndInitRun`, `initRun`, `mutateAll`, `queueMutation`, `startWorkerPool`, `runExecJob`, `mutateExec`, `runCustomExec` | Passing the run context to each custom exec, applying its timeout, starting the process, waiting, and returning. |
| `internal/engine/mode.go`: `newMutantSink`, `executeSink.admit` | Sending each mutation to the worker queue. |
| `internal/engine/runner.go`: `resolveRunner` | Selecting the custom command and limiting a run to one custom worker. |
| `cmd/mutago/main.go`: `main` | Interrupt and termination cancel the engine's context. |
| `scripts/exec/test-mutated-package.sh`, `scripts/exec/test-current-directory.sh`, `scripts/exec/overlay.sh` | Passing an overlay to `go test` without modifying the original source. |

From the repo root, find the PR's squash commit and inspect the diff since it, including uncommitted changes. Before the PR merges, use its product commit as the baseline:

```sh
baseline=$(git log --all --format=%H --extended-regexp \
  --grep='^fix: preserve source and reject errored mutant runs \(#362\)$' -1)
if [ -z "$baseline" ]; then
  baseline=74ae7fd17fbcd57a484cfa1c0b0dae7008093cc1
fi
git diff "$baseline" -- internal/engine/engine.go internal/engine/mode.go internal/engine/runner.go cmd/mutago/main.go scripts/exec/test-mutated-package.sh scripts/exec/test-current-directory.sh scripts/exec/overlay.sh
```

Read every changed covered function and compare its actual operations and ordering with `CustomExec.tla`, its configurations, and `probe_test.go`. If the scripts differ, check a model for each implementation. Update the model and probe for intended changes, including intermediate states where the real code can be interrupted. Move the sync commit forward only after confirming the verification describes the covered code at the new commit; keep checking the working-tree diff too. If the old commit is unavailable, inspect all covered code instead.

## Run and interpret

From the repo root run:

```sh
./.agents/skills/verify-custom-exec-restore/check.sh
```

On macOS or Linux, it needs Go, Java, Python 3 and Bash. The first run also needs `curl` and network access to download the SHA-256-pinned TLC 1.7.4 jar; `MUTAGO_TLC_JAR` may point to the same jar. It runs the normal-completion model, the cancellation model, and a real-code probe in temporary directories. Exit 0 means all three passed, 1 means at least one check failed, and 2 means the jar checksum was wrong; other setup failures also stop the command. The probe uses a Go test overlay to compile `probe_test.go` into the real engine package and replaces `go test` with a controlled child process. It never targets a user's source file.

Read **every model result** with the pstack `/statechart` skill: use `CustomExec.tla`, the configuration being checked, and `view.mjs`. Read its path for an invariant failure or its chart for a pass. The model is an abstraction, not an automatic translation of Go or Bash; the real-code probe and the source comparison above are required even when TLC passes.

The normal and cancellation models should satisfy `SourcePreserved`; the real probes should pass for both scripts under normal completion, cancellation and timeout. The probes check that the test receives the mutant overlay and that the original remains unchanged while the test is running and after the executor returns.

For a failure, replay the path against the covered code and inspect the named probe. A probe that cannot start, reach its checked preconditions, or finish is a setup error, not evidence that the property failed. A model-only failure that the current code cannot reproduce calls for a model correction; preserve any reproducible source-preservation failure as a finding. For an intended change, update the model and prove the probe goes green before advancing the sync commit. To test sensitivity, insert `cp "$MUTATE_CHANGED" "$MUTATE_ORIGINAL"` before `go test` in a disposable copy of one script. Set `ReplaceSource = TRUE` in the copy's model configs. Confirm that the normal model and the affected script's normal probe both fail. `broken-normal.cfg` is the corresponding model-only variant.

## Boundaries

The model starts when one bundled script is preparing its test. `resolveRunner` limits a run with a custom command to one worker. The model explores cancellation before or after starting the test, killing the script, test completion, script exit, and `runCustomExec` return. It permits the child to finish after its parent is killed. The real probe checks the source and overlay while the child is running, then lets the child finish before checking the source again.

The model assumes the script creates the intended overlay and that the test and its descendants do not themselves write the source. It does not cover arbitrary third-party `--exec` programs, multiple mutations or runs, path quoting, or process crashes outside this path. The model allows `Wait` to return before a descendant exits; real output pipes can delay it. No liveness claim is made.
