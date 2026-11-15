---
name: verify-diff-parsers
description: Check the bounded state-machine behavior of the unified-diff and Git changed-line parsers when either parser changes.
---

# Verify diff parsers

Run this skill after changing the unified-diff parser in `internal/parser` or the changed-line parser in `internal/gitdiff`. An incorrect first changed line or changed-line set can cause mutation testing to include or skip the wrong code.

The checked baseline is `7bc356292989fda5ecb9c608c25f518576e48ca7`. Inspect changes since that commit, including uncommitted changes, and assess whether the generated cases and expectations still describe the implementation's intended behavior:

```sh
git diff 7bc356292989fda5ecb9c608c25f518576e48ca7 -- internal/parser internal/gitdiff
git status --short
./.agents/skills/verify-diff-parsers/check.sh
```

Run from the repository root. The script requires Go, Python 3 and Bash. It runs generated Go probes against the production parsers using temporary overlays, then runs each probe against a deliberately broken production guard and requires the probe to fail. Finally it runs the unmodified probes again. A failed mutation check means the probe no longer demonstrates sensitivity to that fault; investigate rather than accepting the run. The overlays are temporary and do not change repository source files.

The unified-diff probe enumerates all 364 edit scripts of length zero through five over context, deletion and addition. Its expected first changed original line comes from the edit script, independently of the parser. It also checks count mismatches, LF and CRLF input, and a variant with a no-newline marker after every generated body line. The Git probe enumerates all 9,331 event sequences of length zero through five over two file headers, deletion-only and nonempty hunks, and an irrelevant line. It derives expected changed-line sets independently and checks the public line-membership behavior and file keys.

These are exhaustive checks only within those finite input domains, not an unbounded proof. They do not cover every malformed diff, arbitrary hunk coordinates and counts, integer limits, Git command execution, all path syntax, or interactions outside the generated sequences. If the parser contract or reachable states change, extend the input generators and their independent expectations, update the mutation checks when their target guards move, and record the new baseline after reviewing the changes.
