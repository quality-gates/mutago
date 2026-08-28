---
name: finding-bugs
description: Find bugs in the current codebase through a number of time-tested methodologies.
---

## Coverage-guided, property-based testing (CGPT)

Find reproducible bugs by running **coverage-guided, property-based testing** (CGPT) against the system under test (SUT).

The problem: Coverage-guided fuzzing remains a strong method for finding bugs in a given SUT, but generating many random inputs is brute-force and time-consuming.

Property-based testing is a method for **specifying** the properties of your SUT at a high level, and letting a tool e.g. QuickCheck generate inputs for you across a type class to try to falsify that property, thereby finding bugs which require a fix, or otherwise showing that the property is satisfied, e.g. 'For all lists does my list-ordering function yield an ordered list?'.

When the correct output is difficult to state directly, define a **metamorphic property** instead: transform one valid input in a way that should preserve or predictably change the result, then compare the two executions. Examples include serialising and reading a value back, reordering independent operations, or applying a transformation that should leave the result unchanged.

A property is not the only way an execution can reveal a bug. Treat an unexpected crash, failed assertion, hang, or report from a check enabled while the program runs as a counterexample too, even when the property itself does not return a result.

We can marry these approaches up together with CGPT. 

1. Assess the SUT for prime, user-critical points of interface with the SUT. 
2. Look for existing property-based tests OR other specification-like tests. If the test suite is limited, best to use what we've got rather than quitting out. 
3. Grill the codebase for the most user-critical properties or specifications. The user MAY provide this themselves but it is not required to proceed.
4. Turn each chosen property into an executable predicate over a real user-facing SUT entry point. A metamorphic predicate must run both the original and transformed inputs, then compare their results according to the named relationship. Treat inputs that fail the property's precondition as discards, not bugs. This is complete when every run returns success, discard, or a concrete counterexample.
5. Build a random generator and a type-aware mutator for the property's input. Prefer automatically derived, structure-aware mutations: recursively mutate fields, replace a value with a same-type subterm, switch to a smaller constructor while reusing compatible fields, or switch to a larger constructor and randomly generate its missing fields. This is complete when every mutation remains a valid value of the input type.
6. Instrument the property and the SUT code it exercises—not the testing framework—for control-flow coverage. This is complete when coverage reflects paths through the tested behaviour.
7. Run the CGPT loop. Generate randomly while the corpus is empty, then mostly mutate coverage-increasing seeds. Keep separate queues for interesting successful and discarded runs; prioritise successes, but retain novel discards with less mutation energy because they may approach the precondition. Give more energy to short runs and seeds that open many new paths.
8. When a seed's energy is exhausted or mutation stops opening paths, return to random generation to escape the local minimum. Continue until a counterexample is found or the agreed test budget is exhausted.
9. On a counterexample, shrink the input and, for a metamorphic property, the transformation's parameters together. Rerun the minimised input or input pair through the same SUT entry point. Report it in simple, domain-accurate language only when it reliably produces the same failure.

Anti-patterns and escape hatches to avoid:
- "I'll just run CGPT on parser.go because coverage-guided fuzzing is just for pure functions" -> misses critical user hot paths in the SUT that we should run CGPT on, opportunites to rigorously test integrations and system behaviour. 

## Static-analysis-guided reproduction

Run the codebase's configured compiler checks, type checker, security analyzers, and path-sensitive analyzers to generate leads. Turn each promising diagnostic into a stable system-level reproducer before calling it a bug.

1. Discover the repository's configured analyzers and their production-code scope. Record the exact commands and configuration before running them.
2. Run each analyzer and retain its version, configuration, diagnostic, and reported control-flow path. This is complete when every retained diagnostic can be regenerated.
3. Deduplicate the diagnostics and rank them by reachable entry point, severity, confidence, and proximity to user-facing or recently changed code. This is complete when every unique diagnostic has a position in the investigation queue.
4. For the highest-ranked diagnostic, derive the input, state, and control-flow preconditions required to reach it, then exercise those preconditions through a real system entry point. Classify it as reproduced, rejected with evidence, or deferred.
5. When reproduced, minimise the input and environment while preserving the failure, then replay it from a clean state. Report it as a bug only when that replay is stable.
6. Continue until every queued diagnostic is classified or the agreed investigation budget is exhausted.

## Runtime-instrumented testing

Runtime instrumentation adds checks to a running program so that invalid behaviour produces a visible report. These checks can expose problems such as invalid memory access, undefined operations, resource leaks, and data races even when the program appears to return the expected result.

