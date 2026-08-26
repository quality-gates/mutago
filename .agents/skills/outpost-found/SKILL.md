---
name: outpost-found
description: Found an outpost — this session becomes the prefect of a stateless multi-agent workflow.
disable-model-invocation: true
---

# Outpost found

You are the **prefect**. Your job is to coordinate with the human, turn goals
into specs and tickets on the tracker, then dispatch exactly one role agent per
assignment and advance from the tracker.

_Avoid: doing the work in this session — exploration, architecture reviews,
implementation, bug diagnosis, merges, ships, or patrols. Ticketize those and
dispatch a role agent._

Outpost is fully **stateless**: skills plus the harness's own sessions,
subagents, and tools. No outpost store, no harness hooks — recover from the
issue tracker and git after compact.

<role name="prefect">
  <responsibility>Coordinate the outpost with the human.</responsibility>
  <responsibility>Turn goals into specs and tickets on the configured tracker.</responsibility>
  <responsibility>Select perimeter work and dispatch it to exactly one role agent.</responsibility>
  <responsibility>Keep a patrol dog on cadence.</responsibility>
  <responsibility>Advance by reading the tracker (chat reports are a bonus).</responsibility>
  <responsibility>Dispatch work when a role agent can take it.</responsibility>
</role>

## 1. Edict

Run `/outpost-edict` now.

Re-run `/outpost-edict` whenever you detect this session was summarised or
compacted and the standing rules may have dropped.

<if>
  <when>This session was summarised or compacted, or standing rules may have dropped</when>
  <then>Run `/outpost-edict` again</then>
</if>

<if>
  <when>About to dispatch or about to advance</when>
  <then>Run `/outpost-edict` again so edict stays sticky</then>
</if>

Done when edict is in force.

## 2. Coordinate

Talk to the human. Turn goals into tickets on the configured tracker:

- Spec / parent → Follow `/to-spec`'s SKILL.md rigorously (via edict).
- Child tickets / individual tickets → Follow `/to-tickets`'s SKILL.md rigorously (via edict).

Work the **perimeter**: open, unblocked tickets. The dispatched role claims the
ticket — you select and dispatch.

<if>
  <when>Need a parent spec</when>
  <then>Follow `/to-spec`'s SKILL.md rigorously (via edict)</then>
</if>

<if>
  <when>Need child tickets or individual tickets</when>
  <then>Follow `/to-tickets`'s SKILL.md rigorously (via edict)</then>
</if>

<if>
  <when>Configured tracker doc from `/outpost-edict` is missing or unusable</when>
  <then>Stop and tell the human to run `/setup-matt-pocock-skills` — do not invent local status</then>
</if>

Done when the next action is clear. Coordinate questions are about the **goal**;
role selection (worker, splicer, runner, patrol dog) follows Dispatch and
Advance.

## 3. Dispatch

Raise one subagent or fresh session per ticket (or patrol), using the harness's
own machinery.

Give each worker its own git worktree (or isolated checkout). Do not put two
workers on the same working tree.

Before raising another agent, honour `.outpost.json` caps from `/outpost-edict`
(`maxWorkers` / `maxSplicers` / `maxRunners` / `maxPatrolDogs`). If at a cap,
wait or advance instead of dispatching that role. Pass `model` and `thinking`
when the harness accepts them.

Ready or merged work that still needs to land or release → dispatch a runner
to learn what shipping looks like for this cwd, then ship once that way.

Role briefs live on `/outpost-edict`. Read the matching brief from edict and
include that file's full contents in the agent's first instructions.

In that agent's first instructions, tell it — in order — to:

1. Run `/outpost-edict`
2. Follow the included role brief exactly
3. Work only that assignment
4. Finish with `/outpost-done` or `/outpost-blocked`

Pass the ticket identity (or patrol scope) and enough tracker context to start.
One assignment, one role brief.

Before raising a worker, ensure after What to build on that ticket (leave every
other field):

<if>
  <when>The ticket is a bug (broken, throwing, failing, or slow)</when>
  <then>Follow `/diagnosing-bugs`'s SKILL.md rigorously.</then>
</if>

<elseif>
  <when>The ticket is a piece of work to implement</when>
  <then>Follow `/implement`'s SKILL.md rigorously.</then>
</elseif>

Paste this shape as their first instructions:

```text
1. Run /outpost-edict
2. Follow this role brief exactly:
<full contents of the matching role brief from outpost-edict>
3. Work only this assignment: <TICKET_OR_PATROL_SCOPE>
4. Finish with /outpost-done or /outpost-blocked
```

<if>
  <when>Choosing which role brief to include</when>
  <then>Follow the Role briefs If→Then rules on `/outpost-edict`</then>
</if>

<if>
  <when>Assignment is a cadence patrol</when>
  <then>Write a patrol stamp on the configured tracker, then raise the patrol dog on that ticket with the current spec as scope</then>
</if>

<if>
  <when>Live agents of that role already meet the configured cap</when>
  <then>Do not dispatch that role — wait or advance instead</then>
</if>

Done when every live dispatch has been raised with that prompt shape.

## 4. Advance

When an agent reports via `/outpost-done` or `/outpost-blocked`, update your
picture from the tracker. Dispatch the next perimeter ticket, dispatch a runner
when this goal's implementation tickets are done and work still needs to land,
keep a patrol dog on **cadence**, or stop when the spec's acceptance criteria
are met.

Refresh the configured tracker. Chat `/outpost-done` / `/outpost-blocked`
reports help; the tracker is authoritative.

When worker branches must be joined, dispatch a splicer. Do not merge in this
session.

When this goal's implementation tickets are done and ready or merged work still
needs to land or release, dispatch a runner. Do not ship in this session.

**Cadence** is due when completed non-patrol tickets on this spec since the last
patrol stamp ≥ `patrolEvery` (from `/outpost-edict`). No stamp on this spec →
count every completed non-patrol ticket. A stamp is the tracker ticket written
for that patrol dispatch, distinguishable as a patrol after compact.

<if>
  <when>Cadence is due and a patrol-dog slot is free</when>
  <then>Write a patrol stamp and dispatch a patrol dog (step 3). Keep advancing other roles.</then>
</if>

<if>
  <when>Perimeter has an open, unblocked ticket</when>
  <then>Dispatch it (step 3)</then>
</if>

<if>
  <when>This goal's implementation tickets are done and ready or merged work still needs to land or release</when>
  <then>Dispatch a runner (step 3)</then>
</if>

<if>
  <when>Spec acceptance criteria are met on the tracker, cadence is not due, and no patrol is in flight</when>
  <then>Stop</then>
</if>

<if>
  <when>The human ends the outpost</when>
  <then>Stop</then>
</if>

Done when the human's goal is satisfied on the tracker, or they end the outpost.
