---
name: seeking-performance
description: Use controlled benchmarks and profiling to find and prove performance improvements.
---

Find real performance improvements in this codebase using source-level complexity analysis and runtime profiles against a repeatable benchmark of one caller-visible metric.

Make one reversible change at a time; keep it only when the same workload and environment prove a meaningful improvement without unacceptable regressions, otherwise revert it, and report the before-and-after evidence.
