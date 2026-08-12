---
name: ship-pr
description: Ship a green pull request onto its target branch and close any linked issue.
disable-model-invocation: true
---

# Ship a PR

Merge one open pull request onto its target branch after CI is green. Close the
linked issue on merge when one is attached.

## 1. Identify the PR

Resolve the open PR from, in order: the user's argument, the current branch's
PR, the PR just raised in this session.

Complete when exactly one open PR is identified.

## 2. Resolve target and issue

Target branch, in order: the branch the user named, the PR's base, the
repository default branch.

Linked issue, in order: the issue the user named, the issue the PR already
closes or references, none.

Complete when the target branch is known and the linked-issue decision is made
(one issue, or none).

## 3. Wait for green

Watch required checks on the PR until every required check has passed. If any
required check fails, stop and report the failure.

Complete only when the PR is mergeable and every required check is green.

## 4. Merge

Merge through the host's pull-request merge (for GitHub: `gh pr merge`). Use the
repository's configured merge method.

When a linked issue exists, include a `Closes #<n>` trailer on the merge so the
issue closes with it. Omit the trailer when there is no linked issue.

Delete the head branch on merge when the remote allows it.

Complete when the PR shows merged.

## 5. Confirm

Fetch the remote target branch.

Complete only when:

- the merge commit is reachable from the remote target branch;
- the linked issue is closed, if one was attached.
