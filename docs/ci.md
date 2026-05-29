# CI Integration

## GitHub Actions

A minimal workflow that gates on mutation score:

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

`--logger-github` emits escaped mutants as `::warning` annotations that appear inline on the PR diff.

For high-assurance environments, pin actions to full commit SHAs instead of version tags and enable Dependabot to keep the pins current automatically.

## Adopting gates on a legacy codebase

If you have existing surviving mutants, record them first so CI doesn't fail on day one:

```bash
# Run once locally to record the current state
mutago --update-baseline ./...
git add mutago-baseline.json
git commit -m "chore: establish mutation baseline"
```

Then in CI, only fail when something *new* escapes:

```yaml
- run: |
    /tmp/mutago \
      --baseline mutago-baseline.json \
      --fail-on-escaped \
      ./...
```

Once you've written tests to kill the known survivors, remove them from the baseline with `--update-baseline` and tighten the gate.

## PR-only mode (changed lines only)

Limit mutation to lines changed in the PR to keep feedback fast and relevant:

```yaml
- run: |
    /tmp/mutago \
      --git-diff-lines \
      --git-diff-base origin/master \
      --ignore-msi-with-no-mutations \
      --min-msi 80 \
      --logger-github \
      ./...
```

`--ignore-msi-with-no-mutations` exits 0 cleanly when the PR touches no mutable code (docs-only PRs, etc.).

## Exit codes

| Code | Meaning |
| :--- | :------ |
| 0 | Gates passed (or no mutations generated with `--ignore-msi-with-no-mutations`) |
| 1 | Internal error |
| 4 | A quality gate was not met |

Use `|| true` only when you want advisory-only output (no gate enforcement).

## Recommended thresholds by maturity

| Maturity | `--min-msi` | `--min-covered-msi` |
| :------- | :---------- | :------------------ |
| Legacy/brownfield | — (baseline only) | — |
| Active development | 60 | 75 |
| Stable library | 75 | 85 |
| High-assurance | 90 | 95 |
