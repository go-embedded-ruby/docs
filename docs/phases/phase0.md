# Phase 0 — Vertical slice

Phase 0 is a **thin vertical slice** through the whole system: source → lexer →
parser → AST → compiler → bytecode → VM, runnable end to end. It is deliberately
narrow but *complete* along its width, so every later phase extends a working
machine rather than assembling one.

## What Phase 0 delivers

**Values**

- Integers (backed by `int64`)
- Floats
- Strings
- Local variables

**Operators**

- Arithmetic: `+`, `-`, `*`, `/`, `%` — including Ruby's **floor division**
  semantics
- Comparisons and equality

**Control flow**

- `if` / `elsif` / `else`, `unless`
- `while` / `until`
- Statement modifiers, e.g. `x if y`

**Methods**

- `def` with required parameters
- Recursion
- Implicit and explicit `return`

**I/O built-ins**

- `puts`, `print`, `p`

## The fib example

The canonical Phase 0 program — method definition, required params, recursion,
arithmetic, and a comparison — runs end to end:

```ruby
def fib(n)
  return n if n < 2
  fib(n - 1) + fib(n - 2)
end

puts fib(20)   # => 6765
```

## Exit criteria (met)

- `puts 1 + 2` prints `3` end to end.
- `fib(20)` evaluates to `6765` end to end.
- **~92% test coverage**, with the `object` package at **100%**.
- Behaviour is **differential-tested against MRI**: the same programs are run on
  reference Ruby and the outputs compared.

## Intentionally deferred

To keep the slice thin, several things are explicitly **out of scope** for
Phase 0 and arrive in later [phases](../roadmap.md):

- **No receiver method calls yet** — only top-level/`self` calls and the
  built-ins above (the object model and dispatch are Phase 1).
- **No blocks** (Phase 3).
- **No bignum promotion** — in the Phase 0 slice integers are plain `int64` and
  do not promote on overflow. (Automatic **Bignum** promotion and
  arbitrary-precision literals have since landed — see the
  [roadmap](../roadmap.md).) See
  [Values & object model](../architecture/object-model.md).
