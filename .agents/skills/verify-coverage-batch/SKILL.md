---
name: verify-coverage-batch
description: Verify that per-test coverage collection rejects an incomplete batch. Use when changing profile scheduling, cancellation, profile parsing, or result merging in internal/coverage.
---

# Verify per-test coverage batches

A missing or unreadable test profile must prevent a partial coverage map from being returned. A partial map can exclude a test that actually covers a mutant.

## Coverage and sync

Last matched to `7bc356292989fda5ecb9c608c25f518576e48ca7`, specifically `BuildPerTestProfileForPackages`, `profileBatch.collect`, `runPerTestWorker`, and `parsePerTestProfile` in `internal/coverage/coverage.go`. Before a check, inspect changes since that commit and any uncommitted changes to those functions. Compare each transition with `CoverageBatch.tla` and the real-code probe. Update both when intended behavior changes. Move the sync commit forward only after confirming that both still describe the code.

The model treats each job's result delivery as one atomic step and explores good and bad results in every order for three jobs. It checks that a complete result requires every job and no failure. The Go probe exercises the actual batch collector and workers for every good or bad combination of zero to three profiles using scripted test commands. It checks that any error returns no map and that a complete batch retains every test. The model does not execute Go, prove liveness, cover process execution or profile parsing, or model cancellation separately. The Go probe uses two workers but does not establish thread safety or exhaustively explore worker scheduling.

## Run and interpret

Run `./.agents/skills/verify-coverage-batch/check.sh` from the repository root. It requires Go, Java, Python 3, Bash, and, on first use, network access to download a SHA-256-pinned TLC 1.7.4 jar. Set `MUTAGO_TLC_JAR` to use an existing copy. The command runs the model, runs the real-code probe, and uses a temporary Go overlay to remove the merger's error guard. The broken version must fail for accepting a partial profile. It does not alter the checkout.

Read every model result with `/statechart` using `CoverageBatch.tla`, `CoverageBatch.cfg`, and `view.mjs`. Replay a counterexample against the named functions. A setup failure or a failure unrelated to the removed guard is not a successful sensitivity check. If the source no longer matches the scratch mutation, inspect the new implementation and replace the mutation with a real change that admits an incomplete profile. Run the restored check before advancing the sync commit.
