# mutago [![Go Reference](https://pkg.go.dev/badge/github.com/quality-gates/mutago/v2.svg)](https://pkg.go.dev/github.com/quality-gates/mutago/v2) [![Mutation Testing](https://github.com/quality-gates/mutago/actions/workflows/mutation.yml/badge.svg)](https://github.com/quality-gates/mutago/actions/workflows/mutation.yml) [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE) [![Go 1.26+](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](https://go.dev)

mutago is a mutation testing tool for Go. It tweaks your code in small ways and checks whether your tests catch the change. If they don't, that's a gap in your test suite worth closing.

## Features

Beyond finding escaped mutants, mutago can enforce quality gates in CI — failing builds below a mutation score threshold, filtering to changed lines only, and ignoring previously-accepted survivors.

| Feature | Flag |
| :--- | :--- |
| Quality gates — fail CI below a mutation score | `--min-msi`, `--min-covered-msi` |
| Coverage-aware MSI — score covered lines separately | `--coverage` |
| Baseline file — only fail on *new* escapes | `--baseline`, `--update-baseline` |
| Git diff filter — only mutate changed lines in a PR | `--git-diff-lines` |
| Per-test filtering — run only covering tests per mutant | `--per-test` |
| Parallel execution (all CPUs by default) | `--workers N` |
| LLM-ready escaped-mutant report | `--logger-agentic-json` |
| GitHub Actions annotations | `--logger-github` |
| GitLab Code Quality report | `--logger-gitlab` |
| Compact stats JSON for badges/dashboards | `--logger-summary-json` |
| Per-mutator allowlist / denylist in config | `enable_mutators`, `disable_mutators` |
| Extra flags for every `go test` call | `--test-flags` |
| Fine-grained output filter | `--output-statuses` |
| Quiet mode — suppress killed/skip noise | `--quiet` |
| Suppress diff output | `--no-diffs` |
| Dry-run mode — count mutations without running tests | `--dry-run` |
| Pre-flight check — fail fast if tests already broken | `--noop` |
| Fail on any escape without a score threshold | `--fail-on-escaped` |
| Run a single mutant by stable ID | `--run-mutant-id` |
| Scale per-mutation timeout by baseline run time | `--timeout-coefficient` |
| Live progress display | automatic on TTY |

```bash
go install github.com/quality-gates/mutago/v2/cmd/mutago@latest
```

Full documentation: https://quality-gates.github.io/mutago/

