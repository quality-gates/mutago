---
name: resume-from-pr
description: Resume interrupted work from a pull request or merge request.
argument-hint: "<pr-or-mr-url>"
disable-model-invocation: true
---

# Resume from PR

Pick up **interrupted work** left on a **pull request** or **merge request** — any
git host. You are the continuation of that work, not a new agent taking a
handoff summary and waiting.

The usual case is a draft, review threads, failing checks, or a branch an
earlier session abandoned.

## 1. Resolve and extract the brief

Run the bundled extractor by absolute path — `scripts/extract-pr.py` next to
this `SKILL.md`:

```bash
python3 /absolute/path/to/resume-from-pr/scripts/extract-pr.py --cwd "$PWD" [url-or-number]
```

- If the user passed a **PR/MR URL** (GitHub, GitLab, Bitbucket, Gitea/Forgejo,
  Azure DevOps, or another host) or a **number**, pass it through.
- If they passed none, omit it — the script selects the open PR/MR for the
  current branch.
- On failure (unparsed URL, missing PR, auth, empty current-branch lookup),
  stop and report the error. Do not invent a PR.

**Done when:** the extractor has printed a **brief** and you know `provider`,
`url`, `number`, `head`, and `head_sha`.

## 2. Ground against the live workspace

Treat the brief as a claim about the world, not ground truth. Before acting:

- If `$PWD` is not the PR's repository, say so once; continue only if the work
  still belongs here or the user pointed at that PR on purpose.
- Fetch the PR head. When the working tree is clean (or its changes belong to
  this PR), check out that head — a worktree is fine — so edits land on the PR.
- Read the **files in play** that still matter. Prefer review-thread paths,
  then the files edited most.
- Check live git state (`status`, branch, SHA, relevant diff) against
  `head_sha` and claimed progress.
- Open any artifact the brief depends on (review thread, failing check log,
  linked issue, spec) rather than trusting the PR's memory of it.

**Done when:** you can state in one short block — goal, what's already done,
what's wrong or unfinished, and the immediate next action — and each claim is
checked against the live tree or a file you just read.

## 3. Continue the interrupted work

Act on the next action immediately. You are resuming, not reporting.

- Carry forward the PR's goal, constraints, and in-flight approach unless the
  live tree shows they are stale.
- Re-invoke any skill the PR was mid-way through, when that skill still applies.
- Skip re-asking for the original goal. Ask only when the brief and the tree
  genuinely conflict and a wrong guess would be destructive.

**Done when:** the interrupted work has moved forward in this turn (edit,
command, diagnosis step, or a blocked question that only the user can answer) —
not when you have merely restated the brief.
