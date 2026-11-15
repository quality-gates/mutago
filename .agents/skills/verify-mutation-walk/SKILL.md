---
name: verify-mutation-walk
description: Verify mutation walk restoration and channel handoff when changing VisitMutations, withMutation, mutationWalk.Visit, or the legacy walk handshakes.
---

# Verify mutation walk

Run from the repository root:

```sh
.agents/skills/verify-mutation-walk/check.sh
```

The check needs Go, Java, Python 3, curl, and shasum. It downloads TLC 1.7.4 if absent and checks the pinned SHA-256. It runs a bounded TLA+ model, overlays Go probes onto the actual walk, then overlays a temporary version with its deferred reset removed. The broken version must fail specifically because the source was not restored; finally the original code must pass again. Neither overlay edits the checkout. The Go probe tests normal return, callback error, cancellation during the callback, and callback panic. It also drives both legacy channel APIs through applied and reset handshakes for two mutations and detects a broken transition from applied acknowledgment to reset.

## Model and scope

Synchronized against production code at `bb1040f`: `VisitMutations`, `mutationWalk.Visit`, `withMutation`, `walkWithHandshake`, `mutationHandshake.acknowledgeApplied`, and `mutationHandshake.acknowledgeReset` in `walk.go`. Re-read these functions on every run; when their behavior changes, update the model, probe, and synchronization commit together. A passing model alone does not establish that the Go code implements it.

`Walk.tla` models two sequential mutations, with separate steps for applying a mutation, callback return or unwind, deferred reset, and advancing. The channel variant models completion of the applied and reset acknowledgments; the implementation owns the corresponding expected phases in `mutationHandshake`. The synchronous variant can return an error, cancel in the callback, or panic. `OriginalOutsideMutation` requires original syntax before another mutation, on the reset notification, and at each terminal state. The Go probe independently checks these observations and exercises both channel result types. Terminal self-loops are deliberate; TLC checks deadlocks elsewhere.

The model assumes `Change` and `Reset` execute atomically and that `Reset` restores the original syntax without panicking. It abstracts AST traversal, mutation contents, other goroutines, cancellation between modeled steps, and channel scheduling. The channel caller may abandon a handshake and leave the goroutine blocked; neither the model nor the probe proves progress in that case. The probe checks representative concrete traces, not every possible implementation or Go interleaving. TLC checks the finite two-mutation model, not an unbounded walk.

## Statechart

The checked chart is in `chart.mmd`. To regenerate it, copy `Walk.tla` and `Walk.cfg` to a scratch directory outside the repository and run the workspace's `/statechart` script:

```sh
<workspace>/.agents/skills/statechart/scripts/statechart <scratch>/Walk.tla \
  --view "$PWD/.agents/skills/verify-mutation-walk/view.mjs" --out <new-output-dir>
```

At the synchronization commit TLC reached 34 states and 40 steps; TypeOK, OriginalOutsideMutation, and NoDeadlock held. The chart merges states that share a phase and hides steps that stay within one chart state. When the model reports a violation, replay the path against the Go implementation before treating it as a real defect.
