---
name: verify-baseline-progress
description: Verify baseline escape comparison and terminal progress transitions when changing baseline capture, comparison, or progress rendering.
---

# Verify baseline and progress

Run `./.agents/skills/verify-baseline-progress/check.sh` from the repository root. It requires Go, Java, Python 3 and Bash; the first run downloads a checksum-pinned TLC jar. TLC checks all sequences in the bounded model with three mutant IDs. The Go probes exercise the actual baseline comparison for every combination of three known and escaped IDs, both with and without a baseline, and 64 six-event progress/summary sequences with and without a terminal. Scratch Go overlays invert the baseline membership and progress-clear predicates; both must fail for the stated behavioral reasons. The checkout is not modified.

Last matched to `23b5495`: `baseline.Capture`, `Snapshot.Compare`, `NewTerminalReporter`, `progressLine.draw` and `progressLine.clear`. Before running after a change, inspect the differences to those functions and compare their transitions to `Reporting.tla` and the probes. Update the model, probes and sensitivity mutations as needed; advance this sync point only after a clean check and `/statechart` review. Replay counterexamples against the real implementation.

The model abstracts the survivor and baseline sets and the terminal line into atomic operations. It does not model filesystem persistence, the baseline update policy, output content, mutex scheduling, CLI or finalization. The concrete probes check presence, set membership, and whether output was written; they do not prove those excluded properties or unbounded behavior. The separate engine tests cover finalization and file writes.
