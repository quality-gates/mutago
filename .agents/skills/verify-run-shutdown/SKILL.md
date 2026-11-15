---
name: verify-run-shutdown
description: Check worker, progress, summary, and temporary-directory shutdown when changing the run lifecycle or execution pool.
---

# Verify run shutdown

The run must wait for workers and the progress monitor before its final summary, and cleanup must execute at most once. Otherwise results can be missing or shutdown can panic on an already closed channel.

Run `./.agents/skills/verify-run-shutdown/check.sh` from the repository root. It needs Go, Java, Python 3 and Bash; the first run downloads a checksum-pinned TLC jar. The model checker explores all orderings of three abstract queued jobs, including jobs skipped after cancellation. It checks that the summary is published at most once and only after the queue, workers and progress are stopped, and that removal follows the summary. A Go probe concurrently calls the real `runSetup.close` three times and checks one summary and removal of its temporary directory. A scratch overlay disables `sync.Once` and must fail with duplicate shutdown; it leaves the checkout untouched. A real engine cancellation during baseline verifies that early exit removes its temporary directory; removing the deferred cleanup in an overlay must make that test fail.

Last matched to `b6995b8`: `RunResolved`, `runSetup.execute`, `runSetup.close`, `startExecutionPool`, `executionPool.submit`, and `executionPool.stop` in `internal/engine/engine.go`. Inspect changes to these functions, including the working tree, before running. Compare every modeled transition to the current code, update the model and probe for intentional changes, and advance the sync point only after confirming they match. Read every model result with `/statechart` and replay any counterexample against the named functions.

The early-exit probe exercises cancellation during baseline, not every preparation error. The model starts with three already-queued jobs; it abstracts submission, worker execution, cancellation and the progress goroutine as atomic steps. A skipped job represents the existing cancellation path, not successful execution. It does not prove liveness, cover dry-run or generate mode, verify the contents of the report, or establish ordering of every external callback. The concrete probe exercises an empty pool and concurrent cleanup, not all modeled worker interleavings. A probe setup failure or unrelated build error is not a successful sensitivity check.
