# `.outpost.json` format

Optional file in the project cwd. `/outpost-edict` reads it when present.
Missing file → defaults below. Invalid or unknown fields → ignore those fields;
do not invent harness APIs.

This config is **advisory**. Outpost is skill-driven and agent-agnostic: the
model honours what it can through the harness's own session/subagent controls.

## Fields

| Field | Type | Default | Meaning |
| --- | --- | --- | --- |
| `maxWorkers` | number | unbounded | Soft cap on concurrent Worker agents |
| `maxSplicers` | number | unbounded | Soft cap on concurrent Splicer agents |
| `maxRunners` | number | `1` | Soft cap on concurrent Runner agents |
| `maxPatrolDogs` | number | `1` | Soft cap on concurrent Patrol dog agents |
| `patrolEvery` | number | `3` | Cadence gap: completed non-patrol tickets on this spec between patrols |
| `model` | string | harness default | Preferred model id/name when dispatching |
| `thinking` | string | harness default | Preferred thinking/reasoning level when dispatching |

## Example

```json
{
  "maxWorkers": 3,
  "maxSplicers": 1,
  "maxRunners": 1,
  "maxPatrolDogs": 1,
  "patrolEvery": 3,
  "model": "default",
  "thinking": "high"
}
```
