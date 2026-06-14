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
targets **Ruby 4.0** semantics and is grown test-first against an MRI
*differential oracle*.

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
