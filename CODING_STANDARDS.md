# Coding standards

## Tests

- Strongly prefer integration tests and end-to-end tests over unit tests.
- Strongly prefer exercising real system behaviour over "the tests pass so it must work."
- Only mock third-party services we cannot control. Do not mock code we own.
- For this codebase, the default proof is: run the real CLI (`mainCmd` or built binary) against real packages/fixtures and assert scores, exit codes, and report consistency — not hardcoded mutant counts.

## Comments and docs

- Code comments use ASD-STE100 Simplified Technical English.
- Ground terms in `CONTEXT.md` domain language when that file exists. Do not invent synonyms for glossary terms.
- Do not write comments that only repeat what the code already makes clear.
- Do not put brittle references in README or comments (versions, line numbers, temporary paths, "as of today" claims) when those details are allowed to change.

## Common footguns

- Tautological tests (asserting the mock was called the way the test just configured it).
- Mocks of modules/services we own.
- "Green suite" treated as proof the product works for a user.
- Narrating comments and README drift magnets.
- Cheating complexity or quality gates with denser syntax, hidden branching, or indirection that does not reduce real complexity.
- Asserting exact mutant counts that churn when mutators or the example suite change.
- Leaving `example/example.go` mutated after tests — always `git restore example/example.go` after `go test ./cmd/mutago/`.

## Go

- Format with `gofmt`; keep `go vet` clean. Prefer small packages under `cmd/`, `mutator/`, and `internal/`.
- Parse and mutate Go via `go/ast` and the existing parser/package-loading stack. Do not add a second parsing approach.
- Export only intentional API surface. Keep orchestration in `cmd/mutago`; keep mutators in `mutator/<kind>`.
- Quality-gate failures use exit code **4** (escaped mutants / MSI threshold). Do not collapse that into exit 1; CI distinguishes tool error from gate failure.
- Self-mutation gates: overall MSI ≥ 75% and covered-code MSI ≥ 80% on the package list CI uses. Prefer killing escapes with tests over weakening thresholds.
- Do not assert on hardcoded mutation counts. Assert behaviour: summary line (`mutation score`) when expected, exit codes, JSON totals consistency (`TotalMutantsCount ==` sum of outcome counts), slice lengths vs stat fields, MSI in `[0.0, 1.0]`.
- For tests that must fail a gate, use an unreachable threshold (e.g. `--min-msi 101`), not a brittle dependency on current escapes.
- Restore any in-place mutation targets the suite touches (`example/example.go` and any package left dirty).
- Avoid unused assignments and high cyclomatic complexity that fail the repo's static gates (`ineffassign`, `gocyclo`, messgo).
- Keep module path and major version (`github.com/quality-gates/mutago/v2`) consistent in imports and docs.
