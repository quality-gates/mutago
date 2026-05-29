# internal/coverage

## Test cost multiplies with mutation count

The mutation runner calls this package's full test suite once per mutant (~151 mutants here). Each `BuildPerTestProfile` call takes ~2.5 s. That means **one extra `BuildPerTestProfile` call in a test adds ~5 minutes to CI** (151 × 2.5 s ÷ 8 workers). Reuse a single result across multiple assertions rather than calling it twice.

## Always use workers=1 in BuildPerTestProfile test calls

Passing `workers > 1` inside a test that itself runs under the mutation runner creates `mutation_workers × N` concurrent `go test` processes. Under CPU contention, some runs time out. A non-zero exit from a timeout looks like "killed" to the mutation runner — falsely inflating MSI. Use `workers=1`.

## Genuine survivors on Linux

These mutations can never be killed and are expected to escape:

- `filepath.ToSlash` calls — no-op on Linux
- `strings.TrimSpace` on `go test -list` output — output is already clean
- Closing a buffered channel — not observable from the caller
- Appending `nil` to `extraTestFlags` — no-op
- `Benchmark`/`Fuzz` prefix checks — no benchmarks or fuzz tests exist in this repo
