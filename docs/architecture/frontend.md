# Front-end (lexer, parser, compiler)

The front-end turns Ruby source into an ISeq. Ruby's grammar is famously
context-sensitive, so each stage carries a little state to disambiguate. The
front-end is **the largest single effort** in the project and ships *inside* the
binary so that `eval` and runtime `require` can re-run it.

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

## Open decision: hand-written vs Prism

How far to hand-write the front-end is an **open question**. The two candidates:

- **Hand-written** lexer + parser, as described above — full control, but the
  most work, and a moving target against MRI.
- **Port Prism** (Ruby's official parser library) compiled **to WebAssembly**
  and run under a **pure-Go [wazero](https://wazero.io/) runtime** — this would
  reuse MRI's own parser while keeping the cgo-free guarantee, at the cost of an
  embedded WASM module and the wazero dependency.

The trade-off is conformance-for-free versus footprint and control; it is not
yet settled.
