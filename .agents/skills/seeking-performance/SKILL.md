---
name: seeking-performance
description: Find and prove performance improvements in a module, handler, or execution path. Use when code is slow, latency or throughput needs improving, or a profiler points to hot code.
---

# Improve performance

Use one controlled experiment at a time:

**Define → Baseline → Attribute → One change → Remeasure → Keep or revert**

A runtime profile shows where a program spent time during one run. It does not
prove which change will improve the result that matters, and it may not expose
an algorithm that becomes slow only as its input grows. Use both source-level
complexity analysis and runtime evidence before changing code.

## 1. Define the experiment

Write down:

- **Code under study** — the module, handler, request, job, or execution path
- **Success metric** — elapsed time, request latency, frame time, completed work
  per second, or another result a caller can observe
- **Workload** — inputs, input sizes, call rate, concurrency, and duration
- **Environment** — machine, runtime, build flags, configuration, and relevant
  background load
- **Stop condition** — the target to reach, the smallest improvement worth
  pursuing, or the maximum number of failed experiments

Resolve these from the request and repository where possible. State assumptions
and ask only for information that would materially change the experiment.

Keep CPU usage, allocation counts, lock wait time, and time inside individual
functions as diagnostic measurements unless one of them is itself the requested
outcome. Record any secondary limits the change must respect, such as memory,
errors, cost, or startup time.

Complete this step only when every field has a concrete value or an explicit
assumption, and another engineer could reproduce the workload, measure the same
outcome, and tell when to stop.

## 2. Establish a baseline

Build or find the smallest benchmark that represents the workload. Warm up the
system when needed, run the benchmark repeatedly, and record the distribution
of results. For latency, include the percentiles that matter; for throughput,
confirm that completed work and errors are counted correctly.

Keep the workload and environment fixed for the before-and-after comparison.
Observe CPU, memory, I/O, locks, queues, and errors while the benchmark runs so
the apparent limit is recorded with the result.

Complete this step when the baseline is repeatable enough to distinguish the
improvement the experiment is meant to detect.

## 3. Attribute the cost

Search in two ways before choosing a change.

### Complexity scan

Name the input dimensions that can grow, such as records `n`, users `u`, or
edges `e`, and their realistic present and expected sizes. Trace the relevant
call path and derive its time and space complexity in those variables.

Look for algorithmic knots:

- nested work over the same growing input;
- repeated scans, sorts, parsing, copying, or allocation;
- linear lookup performed inside a loop;
- accidental Cartesian products or combinatorial recursion;
- database, network, filesystem, or lock operations repeated per item;
- retries, queues, or recursion without a useful bound;
- an intermediate result whose size grows faster than the input.

Record each suspect operation, its Big-O cost, the variable it grows with, and
the code that calls it. When feasible, run the baseline across several input
sizes and compare the growth curve with the predicted complexity. Big-O
describes growth rather than current wall-clock cost, so rank a complexity
improvement only when realistic input sizes make it relevant.

### Runtime attribution

Use evidence suited to the observed limit:

- CPU-bound work: a sampling profile or flame graph;
- waiting or contention: off-CPU, lock, I/O, or scheduler traces;
- request latency: a trace of the critical path;
- allocation or garbage collection: allocation and heap profiles;
- suspected growth-rate problem: an input-size sweep.

Estimate how much improving each candidate could move the success metric. A hot
function is a candidate, not a conclusion: work may overlap, sit outside the
critical path, or account for too little of the total result to matter.

Rank the complexity and runtime candidates together. Write the strongest one as
a testable hypothesis:

> Changing **X**, and only X, should improve **the success metric** by about
> **Y** under **the workload**, because **this evidence connects X to the
> measured limit**.

Complete this step when the chosen hypothesis names one cause, predicts a
meaningful result, and cites both its supporting evidence and its upper bound.

## 4. Change one thing

Make the smallest reversible change that tests the hypothesis. Preserve
behavior and add or adjust correctness tests when the implementation changes an
algorithm, data structure, batching strategy, cache, or concurrency model.

Complete this step when the patch isolates one hypothesized cause and the
relevant correctness checks pass.

## 5. Remeasure

Run the same protocol as the baseline: same workload, input sizes, environment,
warmup, metric, and summary. Compare distributions rather than single runs.
Repeat or lengthen the experiment when the apparent improvement is within the
measurement noise.

Recheck secondary limits and the observed bottleneck. For a complexity change,
repeat the input-size sweep and confirm both the expected growth curve and the
result at realistic sizes.

Complete this step when the before-and-after result is reproducible and large
enough to classify as an improvement, a regression, or no meaningful change.

## 6. Keep or revert

Keep the patch when the success metric improves by enough to matter, the result
is repeatable, and no secondary limit regresses unacceptably. Make the measured
result and protocol part of the handoff.

Otherwise revert the patch. Record what the experiment ruled out, then return
to attribution with the last accepted result as the baseline.

Complete this step only when the patch is kept or reverted and the evidence is
recorded.

## Stop or continue

Stop when any of these is true:

1. The target has been reached.
2. The best possible improvement in the remaining measured cost is below the
   threshold in the stop condition.
3. No complexity knot is relevant at present or expected input sizes, and no
   runtime candidate has enough predicted payoff.
4. The experiment cannot distinguish the improvement being pursued. Repair the
   measurement before changing more code.
5. Three consecutive hypotheses produced no change worth keeping, unless the
   experiment defined another limit.

Otherwise return to attribution. Use the accepted result as the new baseline
and test the next highest-payoff hypothesis.
