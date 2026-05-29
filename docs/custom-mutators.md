# Writing Custom Mutators

mutago exposes a public registration API so you can add mutation operators without forking the binary.

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
    Change func()
    Reset  func()
}
```

Both functions close over the AST node and modify it in place. The framework calls `Change`, prints the mutated file, runs the tests, then calls `Reset`.

## Registration

Call `mutator.Register` from an `init()` function:

```go
package mypkg

import (
    "go/ast"
    "go/token"
    "go/types"

    "github.com/quality-gates/mutago/v2/mutator"
)

func init() {
    mutator.Register("mypkg/flip-sign", flipSign)
}

func flipSign(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
    n, ok := node.(*ast.UnaryExpr)
    if !ok || n.Op != token.SUB {
        return nil
    }
    return []mutator.Mutation{
        {Change: func() { n.Op = token.ADD }, Reset: func() { n.Op = token.SUB }},
    }
}
```

## Wiring into the binary

Because Go's `init()` functions only run for imported packages, you need a fork of `cmd/mutago/main.go` that blank-imports your package:

```go
import (
    _ "github.com/yourorg/mypkg" // registers mypkg/flip-sign
    _ "github.com/quality-gates/mutago/v2/mutator/arithmetic"
    // ... other built-in mutators
)
```

Build your fork with `go build ./cmd/mutago` and use it in place of the upstream binary.

## Guidelines

- Return `nil` quickly if the node type isn't one your mutator handles — the mutator is called for every node in every file.
- Never mutate zero values to the same zero value (e.g. negating `0.0` is a no-op; skip it).
- Use `info.TypeOf(expr)` when you need type information — it returns nil if the expression has no type.
- The name passed to `Register` must be unique; duplicate registration panics at startup.
