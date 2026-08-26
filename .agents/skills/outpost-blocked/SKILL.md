---
name: outpost-blocked
description: >
  Outpost mode only. Signal this assignment blocked for the prefect.
---

# Outpost blocked

Signal that the assignment this agent was dispatched cannot proceed.

<role name="signaling-agent">
  <responsibility>Write the blocker to the tracker first (authoritative signal).</responsibility>
  <responsibility>Briefly report to the prefect or parent session if a channel exists.</responsibility>
  <responsibility>Exit — no further assignment without a fresh dispatch.</responsibility>
</role>

## 1. Update the tracker

For a ticket assignment: mark it blocked (or equivalent) on the configured
tracker. Write the blocker in plain language: what failed, what is missing, who
or what can unblock it.

For a patrol-dog patrol: say what prevented the patrol (tracker access, missing
scope, harness opaque).

<if>
  <when>Assignment was a ticket</when>
  <then>Mark it blocked (or equivalent); write what failed, what is missing, who or what can unblock it</then>
</if>

<if>
  <when>Assignment was a patrol-dog patrol</when>
  <then>Say what prevented the patrol (tracker access, missing scope, harness opaque)</then>
</if>

Done when the tracker or bark carries the blocker clearly enough for the
prefect to act.

## 2. Report

Tell the prefect (or parent session), briefly:

- which assignment is blocked
- the blocker
- the smallest next action that would unblock it

Then exit.
