# mutago

**mutago** is a mutation testing tool for Go. It tweaks your code in small ways and checks whether your tests catch the change. If they don't, that's a gap in your test suite worth closing.

## What is mutation testing?

Mutation testing works by making small, syntactically valid changes to your source code — replacing `+` with `-`, emptying `if` branches, removing statements — then running your tests against each modified version. A mutation that passes your tests (an *escaped mutant*) reveals a test you haven't written.

The **Mutation Score Indicator (MSI)** is killed / total. 100% means every mutation was caught.

## Why use it?

- **Finds untested logic** that coverage alone misses. A line can be covered without its behavior being asserted.
- **Keeps AI-generated code honest.** LLMs write plausible-looking code that slips past code review. Mutation testing checks whether your tests would actually catch a bug.
- **Gives CI a real quality gate.** Rather than relying on coverage percentages, you can gate on whether your tests actually *detect changes*.

## Key features

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
| Compact stats JSON for badges/dashboards | `--logger-summary-json` |
| Per-mutator allowlist / denylist in config | `enable_mutators`, `disable_mutators` |
| Extra flags for every `go test` call | `--test-flags` |
| Fine-grained output filter | `--output-statuses` |
| Quiet mode — suppress killed/skip noise | `--quiet` |
| Suppress diff output | `--no-diffs` |
| Dry-run mode — count mutations without running tests | `--dry-run` |
| Live progress display | automatic on TTY |

## Quick install

```bash
go install github.com/quality-gates/mutago/v2/cmd/mutago@latest
```

Then run it against your package:

```bash
mutago --coverage --min-msi 70 ./...
```

See [Quick Start](quickstart.md) for a step-by-step walkthrough.
