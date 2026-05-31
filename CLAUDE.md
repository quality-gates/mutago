# mutago

Mutation testing for Go. Applies code mutations and checks whether tests catch them.

## Build & test

```bash
go build ./cmd/mutago
go test ./...
```

All packages pass. `internal/importing` and `internal/parser` were once broken in module mode but are fixed; do not skip them.

## Key packages

| Package | What it does |
| :--- | :--- |
| `cmd/mutago/` | Binary entrypoint; all flag wiring and orchestration |
| `mutator/` | Mutator implementations (arithmetic, branch, composite, concurrency, conditional, expression, loop, numbers, select, statement) |
| `internal/models/` | `Report`, `Stats`, `Mutant` types; MSI and quality gate logic |
| `internal/gitdiff/` | Git diff line filter for `--git-diff-lines` |
| `internal/filter/` | Annotation and skip filters |
| `internal/coverage/` | Coverage profile parsing (`--coverage`) and per-test coverage map (`--per-test`) |
| `internal/parser/` | Package loading and the AST cache (`ClearPackageCache`) |
| `internal/annotation/` | Comment-directive filters (block, chain, function, line, regex) |
| `internal/reportmaker/` | JSON / HTML / agentic report generation |

## Self-mutation and quality gates

`.github/workflows/mutation.yml` runs mutago on itself with two hard gates:

| Gate | Threshold | Flag |
| :--- | :--- | :--- |
| Overall MSI | ≥ 75% | `--min-msi 75` |
| Covered-code MSI | ≥ 80% | `--min-covered-msi 80` |

**Run the gates locally before committing.** Build first, then run against the same package list CI uses:

```bash
go build -o /tmp/mutago ./cmd/mutago
/tmp/mutago \
  --exec-timeout 30 --coverage --min-msi 75 --min-covered-msi 80 \
  github.com/quality-gates/mutago/v2/mutator/arithmetic \
  github.com/quality-gates/mutago/v2/mutator/branch \
  github.com/quality-gates/mutago/v2/mutator/composite \
  github.com/quality-gates/mutago/v2/mutator/concurrency \
  github.com/quality-gates/mutago/v2/mutator/conditional \
  github.com/quality-gates/mutago/v2/mutator/expression \
  github.com/quality-gates/mutago/v2/mutator/loop \
  github.com/quality-gates/mutago/v2/mutator/numbers \
  github.com/quality-gates/mutago/v2/mutator/select \
  github.com/quality-gates/mutago/v2/mutator/statement \
  github.com/quality-gates/mutago/v2/internal/filter \
  github.com/quality-gates/mutago/v2/internal/coverage \
  github.com/quality-gates/mutago/v2/internal/gitdiff \
  github.com/quality-gates/mutago/v2/internal/models
```

Exit code 4 means the gate failed (escaped mutants). Exit code 0 means all gates passed.

## Shipping workflow

Follow these steps in order when landing a change:

1. **Build and test locally** — `go build ./...` and `go test ./...`. Restore `example/example.go` after.
2. **Run quality gates** — use the command in the section below. Exit 0 = pass, exit 4 = escaped mutants.
3. **Manual smoke test** — build the binary and actually run it against a real package. Check that user-facing output looks right. Do not skip this.
4. **Update docs if needed** — if your change adds, removes, or renames a flag, mutator, or user-facing behavior, update `README.md` and `docs/*.md` to match before committing.
5. **Update CHANGELOG.md** — add an entry under `[Unreleased]` describing what changed (Added / Fixed / Changed). Keep entries concise. When releasing, rename the `[Unreleased]` section to the new version tag and update the comparison URL at the bottom.
6. **Commit and push** — fix forward only. No `--force-push` and no `--amend` on published commits. If a hook or check fails, fix it in a new commit. **The `master` branch has push protection — direct pushes are rejected by GitHub. All changes must land via a PR.**
7. **Watch CI** — wait for the Actions run to go green before merging into master. Run `gh pr checks <number>` to confirm every workflow is passing; do not merge if any is red.
8. **Merge to master** — then push master.
9. **Tag and release** — pick the next semver tag. Create a GitHub release page: following prior release pages, use a very succinct style and 8th grade English.

## Conventions

- Quality gates exit with code 4 (not 1) so CI can distinguish "escaped mutants" from "tool error".
- `--min-msi` / `--min-covered-msi` CLI flags default to `-1` (sentinel for "use config or skip gate"); config zero value means no gate.
- **Edit files one at a time using Read then Edit.** Do not use scripts or string-replacement tools to make the same change across many files at once. Small differences between files (naming conventions, existing imports, extra test functions) mean a bulk approach produces inconsistent output that must be cleaned up manually.

## Testing posture

Integration tests live in `cmd/mutago/main_test.go`. They invoke `mainCmd` directly and capture stdout+stderr.

**Do not assert on hardcoded mutation counts.** Counts change whenever a mutator is added or the example test suite improves. They are implementation details, not public behaviour.

**Assert on behaviour instead:**
- The summary line appears (`"mutation score"` is always in the output for normal runs). Exception: `--run-mutant-id` suppresses the summary line and skips quality gates entirely.
- Exit codes are correct (`returnOk`, `returnMsiThresholdNotMet`, etc.).
- JSON report totals are internally consistent: `TotalMutantsCount == KilledCount + EscapedCount + ErrorCount + SkippedCount + NotCoveredCount`.
- Collection lengths match stat fields: `len(Escaped) == EscapedCount`, `len(Killed) == KilledCount`.
- Each escaped mutant's `ProcessOutput` contains `"FAIL"`; each killed one contains `"PASS"`.
- MSI is in `[0.0, 1.0]`.

**For quality gate tests that must fail**, use a threshold that is permanently out of reach (e.g. `--min-msi 101`) rather than relying on the example package having escaped mutants.

**After running `go test ./cmd/mutago/`**, always run `git restore example/example.go`. The integration tests invoke the mutation binary against the example package and the file is sometimes left with mutations applied.
