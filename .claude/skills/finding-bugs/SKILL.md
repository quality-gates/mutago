---
name: finding-bugs
description: Find bugs by coverage-guided, property-based testing of the system under test.
---

Find reproducible bugs by running **coverage-guided, property-based testing** (CGPT) against the system under test (SUT).

The problem: Coverage-guided fuzzing remains a strong method for finding bugs in a given SUT, but generating many random inputs is brute-force and time-consuming.

Property-based testing is a method for **specifying** the properties of your SUT at a high level, and letting a tool e.g. QuickCheck generate inputs for you across a type class to try to falsify that property, thereby finding bugs which require a fix, or otherwise showing that the property is satisfied, e.g. 'For all lists does my list-ordering function yield an ordered list?'.

We can marry these approaches up together with CGPT. 

1. Assess the SUT for prime, user-critical points of interface with the SUT. 
2. Look for existing property-based tests OR other specification-like tests. If the test suite is limited, best to use what we've got rather than quitting out. 
3. Grill the codebase for the most user-critical properties or specifications. The user MAY provide this themselves but it is not required to proceed.
4. Turn each chosen property into an executable predicate over a real user-facing SUT entry point. Treat inputs that fail the property's precondition as discards, not bugs. This is complete when every run returns success, discard, or a concrete counterexample.
5. Build a random generator and a type-aware mutator for the property's input. Prefer automatically derived, structure-aware mutations: recursively mutate fields, replace a value with a same-type subterm, switch to a smaller constructor while reusing compatible fields, or switch to a larger constructor and randomly generate its missing fields. This is complete when every mutation remains a valid value of the input type.
6. Instrument the property and the SUT code it exercises—not the testing framework—for control-flow coverage. This is complete when coverage reflects paths through the tested behaviour.
7. Run the CGPT loop. Generate randomly while the corpus is empty, then mostly mutate coverage-increasing seeds. Keep separate queues for interesting successful and discarded runs; prioritise successes, but retain novel discards with less mutation energy because they may approach the precondition. Give more energy to short runs and seeds that open many new paths.
8. When a seed's energy is exhausted or mutation stops opening paths, return to random generation to escape the local minimum. Continue until a counterexample is found or the agreed test budget is exhausted.
9. On a property failure, shrink the input and rerun it through the same SUT entry point. Report it in simple, domain-accurate language when the minimised input reliably falsifies the property.

Anti-patterns and escape hatches to avoid:
- "I'll just run CGPT on parser.go because coverage-guided fuzzing is just for pure functions" -> misses critical user hot paths in the SUT that we should run CGPT on, opportunites to rigorously test integrations and system behaviour. 
