# Front-end (lexer, parser, compiler)

The front-end turns Ruby source into an ISeq. Ruby's grammar is famously
context-sensitive, so each stage carries a little state to disambiguate. The
front-end is **the largest single effort** in the project and ships *inside* the
binary so that `eval` and runtime `require` can re-run it.

!!! note "The lexer, parser and AST are a standalone module"
    The lexer, parser and AST have been **extracted** into the pure-Go
    [go-ruby-parser](https://github.com/go-ruby-parser/parser) module
    (`github.com/go-ruby-parser/parser`), which this interpreter imports — the
    same dogfooding model as the [go-onigmo](https://github.com/go-onigmo)
    regexp engine. They are still compiled *into* the binary, so `eval`/`require`
    keep working; they are simply maintained, tested and reusable on their own
    (any Go program can now parse Ruby to an AST without cgo). The **compiler**
    (AST → bytecode) remains in this repository.

## Lexer

The lexer is **stateful**. The key piece of state is `SpaceBefore` — the direct
analogue of MRI's `spaceSeen` — which records whether whitespace preceded the
current token. Combined with a lexer-state seed, this is what lets Ruby
distinguish, for example, a *command call with an argument* from a *binary
expression*:

```ruby
foo -1      # command call: foo(-1)
foo - 1     # binary minus: foo() - 1
```

The same character sequence lexes differently depending on the surrounding
whitespace and state, so the lexer cannot be a pure, context-free scanner.

## Parser

The parser is **recursive-descent with a Pratt (precedence-climbing) expression
parser** for operators. It maintains a **scope stack**, which is what resolves
Ruby's central syntactic ambiguity: a bare identifier like `x` is a
**local-variable reference** if `x` has been assigned in the current scope, and
otherwise a **method call** on the implicit receiver. The scope stack tracks
which names are known locals at each point, so the parser can make that call
correctly.

## Compiler

The compiler lowers the AST to one or more ISeqs. Its main jobs are emitting the
stack instructions for each node and **resolving locals to slots** — turning
named local variables into the integer slot indices the VM loads and stores. (It
also builds the catch tables described in
[Bytecode & VM](bytecode-vm.md).)

## Decision: hand-written (extracted as a reusable module)

How far to hand-write the front-end was an early open question. It is now
**settled in favour of hand-writing**: the lexer + parser + AST are a complete,
100%-covered, MRI-differential-tested module
([go-ruby-parser](https://github.com/go-ruby-parser/parser)) that this
interpreter consumes and that any Go program can reuse. Full control won out, and
extracting it turned the "most work" cost into ecosystem value.

The alternative once considered — porting **Prism** (Ruby's official parser)
compiled to WebAssembly under a pure-Go [wazero](https://wazero.io/) runtime —
remains theoretically possible (conformance-for-free at the cost of an embedded
WASM module + the wazero dependency), but is not planned.
