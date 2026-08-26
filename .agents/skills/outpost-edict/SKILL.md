---
name: outpost-edict
description: >
  Outpost mode only. Edict standing rules and role briefs; re-edict after
  summary.
---

# Outpost edict

Load these standing rules for the rest of this session.

<role name="edict-agent">
  <responsibility>Obey the If→Then standing rules below for the rest of this session.</responsibility>
  <responsibility>Use the issue tracker this repo's Agent skills pointer names — read that doc before any tracker write.</responsibility>
  <responsibility>When dispatching as prefect, read the matching role brief below and include its full contents in that agent's first instructions.</responsibility>
</role>

## Standing rules

1. Working on production code: Follow /implement's SKILL.md rigorously.
2. Looking at a bug: Follow /diagnosing-bugs's SKILL.md rigorously.
3. If you need to make a spec: Follow /to-spec's SKILL.md rigorously.
4. If you need to make child tickets or individual tickets: Follow /to-tickets's SKILL.md rigorously.
5. Working on resolving merge conflicts: Always follow /resolving-merge-conflicts's SKILL.md rigorously.
6. Shipping via a PR/MR: Follow /ship-pr's SKILL.md rigorously.

<if>
  <when>Working on production code</when>
  <then>Follow /implement's SKILL.md rigorously</then>
</if>

<if>
  <when>Looking at a bug</when>
  <then>Follow /diagnosing-bugs's SKILL.md rigorously</then>
</if>

<if>
  <when>Need to make a spec</when>
  <then>Follow /to-spec's SKILL.md rigorously</then>
</if>

<if>
  <when>Need to make child tickets or individual tickets</when>
  <then>Follow /to-tickets's SKILL.md rigorously</then>
</if>

<if>
  <when>Working on resolving merge conflicts</when>
  <then>Always follow /resolving-merge-conflicts's SKILL.md rigorously</then>
</if>

<if>
  <when>Shipping via a PR/MR</when>
  <then>Follow /ship-pr's SKILL.md rigorously</then>
</if>

## Role briefs

When you dispatch, read the matching brief and include its full contents in that
agent's first instructions:

- Worker (production code or bug) — [worker.md](worker.md)
- Splicer (merge or conflict-resolution) — [splicer.md](splicer.md)
- Runner (ship ready or merged work) — [runner.md](runner.md)
- Patrol dog (cadence patrol) — [patrol-dog.md](patrol-dog.md)

<if>
  <when>Ticket is production code or a bug</when>
  <then>Read and include [worker.md](worker.md)</then>
</if>

<if>
  <when>Ticket is merge or conflict-resolution</when>
  <then>Read and include [splicer.md](splicer.md)</then>
</if>

<if>
  <when>Work is merged or otherwise ready to ship, and the next step is landing or releasing it</when>
  <then>Read and include [runner.md](runner.md)</then>
</if>

<if>
  <when>Assignment is a patrol</when>
  <then>Read and include [patrol-dog.md](patrol-dog.md)</then>
</if>

## Tracker

Resolve the tracker from this repo, then follow that document for every create,
read, claim, comment, and close:

1. Read the **Issue tracker** pointer under `## Agent skills` in `AGENTS.md` or
   `CLAUDE.md` (usually `docs/agents/issue-tracker.md`).
2. Read the pointed doc. That doc is the configured tracker for this session.

Missing pointer or doc → stop and tell the human to run
`/setup-matt-pocock-skills`.

<if>
  <when>AGENTS.md or CLAUDE.md points at an issue-tracker doc</when>
  <then>Read that doc and use it as the configured tracker for this session</then>
</if>

<if>
  <when>That pointer or doc is missing</when>
  <then>Stop and tell the human to run `/setup-matt-pocock-skills`</then>
</if>

## Config

If `.outpost.json` exists in the cwd, read it per
[outpost.json-FORMAT.md](outpost.json-FORMAT.md). Honour `maxWorkers` /
`maxSplicers` / `maxRunners` / `maxPatrolDogs` when dispatching, and
`patrolEvery` when advancing. Pass `model` and `thinking` into dispatch
instructions when the harness accepts them. Missing file → defaults. Do not
invent harness features.

<if>
  <when>`.outpost.json` exists in the cwd</when>
  <then>Read it per [outpost.json-FORMAT.md](outpost.json-FORMAT.md) and apply caps, preferences, and `patrolEvery`</then>
</if>

<if>
  <when>`.outpost.json` is missing</when>
  <then>Use FORMAT defaults</then>
</if>

## Done when

These six rules are in force for this session, the role briefs above are
reachable, and any cwd `.outpost.json` has been applied or defaulted. Re-run
this skill after any session summary that may have dropped them.

<if>
  <when>Any session summary may have dropped them</when>
  <then>Re-run this skill</then>
</if>
