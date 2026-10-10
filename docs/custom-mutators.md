# Writing Custom Mutators

mutago exposes a public registration API for custom mutation operators. To use them in the CLI, build a custom binary from a clone of the mutago repository.

## The Mutator signature

```go
type Mutator func(pkg *types.Package, info *types.Info, node ast.Node) []Mutation
```

- `pkg` — the package being mutated (may be nil for packages without type information)
- `info` — type-checking results from `go/types` (may be nil)
- `node` — an AST node; your mutator should type-assert and return nil if the node isn't relevant
- Return a `[]Mutation` where each entry has a `Change` func (apply the mutation) and a `Reset` func (undo it)

## The Mutation type

```go
type Mutation struct {
    Position token.Pos // optional; defaults to the visited node's position
    Change   func()
    Reset    func()
}
```

Both functions close over the AST node and modify it in place. The framework calls `Change`, prints the mutated file, runs the tests, then calls `Reset`.

## Registration

Call `mutator.Register` from an `init()` function. This example changes the string literal `"hello"` to `"HELLO"`, an edit that no built-in mutator makes:

```go
package mypkg

import (
    "go/ast"
    "go/token"
    "go/types"

    "github.com/quality-gates/mutago/v2/mutator"
)

func init() {
    mutator.Register("mypkg/greeting-uppercase", uppercaseGreeting)
}

func uppercaseGreeting(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
    n, ok := node.(*ast.BasicLit)
    if !ok || n.Kind != token.STRING || n.Value != `"hello"` {
        return nil
    }
    original := n.Value
    return []mutator.Mutation{
        {
            Position: n.Pos(),
            Change: func() { n.Value = `"HELLO"` },
            Reset: func() { n.Value = original },
        },
    }
}
```

## Wiring into the binary

Go's `init()` functions only run for imported packages. Blank-import your package in the **full mutago clone**, keeping its module path `github.com/quality-gates/mutago/v2` unchanged. Do not copy `cmd/mutago/main.go` into your own module: it imports mutago's `internal/` packages, which Go forbids importing from outside that module tree. A `replace` directive alone does not lift that restriction.

The following walkthrough keeps your mutator in a separate module. Start in a fresh directory with Go 1.26.6 or newer:

```sh
mkdir custom-mutago
cd custom-mutago
mkdir -p custom/mypkg
cd custom
go mod init example.com/custom
go get github.com/quality-gates/mutago/v2@v2.10.26
```

Save the registration example above as `mypkg/mypkg.go`. Then clone mutago alongside your module and wire it in:

```sh
cd ..
git clone https://github.com/quality-gates/mutago.git
cd mutago
go mod edit -require=example.com/custom@v0.0.0
go mod edit -replace=example.com/custom=../custom
```

Add this line to the existing import block in the clone's `cmd/mutago/main.go`. Leave all existing imports, including the built-in mutators, in place:

```go
_ "example.com/custom/mypkg" // registers mypkg/greeting-uppercase
```

Build and check registration:

```sh
go mod tidy
go build -o mutago ./cmd/mutago
./mutago --list-mutators
```

The list should include `mypkg/greeting-uppercase`. For a published mutator module, use its real module path and version in `require` and the import; omit the local `replace`.

### Try a mutation

From the clone's `mutago` directory, create the target directory:

```sh
mkdir -p ../custom/greeting
```

In `custom/greeting/greeting.go`, create:

```go
package greeting

func Greeting() string { return "hello" }
```

In `custom/greeting/greeting_test.go`, create:

```go
package greeting

import "testing"

func TestGreeting(t *testing.T) {
    if got := Greeting(); got != "hello" {
        t.Fatalf("Greeting() = %q, want hello", got)
    }
}
```

From the clone's `mutago` directory, run:

```sh
cd ../custom
../mutago/mutago --verbose --workers=1 --exec-timeout=30 ./greeting
```

The output should include a `KILLED` mutant named `mypkg/greeting-uppercase` and a mutation score. All built-in mutators remain enabled. mutago removes duplicate edits, so a custom example such as `-x` → `+x` would instead duplicate `arithmetic/negate` and might not appear.

## Guidelines

- Return `nil` quickly if the node type isn't one your mutator handles — the mutator is called for every node in every file.
- Never mutate zero values to the same zero value (e.g. negating `0.0` is a no-op; skip it).
- Use `info.TypeOf(expr)` when you need type information — it returns nil if the expression has no type.
- The name passed to `Register` must be unique; duplicate registration panics at startup.
