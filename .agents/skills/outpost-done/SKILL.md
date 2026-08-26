---
name: outpost-done
description: >
  Outpost mode only. Signal this assignment complete to the prefect.
---

# Outpost done

Signal completion for the assignment this agent was dispatched.

If acceptance criteria are unchecked, or code work is still uncommitted on the
ticket branch, stop — use `/outpost-blocked` or keep working. Do not complete
this skill.

<role name="signaling-agent">
  <responsibility>Write completion to the tracker first (authoritative signal).</responsibility>
  <responsibility>Briefly report to the prefect or parent session if a channel exists.</responsibility>
  <responsibility>Exit — no further assignment without a fresh dispatch.</responsibility>
</role>

## 1. Update the tracker

For a ticket assignment: close or complete it on the configured tracker. Record
what landed (branch, PR, commit, artifact path — whatever the harness and
tracker use).

For a patrol-dog patrol: write the bark on this patrol ticket and complete it;
leave every other ticket as found.

<if>
  <when>Assignment was a ticket</when>
  <then>Close or complete it on the configured tracker; record what landed (branch, PR, commit, artifact — whatever the harness and tracker use)</then>
</if>

<if>
  <when>Assignment was a patrol-dog patrol</when>
  <then>Write the bark on this patrol ticket and complete it; leave every other ticket as found</then>
</if>

Done when the tracker (and bark) give the prefect enough to advance.

## 2. Report

Tell the prefect (or parent session), briefly:

- which assignment finished
- what landed or what the patrol barked
- anything the next dispatch must know

Then exit.
