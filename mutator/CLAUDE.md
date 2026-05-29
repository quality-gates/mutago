# mutator

## Adding a new mutator

Every mutator needs four things:

1. **Implementation** — a `Mutator*` function in its category package (e.g. `mutator/arithmetic/`).
2. **`init()` registration** — call `mutator.Register(name, fn)` in an `init()` function in the same file. The name must use hyphens, not underscores (e.g. `"arithmetic/my-mutator"`).
3. **Golden file** — add a `.go` input file and one or more `.N.go` expected-output files in `testdata/<category>/`. Do not gofmt these files — see `testdata/CLAUDE.md`.
4. **Test** — a `TestMutator*` function that calls `test.Mutator(...)`, plus an assertion that the mutator is registered (call `mutator.New(name)` and check it returns non-nil).

Missing any of these will either fail the tests or silently prevent `--disable`/`--enable` flags from working.
