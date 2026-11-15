---
name: verify-script-outcomes
description: Check the bundled executor scripts' test-result and driver-error exit protocol when changing their runner or overlay preparation.
---

# Verify bundled script outcomes

A build or setup failure must be skipped, while an actual test failure must kill the mutant. Reversing those outcomes makes the mutation score misleading. A Go driver failure must surface as an execution error rather than a killed mutant.

Run `./.agents/skills/verify-script-outcomes/check.sh` from the repository root. It requires Bash, Python 3 and standard Unix utilities. The check runs both actual scripts with a controlled `go` command and exhausts seven representative result classes: pass, failing test, build failure, setup failure, driver diagnostic, a test diagnostic followed by a failure, and an unknown exit code. It also checks that a failed diff returns a driver error. It expects exit codes 1, 0, 2, 2, 3, 0, 7 and 3 respectively. It then changes a disposable copy of `overlay.sh` so build failures are treated as test failures; that copy must fail for the expected reason. It finally reruns the unmodified scripts. The checkout is untouched.

Last matched to `bb1040f`: `scripts/exec/test-mutated-package.sh`, `scripts/exec/test-current-directory.sh`, and `scripts/exec/overlay.sh`. Inspect the diff from that commit, including uncommitted changes, before rerunning; compare every relevant transition and update the cases for intended changes. Advance the sync commit only after the source review, normal probe and sensitivity check pass. On a counterexample, reproduce the script and output class, then trace diff, overlay preparation, Go invocation and `finish_test_run`; distinguish a changed contract from a regression.

This is bounded exhaustive enumeration of the listed semantic result classes against the shell code, not a proof of all Go output strings or real process schedules. The fake Go executable does not establish that Go emits the assumed text; integration tests in `scripts/exec` use real Go for passing, failing and build-failure cases. The probe does not verify cancellation, descendant processes, overlay contents, shell portability beyond Bash, arbitrary diagnostics, or every output/verbosity combination. The separate `verify-custom-exec-restore` skill covers source preservation under normal, cancelled and timed-out runs.
