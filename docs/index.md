# go-embedded-ruby documentation

**A pure-Go implementation of Ruby** — a bytecode VM with the lexer, parser, and
compiler embedded in the binary, compiling to a single static executable with
**zero cgo**.

go-embedded-ruby compiles Ruby source to bytecode and runs it on a stack VM in
the **mruby/YARV lineage**. Because the front-end (lexer + parser + compiler)
ships *inside* the binary, dynamic features like `eval` and runtime `require`
work without a separate toolchain. Ruby objects are ordinary Go heap objects, so
**Go's garbage collector is reused** rather than reimplemented.

Being pure Go buys three things at once: trivial cross-compilation, a single
static binary, and — most importantly — it is the **first Ruby embeddable in a
Go program with no C toolchain** ([go-mruby][go-mruby] needs cgo). The project
targets **Ruby 4.0** semantics and is grown test-first against an MRI 4.0.5
*differential oracle*, with **100% coverage enforced in CI across all six 64-bit
architectures** (amd64, arm64, riscv64, loong64, ppc64le, s390x) and three OSes.

## Supported today

Every feature below is **differential-tested against MRI Ruby 4.0.5**:

- **Values:** integers (`int64`), floats, strings, symbols, arrays, hashes,
  ranges (incl. beginless/endless), `true`/`false`/`nil`, `self`, `Proc`/lambda,
  `Regexp`/`MatchData`, `Struct`.
- **Operators:** arithmetic (`+ - * / %`, Ruby floor division, `**`),
  comparison/`<=>`, `==`/`===`, `<<`, `&&`/`||`, ternary, ranges; correct
  negative-literal precedence (`-2.abs == 2`, `-2**2 == -4`).
- **Control flow:** `if`/`elsif`/`else`, `unless`, `while`/`until`,
  `case`/`when`, statement modifiers, `begin`/`rescue`/`ensure`/`else`/`retry`,
  `break`/`next`.
- **Methods:** required / optional / `*splat` / keyword (`a:`, `b: 2`) /
  `**rest` / `&block` parameters, setter defs (`def name=`), **endless methods**
  (`def foo = expr`), recursion, `return`, `super`.
- **Blocks / Procs / lambdas:** `{ }` / `do…end` closures, `yield`,
  `block_given?`, `&block` capture, `Proc`/`lambda`/**stabby `->(){}`**, `&proc`
  block-pass and `Symbol#to_proc` (the `&:sym` shorthand).
- **Classes & modules:** inheritance, `@ivars`, `new`/`initialize`, constants and
  constant assignment, **class methods** (`def self.foo`), modules + `include`
  (mixins), `super`, **`attr_accessor`/`reader`/`writer`**, **`Struct.new`**.
- **Metaprogramming:** dynamic dispatch via mutable method tables,
  `method_missing`, `send`/`public_send`, `respond_to?`, **`define_method`**,
  **`instance_eval`/`instance_exec`**, **`class_eval`/`module_eval`/`class_exec`**,
  `instance_variable_get`/`set`/`defined?`.
- **Strings:** interpolation, `%`/`format`/`sprintf`, case/strip/`split`/
  `each_char`/`lines` and friends.
- **Regular expressions:** `/re/imx` literals, `Regexp`/`MatchData`, `=~` /
  `match` / `match?` / `scan` / `gsub` / `sub` / `split`, and the match globals
  `$~` / `$1`..`$N` / `$&` / `` $` `` / `$'` — running on the standalone pure-Go
  [go-onigmo][go-onigmo] engine, so the build stays **CGO=0**.
- **Collections:** Array / Hash / Range with `Enumerable` (map/select/reduce/…)
  and `Comparable`, both written once in embedded Ruby.

Still ahead (see the [roadmap](roadmap.md)): mutable String, pattern matching
`case/in`, Fiber, bignum.

## Repositories

| Repo | What it is |
| --- | --- |
| [`ruby`](https://github.com/go-embedded-ruby/ruby) | the interpreter — the bytecode VM, embedded front-end, and the `rbgo` CLI binary |
| [`docs`](https://github.com/go-embedded-ruby/docs) | this documentation site (MkDocs Material, versioned with mike) |
| [`go-embedded-ruby.github.io`](https://github.com/go-embedded-ruby/go-embedded-ruby.github.io) | the organization landing page (Hugo) |

The regexp engine is **not** in this org: it is a separate pure-Go Onigmo
reimplementation in the sibling org [go-onigmo][go-onigmo] (repo `regexp`).

## How it differs from goruby

There is an existing Go project also called [goruby][goruby]. go-embedded-ruby
takes a deliberately different path:

- **Bytecode VM, not an AST-walking interpreter.** Source is compiled to a YARV
  -style instruction sequence (ISeq) and executed on a stack machine, rather
  than re-walking the syntax tree on every call.
- **Single, tree-shaken static binary.** `rbgo build` selects only the reached
  stdlib and links one self-contained executable.
- **Dynamic `eval` / `require` via an embedded front-end.** The full compiler
  pipeline is in the binary, so code can be parsed and compiled at runtime.
- **Ruby 4.0 as the target.** Behaviour is checked against MRI rather than
  approximated.

## Where to go next

- [Architecture overview](architecture/index.md) — the compilation pipeline and
  the packages that make it up.
- [Roadmap (phases)](roadmap.md) — the nine phases from vertical slice to
  conformance and performance.
- [Phase 0 — Vertical slice](phases/phase0.md) — what runs today and how it is
  tested.

Source lives at
[github.com/go-embedded-ruby/ruby](https://github.com/go-embedded-ruby/ruby).

[go-mruby]: https://github.com/mitchellh/go-mruby
[go-onigmo]: https://github.com/go-onigmo
[goruby]: https://github.com/goruby/goruby
