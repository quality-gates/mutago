---
name: verify-exec-source-restore
description: "Check that the shipped custom exec scripts leave MUTATE_ORIGINAL unchanged after every stop, including the SIGKILL from runCustomExec. Use when editing scripts/exec/test-mutated-package.sh, scripts/exec/test-current-directory.sh, runCustomExec, or the custom-exec worker cap in resolveRunner."
---

# Verify custom exec restores the source

When `scripts/exec/test-mutated-package.sh` or `scripts/exec/test-current-directory.sh` stops, `MUTATE_ORIGINAL` is the same bytes it was before the script started.

A timeout or cancel during `--exec` leaves the user's Go file as the mutant, or deletes it, and the next commit can ship that mutant.

## When a change touches this

Run the check when a change edits any of these:

- `scripts/exec/test-mutated-package.sh` — `clean_up`, `sig_handler`, the `mv` onto `MUTATE_ORIGINAL.tmp`, the `cp` of `MUTATE_CHANGED`, the `go test` line
- `scripts/exec/test-current-directory.sh` — those same steps
- `runCustomExec` in `internal/engine/engine.go` — the `exec.CommandContext` that starts the script, and the timeout that cancels it
- `resolveRunner` in `internal/engine/runner.go` — `r.workers = 1` for a custom command

A change that reaches those functions through a caller counts too.

## Run

From the repository root:

```bash
bash .agents/skills/verify-exec-source-restore/check.sh
```

The machine needs bash, Go, and coreutils. The command uses no network and finishes in a few seconds. On the sync commit it exits 1.

## Read the result

The command runs 30 schedules: both scripts, the pauses `after_diff`, `after_save`, `after_copy`, `during_test`, and `after_restore`, and the stops `none` (run to exit), `TERM` (the process group), and `KILL` (the script process only). It also starts a throwaway `exec.CommandContext` and reads its wait error.

- Exit 0 means every schedule left the original bytes.
- Exit 1 means the property failed. Each `FAIL` line is one schedule. The known finding is the six `KILL` lines in the next section. Any other `FAIL` line is a new break. Keep the known lines as failures.
- Exit 2 means a `MODEL` line: the harness and the code disagree. Update the harness from the code, then rerun, before treating a file result as a finding.

`go test` is stubbed to exit 0. A `FAIL` line is about the file bytes.

## Known finding

`runCustomExec` reaches `during_test` with `KILL`. `--exec-timeout` and a cancelled run use `exec.CommandContext`. Its default `Cancel` calls `os.Process.Kill`, and `Kill` sends `SIGKILL` to the script process. The trap names `SIGHUP`, `SIGINT`, and `SIGTERM`, so `clean_up` does not run.

Both scripts report this for `during_test`:

```
FAIL scripts/exec/test-mutated-package.sh during_test KILL
  diff read MUTATE_ORIGINAL and MUTATE_CHANGED.
  mv renamed MUTATE_ORIGINAL to MUTATE_ORIGINAL.tmp.
  cp copied MUTATE_CHANGED onto MUTATE_ORIGINAL.
  go test was running.
  SIGKILL reached the script process only, the same stop as runCustomExec: exec.CommandContext calls os.Process.Kill, and Kill sends SIGKILL to that process.
  The trap names SIGHUP, SIGINT, and SIGTERM, so clean_up did not run.
  MUTATE_ORIGINAL holds the mutant.
```

`test-current-directory.sh` prints the same steps for `during_test`. `after_save` with `KILL` leaves `MUTATE_ORIGINAL` gone and the original bytes only in `MUTATE_ORIGINAL.tmp`. `after_copy` with `KILL` leaves the mutant in place. Both scripts fail those two pauses too: six `FAIL` lines. The summary line is `25 restored, 6 broke the property, 0 harness gap(s)`.

`none` and `TERM` leave the original bytes. `KILL` after `clean_up` leaves them too, because that `mv` has already put the file back. `KILL` before the save `mv` leaves the file untouched.

## Bounds and assumptions

- Each pause is one command in the script, after that command has finished its change to the file: `diff`, the save `mv`, `cp`, `go test`, and the `mv` inside `clean_up`.
- The stub `go test` exits 0 and does not compile a package. The file bytes do not depend on the test result.
- The file is `foo.go`, with no space or glob character in the path. The scripts expand `$MUTATE_ORIGINAL` unquoted.
- `diff`, `mv`, and `cp` succeed.
- One script runs at a time. `resolveRunner` sets `r.workers = 1` for a custom command. If that assignment is gone the command exits 2, and these schedules do not cover the run.

## Update the check when the change is intended

1. Read the new scripts and the new `runCustomExec` and `resolveRunner`. If a paused command was renamed, split, or removed, change the anchors and the pause in `.agents/skills/verify-exec-source-restore/check.sh` so each pause is still that one command. Write the harness from the code.
2. Rerun the command in Run. Done means the exit code and the `FAIL` lines match the code you just read.
3. Change the property sentence only when the promise itself changed on purpose. The evidence is a comment, a test, or a changelog entry that says the source file may stay mutated. A failure with no such evidence stays a finding. Leave the product code alone unless the user asked for a fix.
4. Move the sync commit below to the new commit only after the harness and the property describe that commit. A commit that leaves the covered code as it was does not move the sync commit.

Edits that landed without this check get `/maintain-formal-verification-skill`.

## Outside this check

- The built-in runner tests with `go test -overlay` and does not swap the source file.
- A user's own `--exec` script.
- Two custom execs at once. The worker cap is an assumption, reported as a `MODEL` line when the assignment disappears.
- An `mv` or `cp` that fails. The scripts have no `set -e`.
- A path that contains a space or a glob character.

## Coverage record

Sync commit: `e855cca5ca7041ff4b28d38d837c7c8b45c3b7de`

| Code | What the schedules follow |
| --- | --- |
| `scripts/exec/test-mutated-package.sh` | `clean_up`, `sig_handler`, the save `mv`, `cp`, `go test` |
| `scripts/exec/test-current-directory.sh` | those same steps |
| `internal/engine/engine.go` `runCustomExec` | `exec.CommandContext` and the timeout cancel |
| `internal/engine/runner.go` `resolveRunner` | `r.workers = 1` for a custom command |

The sync commit is the covered code. The skill commit does not change those files.