Forked from [avito-tech/mutago](https://github.com/avito-tech/mutago), itself a fork of [zimmski/go-mutesting](https://github.com/zimmski/go-mutesting).

## Quick example

The following command mutates the mutago project with all available mutators.

```bash
mutago github.com/quality-gates/mutago/v2/...
```

For each mutation the tool prints whether the tests caught it. If they didn't, the source code patch is printed so the mutation can be investigated. The following shows an example patch.

```diff
for _, d := range opts.Mutator.DisableMutators {
	pattern := strings.HasSuffix(d, "*")

-	if (pattern && strings.HasPrefix(name, d[:len(d)-2])) || (!pattern && name == d) {
+	if (pattern && strings.HasPrefix(name, d[:len(d)-2])) || false {
		continue MUTATOR
	}
}
```

The example shows that the right term `(!pattern && name == d)` of the `||` operator is made irrelevant by substituting it with `false`. Since this source code change is not detected by the test suite (the tests did not fail), we can mark it as untested code.

## <a name="table-of-content"></a>Table of contents

- [What is mutation testing?](#what-is-mutation-testing)
- [How do I use mutago?](#how-do-i-use-mutago)
  - [Blacklist false positives](#black-list-false-positives)
  - [Quality gates](#quality-gates)
  - [Git diff filtering](#git-diff)
  - [Baseline — only fail on new escapes](#baseline)
  - [LLM-ready report](#agentic-json)
  - [Live progress](#progress)
  - [CI integration (GitHub Actions)](#ci-integration)
  - [JSON output schemas](#json-outputs)
  - [Mutation control via annotations](#mutation-annotations)
- [How do I write my own mutation exec commands?](#write-mutation-exec-commands)
- [Which mutators are implemented?](#list-of-mutators)
- [Other mutation testing tools for Go](#other-projects)
- [Can I make feature requests and report bugs and problems?](#feature-request)

## <a name="what-is-mutation-testing"></a>What is mutation testing?

The definition of mutation testing is best quoted from Wikipedia:

> Mutation testing (or Mutation analysis or Program mutation) is used to design new software tests and evaluate the quality of existing software tests. Mutation testing involves modifying a program in small ways. Each mutated version is called a mutant and tests detect and reject mutants by causing the behavior of the original version to differ from the mutant. This is called killing the mutant. Test suites are measured by the percentage of mutants that they kill. New tests can be designed to kill additional mutants.
> <br/>-- <cite>[https://en.wikipedia.org/wiki/Mutation_testing](https://en.wikipedia.org/wiki/Mutation_testing)</cite>

> Tests can be created to verify the correctness of the implementation of a given software system, but the creation of tests still poses the question whether the tests are correct and sufficiently cover the requirements that have originated the implementation.
> <br/>-- <cite>[https://en.wikipedia.org/wiki/Mutation_testing](https://en.wikipedia.org/wiki/Mutation_testing)</cite>

Although the definition focuses on finding code paths not covered by tests, other flaws can be found too. Mutation testing can for example uncover dead and unneeded code.

Mutation testing is also especially interesting for comparing automatically generated test suites with manually written ones.

It is also one of the strongest tools available for keeping AI-generated code honest. AI tools write plausible-looking code that often slips past code review. Mutation testing checks whether your tests would actually catch a bug — not just whether the code looks right.

## <a name="how-do-i-use-mutago"></a>How do I use mutago?

Install the binary with `go install`:

```bash
go install -v github.com/quality-gates/mutago/v2/...
```

The binary's help can be invoked by executing the binary without arguments or with the `--help` argument.

```bash
mutago --help
```

> **Note**: This README describes only a few of the available arguments. It is therefore advisable to examine the output of the `--help` argument.

The targets of the mutation testing can be defined as arguments to the binary. Every target can be either a Go source file, a directory or a package. Directories and packages can also include the `...` wildcard pattern which will search recursively for Go source files. Test source files with the suffix `_test` are excluded, since this would interfere with the testing process most of the time.

The following example gathers all Go files from the given targets and generates mutations with all available mutators.

```bash
mutago parse.go example/ github.com/quality-gates/mutago/v2/mutator/...
```

Every mutation has to be tested using an [exec command](#write-mutation-exec-commands). By default the built-in exec command is used, which tests a mutation using the following steps:

- Replace the original file with the mutation.
- Execute all tests of the package of the mutated file.
- Report if the mutation was killed.

Alternatively the `--exec` argument can be used to invoke an external exec command. The [/scripts/exec](/scripts/exec) directory holds basic exec commands for Go projects. The [test-mutated-package.sh](/scripts/exec/test-mutated-package.sh) script implements all steps and almost all features of the built-in exec command. It can be for example used to test the [github.com/quality-gates/mutago/v2/example](/example) package.

```bash
mutago --exec "$GOPATH/src/github.com/quality-gates/mutago/scripts/exec/test-mutated-package.sh" github.com/quality-gates/mutago/v2/example
```

The execution will print the following output.

```diff
KILLED example/example.go:18 (statement/remove)
KILLED example/example.go:22 (branch/if)
KILLED example/example.go:24 (numbers/incrementer)
--- Original
+++ New
@@ -16,7 +16,7 @@
        }

        if n < 0 {
-               n = 0
+
        }

        n++
ESCAPED example/example.go:17 (statement/remove)
KILLED example/example.go:26 (arithmetic/base)
KILLED example/example.go:28 (expression/remove)
--- Original
+++ New
@@ -24,7 +24,6 @@
        n += bar()

        bar()
-       bar()

        return n
 }
ESCAPED example/example.go:25 (statement/remove)
KILLED example/example.go:30 (branch/if)
The mutation score is 75.00% (6 killed, 2 escaped, 0 errored, 0 not covered, 0 skipped, 8 total)
The covered-code mutation score is 0.00%
```

The output shows eight mutations. Six were killed (tests detected the mutation — shown as `KILLED`). Two escaped (tests didn't catch them — shown as `ESCAPED`), and their diffs are printed so you can write a test to cover the gap.

The summary shows the **mutation score** (MSI): killed / total. For the example above, 6/8 = 75.00%. A score of 100% means every mutation was caught.

### <a name="black-list-false-positives"></a>Blacklist false positives

Mutation testing can produce false positives when the mutated code path is never reachable, or when the unoptimized path produces the same result as the optimized one. These cases are not bugs in your tests — they just aren't worth tracking down.

Use `--blacklist` with a file that lists the MD5 checksum of each mutation to ignore (one per line). Checksums are derived from only the lines that actually changed, not the whole file, so they stay valid when unrelated code in the same file is edited.

To get the checksum for a mutation, run mutago with `--debug` and copy the hex string printed next to the mutation. For example, if a mutation's checksum is `a1b2c3d4...`, create a file:

```
a1b2c3d4e5f6...
```

The blacklist file, which is named `example.blacklist` in this example, can then be used to invoke mutago.

```bash
mutago --blacklist example.blacklist github.com/quality-gates/mutago/v2/example
```

The execution will print the following output.

```diff
KILLED example/example.go:18 (statement/remove)
KILLED example/example.go:22 (branch/if)
KILLED example/example.go:24 (numbers/incrementer)
--- Original
+++ New
@@ -16,7 +16,7 @@
        }

        if n < 0 {
-               n = 0
+
        }

        n++
ESCAPED example/example.go:17 (statement/remove)
KILLED example/example.go:26 (arithmetic/base)
KILLED example/example.go:28 (expression/remove)
KILLED example/example.go:30 (branch/if)
The mutation score is 85.71% (6 killed, 1 escaped, 0 errored, 0 not covered, 0 skipped, 7 total)
The covered-code mutation score is 0.00%
```

By comparing this output to the original output we can state that we now have 7 mutations instead of 8.

### <a name="skip-make-args"></a>Skipping make() arguments mutation

Before this filter, numeric arguments in make() calls for slices and maps were mutated by incrementer/decrementer mutators, leading to false positives or invalid code:

```go
// Original code
slice := make([]int, 0)  // Capacity argument (0) was mutated

// Mutated versions
slice := make([]int, 1)  // Incrementer mutation
slice := make([]int, -1)   // Decrementer mutation
```

These mutations are almost always irrelevant because:

1. They don't affect logical correctness
2. Capacity/length arguments are typically intentional
3. Tests rarely validate exact allocation sizes

The filter prevents mutations in make() arguments.

### <a name="quality-gates"></a>Quality gates

Use `--min-msi` and `--min-covered-msi` to fail CI if mutation scores drop below a threshold. The tool exits with code 4 when a gate isn't met.

```bash
mutago --min-msi 60 --min-covered-msi 80 ./...
```

Add `--coverage` to generate a coverage profile first. Mutants on uncovered lines are marked "not covered" and excluded from the covered-MSI denominator (so you're not penalised for code your tests don't reach at all).

```bash
mutago --coverage --min-msi 50 --min-covered-msi 75 ./...
```

The final summary includes a per-mutator breakdown so you can see which mutation types your tests are weakest against.

Use `--noop` to run the test suite once without any mutations first. If the clean suite already fails, mutago exits immediately rather than producing meaningless results.

Use `--timeout-coefficient` to scale the per-mutation timeout relative to the baseline test-suite run time (e.g. `--timeout-coefficient 3` allows each mutation up to 3× the clean run). More reliable than a fixed `--exec-timeout` on machines with variable load.

### <a name="git-diff"></a>Git diff filtering (CI mode)

`--git-diff-lines` limits mutation to lines changed since a given git ref. Combine it with `--ignore-msi-with-no-mutations` so the gate passes cleanly on PRs that touch no mutable code.

```bash
mutago \
  --git-diff-lines \
  --git-diff-base master \
  --ignore-msi-with-no-mutations \
  --min-msi 80 \
  ./...
```

Add `--logger-github` to emit escaped mutants as GitHub Actions `::warning` annotations, so they show up inline on the PR diff.

### <a name="baseline"></a>Baseline — only fail on new escapes

If you have existing mutants that survive and your team has accepted them, you can record them in a baseline file. Future runs only fail when a *new* mutant escapes — not an already-known one.

```bash
# First run: record the current survivors and exit 0
mutago --update-baseline ./...

# Normal CI run: fail only if something new escapes
mutago --fail-on-escaped --baseline mutago-baseline.json ./...
```

Commit `mutago-baseline.json` to your repo. The baseline uses stable mutant IDs — they survive refactors that shift line numbers without changing the actual code.

### <a name="agentic-json"></a>LLM-ready report

`--logger-agentic-json` writes `mutago-agentic.json`. Each escaped mutant gets a stable ID, the diff, surrounding context lines, nearby test file paths, a plain-English description of what the mutator did, and a hint for writing a killing test. Feed it to an LLM to get targeted test suggestions.

```bash
mutago --logger-agentic-json --quiet ./...
```

Use `--run-mutant-id` to re-run a single mutant by its stable ID (copy the `id` field from `mutago-agentic.json`). Useful for iterating on a specific test gap without waiting for the full suite.

### <a name="progress"></a>Live progress

When running in a terminal, mutago shows a live progress line on stderr (killed / escaped / skip counts). It clears automatically before the final summary. It is suppressed in `--verbose`, `--debug`, and silent mode.

### <a name="ci-integration"></a>CI integration (GitHub Actions)

A minimal workflow that gates on mutation score. Adjust thresholds to match your project's baseline.

```yaml
name: Mutation Testing
on:
  push:
    branches: [master]
  pull_request:

jobs:
  mutating:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v6
        with:
          go-version: stable
      - run: go build -o /tmp/mutago ./cmd/mutago
      - run: |
          /tmp/mutago \
            --coverage \
            --min-msi 70 \
            --min-covered-msi 80 \
            --logger-github \
            ./...
```

`--logger-github` emits escaped mutants as `::warning` annotations that appear inline on the PR diff. `--coverage` excludes untested code from the covered-MSI denominator so you aren't penalised for dead code your tests never reach.

For GitLab, replace `--logger-github` with `--logger-gitlab`. This writes `mutago-gitlab.json` in GitLab Code Quality format, which GitLab CI picks up as a code quality report on merge requests.

**Adopting gates on a legacy codebase**: record the current survivors first, then only fail on new regressions.

```bash
# Run once to capture current state
mutago --update-baseline ./...
git add mutago-baseline.json
git commit -m "chore: establish mutation baseline"

# CI: only fail if something new escapes
mutago --baseline mutago-baseline.json --fail-on-escaped ./...
```

### <a name="json-outputs"></a>JSON output schemas

#### `--logger-summary-json`

Writes `mutago-summary.json` after each run. Useful for badges, dashboards, and downstream scripts.

```json
{
  "totalMutantsCount": 42,
  "killedCount": 35,
  "escapedCount": 5,
  "errorCount": 0,
  "skippedCount": 2,
  "notCoveredCount": 0,
  "msi": 0.8333,
  "coveredCodeMsi": 0.9211
}
```

| Field | Type | Description |
| :---- | :--- | :---------- |
| `totalMutantsCount` | int | Total mutations generated |
| `killedCount` | int | Mutations caught by tests |
| `escapedCount` | int | Mutations not caught (test gaps) |
| `errorCount` | int | Mutations that caused a build or test error |
| `skippedCount` | int | Mutations skipped (blacklisted or annotated) |
| `notCoveredCount` | int | Mutations on lines with no coverage (requires `--coverage`) |
| `msi` | float | Mutation Score Indicator: killed / total, range 0–1 |
| `coveredCodeMsi` | float | MSI restricted to covered lines, range 0–1 |

#### `--logger-agentic-json`

Writes `mutago-agentic.json` — a richer payload designed for LLM consumption. Each survived mutant gets a stable ID, the unified diff, surrounding context, nearby test file paths, a plain-English description, and a hint for writing a killing test.

```json
{
  "generated_at": "2026-05-19T08:13:38Z",
  "msi": 0.5857,
  "escaped_count": 5,
  "reminder": "A mutant is an example of how this code could be wrong...",
  "mutants": [
    {
      "id": "abc123",
      "file": "pkg/foo/foo.go",
      "line": 42,
      "mutator": "branch/if",
      "diff": "--- Original\n+++ Mutated\n...",
      "context_start_line": 39,
      "context_lines": ["func Foo() {", "  if x > 0 {", "  }"],
      "test_files": ["pkg/foo/foo_test.go"],
      "description": "Removes an if-block body so the condition becomes a no-op",
      "kill_hint": "Write a test that enters this branch and asserts the output or side effect it produces"
    }
  ]
}
```

| Field | Type | Description |
| :---- | :--- | :---------- |
| `generated_at` | string | RFC 3339 timestamp of the run |
| `msi` | float | Overall MSI as a percentage (0–100) — note: summary JSON uses 0–1 ratio |
| `escaped_count` | int | Number of survived mutants |
| `reminder` | string | Plain-English reminder about how to interpret mutants; included as context for LLMs |
| `mutants[].id` | string | Stable hash of file + mutator + diff — survives refactors |
| `mutants[].file` | string | Path to the mutated file, relative to the module root |
| `mutants[].line` | int | Line number of the mutation |
| `mutants[].mutator` | string | Mutator name (e.g. `branch/if`) |
| `mutants[].diff` | string | Unified diff of original vs mutated |
| `mutants[].context_start_line` | int | 1-based line number of `context_lines[0]`; anchors the snippet without guessing |
| `mutants[].context_lines` | []string | Surrounding source lines for orientation |
| `mutants[].test_files` | []string | Test files in the same package |
| `mutants[].description` | string | Human-readable description of what the mutator changed |
| `mutants[].kill_hint` | string | Concrete suggestion for a test that would kill this mutant |

Feed `mutago-agentic.json` to an LLM to get targeted test suggestions for each gap.

### <a name="mutation-annotations"></a>Mutation control via annotations

To further reduce false positives and provide granular control over mutations, 
mutago now supports special comment annotations. These allow you to exclude specific functions, lines, or patterns from mutation.

#### Annotation Types

1. `// mutator-disable-func` — disables all mutations in the function that follows it.

```go
// mutator-disable-func
func CalculateDiscount(price float64) float64 {
    return price * 0.9
}
```

2. `// mutator-disable-next-line <mutator1>, <mutator2>` — disables mutations on the next line. Use `*` for all mutators.

```go
// mutator-disable-next-line *
x = 42

// mutator-disable-next-line branch/if, increment
if x > 0 {
    y += 1
}
```

3. `// mutator-disable-regexp <pattern> <mutator1>, <mutator2>` — disables mutations on any line in the file matching the regex. Use `*` for all mutators.

```go
s := MyStruct{name: "Go"}
s.Method()

// mutator-disable-regexp s\.Method\(\) *
```

All mutation annotations only apply to the file where they are declared. There is no global/cross-file propagation.

## <a name="write-mutation-exec-commands"></a>How do I write my own mutation exec commands?

A mutation exec command is invoked for each mutation. Commands should handle at least the following phases.

1. **Setup** the source to include the mutation.
2. **Test** the source by invoking the test suite and possibly other test functionality.
3. **Cleanup** all changes and remove all temporary assets.
4. **Report** if the mutation was killed.

It is important to note that each invocation should be isolated and therefore stateless. This means that an invocation must not interfere with other invocations.

A set of environment variables, which define exactly one mutation, is passed on to the command.

| Name            | Description                                                               |
| :-------------- | :------------------------------------------------------------------------ |
| MUTATE_CHANGED  | Defines the filename to the mutation of the original file.                |
| MUTATE_DEBUG    | Defines if debugging output should be printed.                            |
| MUTATE_ORIGINAL | Defines the filename to the original file which was mutated.              |
| MUTATE_PACKAGE  | Defines the import path of the original file.                             |
| MUTATE_TIMEOUT  | Defines a timeout which should be taken into account by the exec command. |
| MUTATE_VERBOSE  | Defines if verbose output should be printed.                              |
| TEST_RECURSIVE  | Defines if tests should be run recursively.                               |

A command must exit with an appropriate exit code.

| Exit code | Description                                                                                                   |
| :------   | :--------                                                                                                     |
| 0         | The mutation was killed. Which means that the test led to a failed test after the mutation was applied.       |
| 1         | The mutation is alive. Which means that this could be a flaw in the test suite or even in the implementation. |
| 2         | The mutation was skipped, since there are other problems e.g. compilation errors.                             |
| >2        | The mutation produced an unknown exit code which might be a flaw in the exec command.                         |

Examples for exec commands can be found in the [scripts](/scripts/exec) directory.

## <a name="list-of-mutators"></a>Which mutators are implemented?

### Arithmetic mutators
#### arithmetic/base
| Name	         | Original | Mutated |
| :------------- | :------- | :------ |
| Plus           | +        | -       |
| Minus          | -        | +       |
| Multiplication | *        | /       |
| Division       | /        | *       |
| Modulus        | %        | *       |

#### arithmetic/bitwise
| Name	        | Original | Mutated |
| :------------ | :------- | :------ |
| BitwiseAnd    | &        | &#124;  |
| BitwiseOr     | &#124;   | &       |
| BitwiseXor    | ^        | &       |
| BitwiseAndNot | &^       | &       |
| ShiftRight    | \>>      | <<      |
| ShiftLeft     | <<       | \>>     |

#### arithmetic/assign_invert
| Name	           | Original | Mutated |
| :--------------- | :------- | :------ |
| AddAssign        | +=       | -=      |
| SubAssign        | -=       | +=      |
| MulAssign        | *=       | /=      |
| QuoAssign        | /=       | *=      |
| RemAssign        | %=       | *=      |
| AndAssign        | &=       | &#124;= |
| OrAssign         | &#124;=  | &=      |
| XorAssign        | ^=       | &=      |
| ShlAssign        | <<=      | \>>=    |
| ShrAssign        | \>>=     | <<=     |
| AndNotAssign     | &^=      | &=      |

#### arithmetic/assignment
| Name	           | Original | Mutated |
| :--------------- | :------- | :------ |
| AddAssignment    | +=       | =       |
| SubAssignment    | -=       | =       |
| MulAssignment    | *=       | =       |
| QuoAssignment    | /=       | =       |
| RemAssignment    | %=       | =       |
| AndAssignment    | &=       | =       |
| OrAssignment     | &#124;=  | =       |
| XorAssignment    | ^=       | =       |
| SHLAssignment    | <<=      | =       |
| SHRAssignment    | \>>=     | =       |
| AndNotAssignment | &^=      | =       |

#### arithmetic/negate
Inverts unary minus expressions. Catches code that relies on a sign flip that tests don't verify.

| Name          | Original | Mutated |
| :------------ | :------- | :------ |
| InvertNegation | -x      | +x      |

### Loop mutators
#### loop/break
Name	           | Original | Mutated  |
| :--------------- | :------- | :------- |
| Break            | break    | continue |
| Continue         | continue | break    |

#### loop/condition
Name	                 | Original | Mutated  |
| :--------------------- | :------- | :------- |
| for k < 100            | k < 100  | 1 < 1    |
| for i := 0; i < 5; i++ | i < 5    | 1 < 1    |

#### loop/range_break
It is a loop/condition-like mutator in its purpose: removing iterations from code.  
However, the implementation is slightly different. The mutator adds a break to the beginning of each range loop.

Name	             | Original Body | Mutated Body |
| :----------------- | :------------ | :----------- |
| for i,v := range x | without break | with break   |

### Numbers mutators
#### numbers/incrementer
Name	           | Original | Mutated  |
| :--------------- | :------- | :------- |
| IncrementInteger | 100      | 101      |
| IncrementFloat   | 10.1     | 11.1     |

#### numbers/decrementer
Name	           | Original | Mutated  |
| :--------------- | :------- | :------- |
| DecrementInteger | 100      | 99       |
| DecrementFloat   | 10.1     | 9.1      |

#### numbers/float-negate
Replaces a float literal with its negation. Catches missing sign-handling in arithmetic.

| Name        | Original | Mutated |
| :---------- | :------- | :------ |
| FloatNegate | 3.14     | -3.14   |

### Composite mutators
#### composite/field-clear
Drops one keyed field from a composite literal (struct, map, or keyed array/slice literal), letting it fall back to its zero value. Targets fields that are set to a meaningful value but never asserted by a test — e.g. an options or config struct populated in full where only one or two fields actually matter to the suite. Fields already set to a zero value (`0`, `""`, `false`, `nil`) and positional (unkeyed) elements are skipped to avoid no-op mutations.

| Name       | Original                          | Mutated              |
| :--------- | :-------------------------------- | :------------------- |
| FieldClear | `Config{Timeout: 30, Retries: 3}` | `Config{Retries: 3}` |

### Concurrency mutators
#### concurrency/goroutine-remove
Removes the `go` keyword from goroutine launches, turning concurrent calls into synchronous ones. Kills tests that rely on goroutines running independently.

| Name            | Original       | Mutated   |
| :-------------- | :------------- | :-------- |
| GoroutineRemove | go f()         | f()       |

### Select mutators
#### select/case-remove
Empties the body of each `case` branch in a `select` statement, one at a time.

#### select/default-remove
Empties the `default` branch of a `select` statement.

### Conditional mutators
#### conditional/negated
Name	                          | Original | Mutated  |
| :------------------------------ | :------- | :------- |
| GreaterThanNegation          | \>       | <=       |
| LessThanNegation             | <        | \>=      |
| GreaterThanOrEqualToNegation | \>=      | <        |
| LessThanOrEqualToNegation    | <=       | \>       |
| Equal                           | ==       | !=       |
| NotEqual                        | !=       | ==       |

If you are looking for simple comparison mutators - see [expression-mutators](#expression-mutators)

#### conditional/bool-literal
Swaps `true`↔`false` in assignment right-hand sides and function call arguments. Finds hardcoded boolean values that tests never flip — e.g. a config flag that always stays at its default.

| Name        | Original      | Mutated       |
| :---------- | :------------ | :------------ |
| BoolLiteral | x = true      | x = false     |

#### conditional/not
Removes the `!` operator from negated conditions in `if`, `for`, and `&&`/`||` expressions. Finds negations that tests never exercise the non-negated path of.

| Name           | Original    | Mutated |
| :------------- | :---------- | :------ |
| ConditionalNot | if !x { ... } | if x { ... } |

### Branch mutators
#### branch/case
Empties case bodies.

#### branch/if
Empties branches of `if` and `else if` statements.

#### branch/else
Empties branches of `else` statements.

### Expression mutators
#### expression/comparison
Searches for comparison operators, such as `>` and `<=`, and replaces them with similar operators to catch off-by-one errors, e.g. `>` is replaced by `>=`.

Name	               | Original | Mutated  |
| :------------------- | :------- | :------- |
| GreaterThan          | \>       | \>=      |
| LessThan             | <        | <=       |
| GreaterThanOrEqualTo | \>=      | \>       |
| LessThanOrEqualTo    | <=       | <        |

#### expression/logical
Swaps `&&` and `||` operators.

Name	      | Original | Mutated
| :--------- | :------- | :------
| LogicalAnd | &&       | &#124;&#124;
| LogicalOr  | &#124;&#124; | &&

#### expression/remove
Searches for `&&` and <code>\|\|</code> operators and makes each term of the operator irrelevant by using `true` or `false` as replacements.

#### expression/context-nil
Replaces `context.Context` arguments at call sites with `nil`. Finds code paths that silently ignore a nil context instead of propagating it.

| Name       | Original          | Mutated        |
| :--------- | :---------------- | :------------- |
| ContextNil | f(ctx, x)         | f(nil, x)      |

#### expression/error-guard
Replaces the condition of `if err != nil` and `if err == nil` guards with a boolean constant. Finds error-handling branches that tests never exercise.

| Name       | Original          | Mutated         |
| :--------- | :---------------- | :-------------- |
| ErrNotNil  | if err != nil     | if false        |
| ErrIsNil   | if err == nil     | if true         |

#### expression/errorf-wrap
Downgrades the error-wrapping verb in `Errorf`-style calls from `%w` to `%v`. The message is byte-for-byte identical, but the returned error no longer wraps its cause, so `errors.Is` and `errors.As` against the original error stop matching. Finds code that wraps errors out of habit where no test ever unwraps the result.

| Name       | Original                      | Mutated                       |
| :--------- | :---------------------------- | :---------------------------- |
| ErrorfWrap | `fmt.Errorf("load: %w", err)` | `fmt.Errorf("load: %v", err)` |

#### expression/recover-clear
Neutralises a `recover()` call by turning it into `any(nil)`. Because both expressions have type `any`, the rewrite type-checks in every context, but the recovered value is always nil, so the guarded recovery branch never runs and a panic propagates. Finds deferred recovery blocks that no test exercises.

| Name         | Original                      | Mutated                      |
| :----------- | :---------------------------- | :--------------------------- |
| RecoverClear | `if r := recover(); r != nil` | `if r := any(nil); r != nil` |

#### expression/string-literal
Replaces non-empty string literals in `==` and `!=` comparisons with `""`. Finds code that compares against a specific string value that tests never assert on — e.g. `if err.Error() == "not found"` where an empty-string match would still pass.

| Name          | Original                    | Mutated                  |
| :------------ | :-------------------------- | :----------------------- |
| StringLiteral | `s == "expected"`           | `s == ""`                |

### Statement mutators
#### statement/remove
Removes assignment, increment, decrement and expression statements.

#### statement/remove-self-assign
Removes self-assignment statements (`a = a`). These are typically dead code; this mutator confirms the surrounding tests don't accidentally rely on them.

#### statement/return
Replaces each return value with the zero value for its type (`false` for bool, `0` for int, `""` for string, `nil` for pointers and interfaces). Uses `go/types` for type resolution. Finds functions whose return values tests never validate.

#### statement/defer-remove
Removes the `defer` keyword, turning deferred calls into immediate calls. Tests whether the timing of cleanup matters — e.g. mutex unlocks and file closes that must happen after the function body, not during it.

| Name        | Original   | Mutated |
| :---------- | :--------- | :------ |
| DeferRemove | defer f()  | f()     |

## Config file

There is a configuration file where you can fine-tune mutation testing.  
The config must be written in YAML format.  
If `--config` is provided, mutago will use that file. Otherwise no config file is used.  
The config contains the following parameters:  


| Name                 | Default value | Description                                                                                                                                                        |
|:---------------------| :------------ |:-------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| skip_without_test    | true          | Skip files without _test.go tests.                                                                                                                                 |
| skip_with_build_tags | true          | Skip test files that contain build constraints.                                                                                                                    |
| json_output          | false         | Writes a report.json file with the mutation test report.                                                                                                           |
| html_output          | false         | Writes a mutago-report.html file with the mutation test report.                                                                                              |
| silent_mode          | false         | Do not print mutation stats.                                                                                                                                       |
| min_msi              | 0             | Minimum required MSI (0–100). 0 means no gate.                                                                                                                    |
| min_covered_msi      | 0             | Minimum required covered-code MSI (0–100). 0 means no gate.                                                                                                       |
| exclude_dirs         | []string(nil) | File path prefixes to skip. Any file whose path starts with one of these strings is excluded. `vendor/` skips all files under vendor; `internal/generated` skips any path starting with that string. |
| disable_mutators     | []string(nil) | Mutator names to disable via config. Merged with `--disable` CLI flags. Supports trailing-`*` wildcard (e.g. `arithmetic/*`). |
| enable_mutators      | []string(nil) | Allowlist: if non-empty, only matching mutators run. `--disable` can still exclude entries. Supports trailing-`*` wildcard. |
| ignore_source_lines  | []string(nil) | List of regexes. Any source line matching one of these patterns is skipped entirely. Useful for suppressing mutations on generated code or boilerplate. |

Example config file:

```yaml
skip_without_test: true
min_msi: 70
min_covered_msi: 80
exclude_dirs:
  - vendor/
  - internal/generated
disable_mutators:
  - numbers/incrementer
  - numbers/decrementer
ignore_source_lines:
  - "// Code generated"
  - "nolint"
```

## <a name="write-mutators"></a>How do I write my own mutators?

Each mutator must implement the `Mutator` interface of the [github.com/quality-gates/mutago/v2/mutator](https://pkg.go.dev/github.com/quality-gates/mutago/v2/mutator#Mutator) package. The methods of the interface are described in detail in the source code documentation.

Additionally each mutator has to be registered with the `Register` function of the [github.com/quality-gates/mutago/v2/mutator](https://pkg.go.dev/github.com/quality-gates/mutago/v2/mutator#Mutator) package to make it usable by the binary.

Examples for mutators can be found in the [github.com/quality-gates/mutago/v2/mutator](https://pkg.go.dev/github.com/quality-gates/mutago/v2/mutator) package and its sub-packages.

## <a name="other-projects"></a>Other mutation testing tools for Go

The main active alternative is [gremlins](https://github.com/singerdmx/gremlins). This project is forked from [avito-tech/mutago](https://github.com/avito-tech/mutago), which has been inactive since late 2025.

**gremlins** is well-maintained and simple to use. It has a clean CLI and a solid set of mutators. It does not have MSI quality gates, a baseline file, coverage-aware filtering, or a git-diff mode, so it works well for local exploration but is harder to wire into CI in a way that fails only on new regressions.

**avito-tech/mutago** is the dormant upstream this project was forked from. This fork adds everything in the [features table](#features) above.

If you want the smallest possible tool to run locally and see which mutants survive, gremlins is a reasonable choice. If you want to enforce mutation score thresholds in CI, track accepted escapes in a baseline, or pipe results to an LLM for test suggestions, this tool is the better fit.

## <a name="feature-request"></a>Can I make feature requests and report bugs and problems?

Sure, just submit an [issue via the project tracker](https://github.com/quality-gates/mutago/issues/new) and I'll see what I can do.
