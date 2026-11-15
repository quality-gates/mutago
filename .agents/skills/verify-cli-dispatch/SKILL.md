---
name: verify-cli-dispatch
description: Verify CLI early-exit precedence when changing argument parsing or early-exit flags.
---

# Verify CLI early-exit dispatch

Version, help and mutator-list requests must be handled in that order before parse or configuration errors when the corresponding flag has been parsed. Otherwise users can lose access to help or receive the wrong output when combining options.

Run `./.agents/skills/verify-cli-dispatch/check.sh` from the repository root. It requires Bash, Python 3 and Go. Its Go overlay exhaustively checks all 16 combinations of version, help, list and shell-completion state against the real `handleEarlyExitFlags` implementation. It also checks specific `checkArguments` cases where a parsed early-exit option precedes an invalid flag or a missing configuration file. The script runs the probe, removes the version branch in a temporary production-code overlay, requires a precedence failure, then reruns the unchanged implementation. It never modifies production files.

The checked source is `cmd/mutago/main.go` at `78cd68e3b14a9199c73d49b10e1a6dd35bcda865`, specifically `checkArguments` and `handleEarlyExitFlags`. Before using this check, inspect changes to those functions since that commit, including uncommitted changes. If dispatch has changed intentionally, update the generated cases, expectations and scratch mutation; advance the sync commit only after the check passes and the broken implementation fails for the intended reason.

This uses finite enumeration against the Go code because the property concerns a small input domain and no Go source verifier is used here. The option combinations directly set parsed flags, so they do not prove argument parser behavior for every flag ordering. The selected end-to-end argument cases do not cover target resolution, file-list or AST output, config merging, or arbitrary malformed options. Signal registration and first/second signal behavior are outside this check; see `cmd/mutago/signal_test.go` for process-level cancellation checks. A compiler error, setup failure, or unrelated test failure does not count as a detected precedence fault. When behavior differs, reproduce the input in the CLI, trace `checkArguments` and `handleEarlyExitFlags`, and distinguish an intended contract change from a regression before updating the check.
