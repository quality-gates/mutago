---
name: seeking-performance
description: Audit a codebase for performance bottlenecks and produce a severity-ranked Big O report.
---

Grill the relevant production code thoroughly. Inventory its entry points and performance-sensitive paths, then trace loops, recursion, collection pipelines, queries, I/O, allocation, and repeated work through their callers. Account for every subsystem in scope rather than stopping at the first plausible bottleneck.

For each candidate, name the input variables, derive its worst-case time and space complexity from the reachable code, and establish that realistic inputs can reach it. Use profiles or repeatable benchmarks where practical to distinguish an algorithmic concern from an active bottleneck. State uncertainty when workload or runtime evidence is unavailable.

Produce a report; leave the code unchanged unless the user also asks for fixes. Rank findings primarily by asymptotic time complexity, from fastest-growing to slowest-growing (`O(n!)`, `O(c^n)`, higher-degree polynomial, `O(n^2)`, `O(n log n)`, `O(n)`, `O(log n)`, `O(1)`). For a finding with multiple input variables, state how those variables scale when comparing it with other findings; if their growth cannot be compared, say so and use the tie-breakers instead. Use space complexity, call frequency, realistic input size, and measured impact as tie-breakers. For each finding include:

- rank and severity expressed as an asymptotic class;
- location and caller-visible path;
- input variables and current time and space complexity;
- the code-level cause and supporting evidence; for runtime evidence, include the command or workload, runtime or compiler version, and relevant machine and operating-system details needed to reproduce it;
- a remediation direction and its expected complexity;
- confidence and any missing evidence.

Finish only when every subsystem in scope is represented in the coverage summary and every reported finding is supported by source analysis rather than a generic optimization suggestion. If no meaningful bottleneck is found, say so and report what was inspected and ruled out.
