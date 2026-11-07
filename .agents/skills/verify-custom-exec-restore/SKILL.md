---
name: verify-custom-exec-restore
description: Verify source restoration when a bundled custom exec finishes, is cancelled, or times out. Use when changing runCustomExec, its runner, context or timeout, or scripts/exec/test-mutated-package.sh or test-current-directory.sh.
---

# Verify custom exec source restoration

**Property:** When `runCustomExec` returns from either bundled script, the source file contains its original contents.

**Harm:** A mutant left in a user's source can be mistaken for their own code and committed.

## Coverage and sync

Last matched against commit `e855cca5ca7041ff4b28d38d837c7c8b45c3b7de`:

| Code | What to match to the model |
| --- | --- |
| `internal/engine/engine.go`: `RunResolved`, `validateAndInitRun`, `initRun`, `mutateAll`, `queueMutation`, `startWorkerPool`, `runExecJob`, `mutateExec`, `runCustomExec` | Passing the run context to each custom exec, applying its timeout, starting the process, waiting, and returning. |
| `internal/engine/mode.go`: `newMutantSink`, `executeSink.admit` | Sending each mutation to the worker queue. |
| `internal/engine/runner.go`: `resolveRunner` | Selecting the custom command and limiting a run to one custom worker. |
| `cmd/mutago/main.go`: `main` | Interrupt and termination cancel the engine's context. |
| `scripts/exec/test-mutated-package.sh`, `scripts/exec/test-current-directory.sh`: `clean_up`, `sig_handler`, and the main script | Saving the original, installing the mutant, running `go test`, and restoring the backup on the normal and trapped-signal paths. |

From the repo root, inspect the diff since that commit for these files, including uncommitted changes:

```sh
git diff e855cca5ca7041ff4b28d38d837c7c8b45c3b7de -- internal/engine/engine.go internal/engine/mode.go internal/engine/runner.go cmd/mutago/main.go scripts/exec/test-mutated-package.sh scripts/exec/test-current-directory.sh
```

Read every changed covered function and compare its actual operations and ordering with `CustomExec.tla`, its configurations, and `probe_test.go`. If the scripts differ, check a model for each implementation. Update the model and probe for intended changes, including intermediate states where the real code can be interrupted. Move the sync commit forward only after confirming the verification describes the covered code at the new commit; keep checking the working-tree diff too. If the old commit is unavailable, inspect all covered code instead.

## Run and interpret

From the repo root run:

```sh
./.agents/skills/verify-custom-exec-restore/check.sh
```

On macOS or Linux, it needs Go, Java, Python 3 and Bash. The first run also needs `curl` and network access to download the SHA-256-pinned TLC 1.7.4 jar; `MUTAGO_TLC_JAR` may point to the same jar. It runs the normal-completion model, the cancellation model, and a real-code probe in temporary directories. Exit 0 means all three passed, 1 means at least one check failed, and 2 means the jar checksum was wrong; other setup failures also stop the command. The probe uses a Go test overlay to compile `probe_test.go` into the real engine package and replaces `go test` with a controlled child process. It never targets a user's source file.

Read **every model result** with the pstack `/statechart` skill: use `CustomExec.tla`, the configuration being checked, and `view.mjs`. Read its path for an invariant failure or its chart for a pass. The model is an abstraction, not an automatic translation of Go or Bash; the real-code probe and the source comparison above are required even when TLC passes.

At the sync commit, the normal model and both normal probes pass. The cancellation model fails `SourceRestored`, and both scripts fail the `cancel` and `timeout` probes: the script installed the mutant, Go killed the script while its test was running, and `runCustomExec` returned while the mutant remained. The original was still in `source.go.tmp` in these probes. This is a product finding, not an expected success; the command exits 1.

For a failure, replay the path against the covered code and inspect the named probe. A probe that cannot start, reach its checked preconditions, or finish is a setup error, not evidence that the property failed. A model-only failure that the current code cannot reproduce calls for a model correction; preserve any reproducible source-restoration failure as a finding. For an intended fix, update the model and prove the probe goes green before advancing the sync commit. To test sensitivity, remove the normal `clean_up` call in a disposable copy, set `RestoreNormally = FALSE` in its model configs, and confirm the normal model and the affected script's normal probe both fail; restore the copy afterward. `broken-normal.cfg` is the corresponding model-only variant.

## Boundaries

The model starts after one script has successfully moved the original to `.tmp` and copied one mutant, with one `go test` child running. `resolveRunner` limits a run with a custom command to one worker. The model explores the possible order of cancellation, killing the script, test completion, cleanup, script exit, and `runCustomExec` return. It assumes a successful backup and restore on the normal path and permits the child to finish after its parent is killed. The real probe checks the starting condition and lets that child finish before asserting the final source.

It does not cover cancellation during the initial `mv` or `cp`, failed filesystem operations, arbitrary third-party `--exec` programs, multiple mutations or runs, path quoting, or process crashes outside this path. The model allows `Wait` to return before a descendant exits; real output pipes can delay it. No liveness or recovery-after-crash claim is made.
