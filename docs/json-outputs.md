# JSON Output Schemas

mutago can write three JSON files after each run.

## Full report (`json_output`)

Set `json_output: true` in the configuration file to write `report.json`. Its
top-level `sources` object maps each source-file path to its original text.
Mutants refer to that path through `mutator.originalFilePath`, so consumers can
look up the source once instead of receiving a duplicate copy for every mutant.

The legacy `mutator.originalSourceCode` and `mutator.mutatedSourceCode` fields
remain accepted by the Go model for compatibility, but new reports omit them.

## `--logger-summary-json`

Writes `mutago-summary.json`. Useful for CI badges, dashboards, and downstream scripts.

```json
{
  "totalMutantsCount": 42,
  "killedCount": 35,
  "escapedCount": 5,
  "errorCount": 0,
  "skippedCount": 2,
  "notCoveredCount": 0,
  "timeOutCount": 0,
  "msi": 0.8333,
  "mutationCodeCoverage": 0,
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
| `timeOutCount` | int | Mutations that timed out during testing |
| `msi` | float | Mutation Score Indicator: killed / total, range 0–1 |
| `mutationCodeCoverage` | int | Lines covered by the coverage profile |
| `coveredCodeMsi` | float | MSI restricted to covered lines only, range 0–1 |

`msi` and `coveredCodeMsi` are in the **0–1 range** (not 0–100). Note that the agentic JSON report (`--logger-agentic-json`) uses the **0–100 percentage** scale for its `msi` field — both are correct within their respective formats, but scripts that consume both must account for the difference.

## `--logger-agentic-json`

Writes `mutago-agentic.json`. A richer payload designed for LLM consumption. Each survived mutant gets a stable ID, the unified diff, surrounding context lines, nearby test file paths, a plain-English description of the mutation, and a hint for writing a killing test.

```json
{
  "generated_at": "2026-05-19T08:13:38Z",
  "msi": 58.57,
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
| `msi` | float | Overall MSI as a **percentage (0–100)** — note this differs from the summary JSON, which uses a 0–1 ratio |
| `escaped_count` | int | Number of survived mutants |
| `reminder` | string | A plain-English reminder about how to interpret mutants — useful context when feeding the file to an LLM |
| `mutants[].id` | string | Stable hash of file + mutator + diff — survives refactors that shift line numbers |
| `mutants[].file` | string | Path to the mutated file, relative to the module root |
| `mutants[].line` | int | Line number of the mutation |
| `mutants[].mutator` | string | Mutator name (e.g. `branch/if`, `statement/return`) |
| `mutants[].diff` | string | Unified diff of original vs mutated code |
| `mutants[].context_start_line` | int | 1-based source line number of `context_lines[0]`; use this to anchor the context snippet without guessing offsets |
| `mutants[].context_lines` | []string | Surrounding source lines for orientation |
| `mutants[].test_files` | []string | Test files in the same package |
| `mutants[].description` | string | Human-readable description of what the mutator changed |
| `mutants[].kill_hint` | string | A concrete suggestion for a test that would kill this mutant |

### Usage with an LLM

```bash
mutago --logger-agentic-json --quiet ./...
# Then feed mutago-agentic.json to an LLM:
# "Here are the mutants my tests didn't catch. For each one,
#  write a Go test that would kill it."
```
