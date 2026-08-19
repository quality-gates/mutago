---
name: outpost-setup
description: Create .outpost.json in the project cwd with reasonable outpost defaults.
disable-model-invocation: true
---

# Outpost setup

Write a project `.outpost.json` so `/outpost-edict` has caps to honour when
dispatching.

Fields and meanings: [outpost.json-FORMAT.md](../outpost-edict/outpost.json-FORMAT.md).

## 1. Check

Look for `.outpost.json` in the project cwd.

<if>
  <when>`.outpost.json` already exists</when>
  <then>Stop. Tell the human the path and contents; do not overwrite</then>
</if>

Done when the file is missing, or this skill has stopped because it exists.

## 2. Write defaults

Create `.outpost.json` in the project cwd with exactly:

```json
{
  "maxWorkers": 3,
  "maxSplicers": 1,
  "maxPatrolDogs": 1
}
```

Omit `model` and `thinking` so the harness defaults apply. The human can add
them later per the format doc.

Done when that file exists on disk with those three fields.

## 3. Confirm

Tell the human the path written and that `/outpost-found` / `/outpost-edict` will
honour these caps. Stop.
