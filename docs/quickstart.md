# Quick Start

## 1. Install

```bash
go install github.com/quality-gates/mutago/v2/cmd/mutago@latest
```

## 2. Run against your package

```bash
mutago ./...
```

Each mutation prints `PASS` (killed — tests detected it) or `FAIL` (escaped — tests missed it). Escaped mutants print a diff showing exactly what changed.

## 3. Add coverage-awareness

Without `--coverage`, uncovered code inflates the "not covered" bucket and can produce a misleadingly low MSI. With it, mutants on untested lines are separated out and excluded from the covered-MSI denominator.

```bash
mutago --coverage ./...
```

## 4. Set quality gates

```bash
mutago --coverage --min-msi 70 --min-covered-msi 80 ./...
```

Exit code 4 means a gate wasn't met. Exit code 0 means all gates passed.

## 5. Reduce noise

Use `--quiet` to suppress killed/skipped output and only show escaped mutants and the summary.

```bash
mutago --quiet --coverage ./...
```

## 6. Limit to changed lines (PR mode)

```bash
mutago \
  --git-diff-lines \
  --git-diff-base main \
  --ignore-msi-with-no-mutations \
  --min-msi 80 \
  ./...
```

## 7. Get LLM-ready suggestions

```bash
mutago --logger-agentic-json --quiet ./...
# Feed mutago-agentic.json to an LLM for targeted test suggestions.
```
