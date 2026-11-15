---
name: verify-admission-runner
description: Verify candidate deduplication and runner outcome translation when changing scope admission, result recording, or built-in and custom runner protocols.
---

# Verify admission and runner outcomes

A rejected candidate must not consume its edit key: otherwise an eligible candidate with the same edit can disappear from a run. A built-in test success means escaped and a test failure means killed; custom runner exit codes have the opposite convention for zero and one. Build and setup failures are skipped, and driver failures are errors. Reversing either contract reports a false mutation outcome.

## Coverage and sync

Last matched to `002b04a`: `mutantScope.Admit`, `recordOneMutation`, `runnerPolicy.execute`, `runExecJob`, `runBuiltinExec`, `classifyGoTestResult`, `mapTestExitToResult`, `runCustomExec`, and `recordMutantResult`. Inspect the diff from that commit, including uncommitted changes, whenever any covered function, runner contract, or `scopeCandidate` changes. Compare the code with this model and the named behavior tests. Update the check when intended behavior changes and advance the sync point only after a passing check and source comparison.

The TLA+ model treats each candidate admission as an atomic step and explores every sequence for two edit keys. `OnlyAcceptedAreSeen` means rejection cannot poison a key; a counterexample shows a rejection that adds a key before any admission. It abstracts all reasons for rejection into one event. It does not model concurrent discovery, edit construction, program execution, cancellation, or unbounded key sets.

The real-code check runs `TestMutantScopeAdmitDuplicateAndOrder` and `TestRunnerOutcomes`. The former exercises an out-of-scope candidate followed by an eligible candidate with the same key and then a duplicate. The latter executes controlled subprocesses through both runners and checks the reported outcome for success, test failure, build and setup failure, driver errors, and custom exits 0, 1, 2, and 7. These finite cases do not prove the shell scripts or arbitrary commands correct.

## Run and interpret

Run `./.agents/skills/verify-admission-runner/check.sh` from the repository root. It requires Bash, Go, Java, Python 3, and `curl` on the first run to download a SHA-256-pinned TLC jar. `MUTAGO_TLC_JAR` can name the jar. It checks the model and real code, then uses scratch overlays that poison a rejected key and invert the built-in success result. Each broken implementation must fail for the stated behavior, and the unchanged tests must pass again. It also checks that TLC detects the corresponding broken admission model. The checkout is not modified.

Read every model result with `/statechart`. Replay a failing trace against `mutantScope.Admit` and the scope test. A failure caused by setup, compilation, or a changed mutation site is not evidence that the guard works; inspect the covered code and update the probe before using the result. For source-preservation and custom-runner cancellation, also run `/verify-custom-exec-restore`. For changes that were not rechecked, use `/maintain-formal-verification-skill`.
