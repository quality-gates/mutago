# Mutators

mutago ships with a full set of built-in mutation operators, organised by category.

## Arithmetic

### arithmetic/base
Swaps binary arithmetic operators.

| Original | Mutated |
| :------- | :------ |
| `+` | `-` |
| `-` | `+` |
| `*` | `/` |
| `/` | `*` |
| `%` | `*` |

### arithmetic/bitwise
Swaps bitwise operators.

| Original | Mutated |
| :------- | :------ |
| `&` | `\|` |
| `\|` | `&` |
| `^` | `&` |
| `&^` | `&` |
| `>>` | `<<` |
| `<<` | `>>` |

### arithmetic/assign\_invert
Inverts compound assignment operators.

| Original | Mutated |
| :------- | :------ |
| `+=` | `-=` |
| `-=` | `+=` |
| `*=` | `/=` |
| `/=` | `*=` |
| `%=` | `*=` |
| `&=` | `\|=` |
| `\|=` | `&=` |
| `^=` | `&=` |
| `<<=` | `>>=` |
| `>>=` | `<<=` |
| `&^=` | `&=` |

### arithmetic/assignment
Strips compound assignment operators, replacing them with plain `=`.

| Original | Mutated |
| :------- | :------ |
| `+=` | `=` |
| `-=` | `=` |
| `*=` | `=` |
| … | `=` |

### arithmetic/negate
Inverts unary minus expressions. Catches code that relies on a sign flip that tests don't verify.

| Original | Mutated |
| :------- | :------ |
| `-x` | `+x` |

## Loop

### loop/break
Swaps `break` and `continue` inside loops.

### loop/condition
Replaces loop conditions with `1 < 1` (always false), causing the loop body to never execute.

### loop/range\_break
Inserts a `break` at the start of each range loop body, causing only the first iteration to run.

## Numbers

### numbers/incrementer
Increments integer and float literals by 1.

### numbers/decrementer
Decrements integer and float literals by 1.

### numbers/float-negate
Replaces a float literal with its negation.

| Original | Mutated |
| :------- | :------ |
| `3.14` | `-3.14` |

## Composite

### composite/field-clear
Drops one keyed field from a composite literal (struct, map, or keyed array/slice literal), letting it fall back to its zero value. Targets fields set to a meaningful value that no test asserts — e.g. a config or options struct populated in full where only a couple of fields matter to the suite. Fields already at a zero value (`0`, `""`, `false`, `nil`) and positional elements are skipped.

| Original | Mutated |
| :------- | :------ |
| `Config{Timeout: 30, Retries: 3}` | `Config{Retries: 3}` |

## Concurrency

### concurrency/goroutine-remove
Removes the `go` keyword from goroutine launches, making concurrent calls synchronous. Kills tests that rely on goroutines running independently.

| Original | Mutated |
| :------- | :------ |
| `go f()` | `f()` |

## Select

### select/case-remove
Empties the body of each `case` branch in a `select` statement, one at a time.

### select/default-remove
Empties the `default` branch of a `select` statement.

## Conditional

### conditional/negated
Negates comparison operators — `>` becomes `<=`, `==` becomes `!=`, etc. Catches off-by-one and inverted condition bugs.

### conditional/bool-literal
Swaps `true`↔`false` in assignment right-hand sides and function call arguments. Finds hardcoded boolean values that tests never flip.

| Original | Mutated |
| :------- | :------ |
| `x = true` | `x = false` |
| `f(true)` | `f(false)` |

### conditional/not
Removes the `!` operator from negated conditions in `if`, `for`, and `&&`/`||` expressions. Finds negations that tests never exercise the non-negated path of.

| Original | Mutated |
| :------- | :------ |
| `if !x { ... }` | `if x { ... }` |

## Branch

### branch/case
Empties `case` bodies in `switch` statements.

### branch/if
Empties the body of `if` and `else if` branches.

### branch/else
Empties the body of `else` branches.

## Expression

### expression/comparison
Shifts comparison operators by one step — `>` becomes `>=`, `>=` becomes `>`. Catches off-by-one boundary errors.

### expression/logical
Swaps `&&` and `||` operators.

### expression/remove
Makes each operand of `&&` and `||` irrelevant by replacing it with `true` or `false`.

### expression/context-nil
Replaces `context.Context` arguments at call sites with `nil`. Finds code paths that silently accept a nil context instead of propagating a real one.

| Original | Mutated |
| :------- | :------ |
| `f(ctx, x)` | `f(nil, x)` |

### expression/error-guard
Replaces the condition of `if err != nil` / `if err == nil` guards with a boolean constant. Finds error-handling branches that tests never enter.

| Original | Mutated |
| :------- | :------ |
| `if err != nil` | `if false` |
| `if err == nil` | `if true` |

### expression/errorf-wrap
Downgrades the error-wrapping verb in `Errorf`-style calls from `%w` to `%v`. The message is identical, but the returned error no longer wraps its cause, so `errors.Is` / `errors.As` stop matching. Finds error wrapping that no test ever unwraps.

| Original | Mutated |
| :------- | :------ |
| `fmt.Errorf("load: %w", err)` | `fmt.Errorf("load: %v", err)` |

### expression/recover-clear
Neutralises a `recover()` call by rewriting it to `any(nil)`. The recovered value is always nil, so the recovery branch never runs and a panic propagates. Finds deferred recovery blocks that no test exercises.

| Original | Mutated |
| :------- | :------ |
| `if r := recover(); r != nil` | `if r := any(nil); r != nil` |

### expression/string-literal
Replaces non-empty string literals in `==` and `!=` comparisons with `""`. Finds code that compares against a specific string value that tests never assert on.

| Original | Mutated |
| :------- | :------ |
| `s == "expected"` | `s == ""` |

## Statement

### statement/remove
Removes assignment, increment, decrement, and expression statements.

### statement/remove-self-assign
Removes self-assignment statements (`a = a`). These are typically dead code; this mutator confirms tests don't accidentally rely on them.

### statement/return
Replaces each return value with the zero value for its type (`false` for bool, `0` for int, `""` for string, `nil` for pointers and interfaces). Uses `go/types` for type resolution. Finds functions whose return values tests never validate.

### statement/defer-remove
Removes the `defer` keyword, turning deferred calls into immediate calls. Tests whether the timing of cleanup matters — e.g. mutex unlocks and file closes that must happen after the function body, not during it.

| Original | Mutated |
| :------- | :------ |
| `defer f()` | `f()` |

## Disabling mutators

Use `--disable <name>` to turn off a specific mutator:

```bash
mutago --disable statement/remove ./...
```

Use `--list-mutators` to list all registered mutator names.
