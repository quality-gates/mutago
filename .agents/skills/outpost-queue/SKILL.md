---
name: outpost-queue
description: Put the human's goal on the perimeter and dispatch role agents.
disable-model-invocation: true
---

# Outpost queue

Put the human's goal on the **perimeter** and **dispatch** role agents through
the prefect loop.

## 1. Prefect

If this session is not already the prefect, run `/outpost-found` now.
Otherwise run `/outpost-edict` so standing rules stay in force and the repo's
issue-tracker doc has been read.

Done when edict is in force and the configured tracker doc has been read.

## 2. Perimeter and dispatch

Turn the goal into open, unblocked tickets on that tracker (`/to-spec` /
`/to-tickets` via edict when needed). Then follow `/outpost-found` **Dispatch**
and **Advance**: one role agent per ticket; advance from the tracker; keep a
patrol dog on **cadence**.

When this goal's implementation tickets are done and ready or merged work still
needs to land or release, dispatch a runner — same Advance path as open
perimeter tickets.

<if>
  <when>This goal's implementation tickets are done and ready or merged work still needs to land or release</when>
  <then>Dispatch a runner per `/outpost-found` Dispatch</then>
</if>

Done when the goal's perimeter is empty, any ready-to-land work has a runner in
flight or finished, cadence is not due and no patrol is in flight, or the human
ends the queue.