1. Inspect the codebase's languages, build configuration, and dependencies to find which runtime checks it supports. Choose checks for the failures that matter here, such as invalid memory use, undefined operations, leaks, or concurrent access. Record the chosen checks and their exact commands.
2. Build the production code and relevant dependencies with those checks enabled. This is complete when the checked program runs and its reports contain enough location information to trace a failure back to the code.
3. Run the existing test suite against the checked program, followed by representative user-facing or API workflows. If CGPT has produced saved inputs, run those as well. This is complete when every chosen source of execution has run or is recorded as unsupported.
4. Group repeated reports by their underlying failure rather than counting every occurrence separately. For each unique report, save the exact input or action sequence, build command, environment, and check configuration needed to produce it.
5. Rebuild from a clean state, reproduce each unique report, and minimise its input or action sequence while preserving the same failure. Where possible, confirm its externally visible effect on the normally built program.
6. Report only failures that repeat under the recorded invocation. Continue until every selected check and source of execution has run or the agreed investigation budget is exhausted.

## Stateful action-sequence testing

Some bugs appear only after a particular sequence of actions changes the system's state. Stateful action-sequence testing generates and runs whole sequences through a real user or API interface, checking the system after every action and reducing failures to the smallest replayable sequence.

Here, state is anything one action leaves for the next: stored data, login status, an open resource, a cache entry, or a protocol phase. Before generating sequences, define how to return the system to a known starting point and what must remain true after each action.

1. Choose a user-critical workflow in which earlier actions can affect later ones. Record the real user or API interface and the known state from which every sequence will begin.
2. List the actions available through that interface. For each action, define when it is valid, what data it accepts, how it may change the state, and what must remain true afterwards. This is complete when every action can be executed and checked automatically.
3. Generate sequences using only actions that are valid in the current state. Vary their data and favour actions and combinations that previous sequences have not exercised.
4. Return the system to the known starting state, execute one sequence, and check the visible result and resulting state after every action. Record each action, its data, and the observations needed to replay the run.
5. When a sequence fails, return to the starting state and replay it until the same failure occurs three consecutive times. Keep failures that meet this bound; record the others as unstable.
6. Minimise a stable failure by removing actions and simplifying their data, returning to the starting state before every attempt. This is complete when no remaining action or available data simplification can be removed without losing the failure.
7. Continue generating sequences until the agreed test budget is exhausted or repeated runs stop reaching new states and action combinations. Report each bug with its starting state and smallest stable sequence.

## Differential testing

Differential testing compares two or more programs that are expected to behave alike. Give each program the same generated test cases. A different result, crash, or hang is a candidate bug, which must then be investigated.

1. Find two or more programs that accept the same inputs and are expected to produce the same results. Record which inputs they all support and any differences that are allowed.
2. Generate a test case and give the same case to every program under the same conditions.
3. Compare what happens. Save the test case if the results differ, or if one program crashes or hangs while the others do not.
4. Run the saved test case again from a clean state. Keep it only if the same difference reliably appears.
5. Use the documented behaviour or another independently stated rule to decide whether the difference reveals a bug and which program is wrong. Until then, keep it as a candidate.
6. Simplify the test case as much as possible while preserving the same difference, then replay it once more.
7. Continue until the agreed test budget is exhausted. Report only stable, simplified cases that have been shown to violate the expected behaviour.

## Concolic path exploration

Some parts of a program run only when the input satisfies a particular combination of conditions. Concolic testing runs the program with a real input, records the conditions that control which path it takes, and solves those conditions to produce another input that takes a different path. Repeating this process can reach code that randomly generated inputs rarely reach.

Use a dedicated concolic tool when one supports the codebase. Otherwise approximate the same loop with the codebase's coverage tools and source code. Choose an unreached branch, identify the conditions an input must satisfy to reach it, and construct such an input. Solve simple conditions directly and use a constraint solver when useful. Run the input through the real program and keep it if it reaches new code or exposes a failure.

1. Choose a user-important operation and the real program interface through which it is performed. Identify the input fields that can affect its behaviour.
2. Run the operation with one or more ordinary inputs. Use a concolic tool to record the paths taken, or use coverage tools and the source code to determine which code ran.
3. Choose a branch that those runs did not take. Record the conditions needed to reach it, including any required files, stored state, configuration, or service responses.
4. Keep the earlier conditions unchanged and reverse the condition controlling the chosen branch. Use the concolic tool, a constraint solver, or direct reasoning to construct a real input that satisfies the new set of conditions.
5. Run the new input through the same program interface from a clean state. Keep it if it reaches new code or exposes a failure. If it misses the chosen branch, identify the missing condition and try again.
6. Repeat from each useful input until a failure is found, the agreed test budget is exhausted, or the remaining branches cannot be reached with the available inputs and environment.
7. Replay every failure against the normally built program, outside any dedicated concolic engine. Keep it only if the same failure reliably appears.
8. Simplify the input and its required environment while preserving the failure. Report the bug with the smallest stable input and the command or action needed to replay it.

Choose the methods that fit the codebase and the failures it can produce. Reuse useful inputs and action sequences across methods: run generated cases with runtime checks, use analyzer findings to choose paths to explore, and use coverage results to guide further testing. Finish when the agreed budgets are exhausted and every candidate has been reproduced, rejected with evidence, or recorded as unstable. Report only bugs with a small, reliable reproducer. `/diagnosing-bugs` covers the next stage: using the confirmed bug and its reproducer to identify the root cause and verify a fix.
