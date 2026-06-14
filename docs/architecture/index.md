# Architecture overview

go-embedded-ruby is a compiler **and** a virtual machine. Ruby source is lowered
through a conventional front-end into a bytecode instruction sequence (an
*ISeq*), and that ISeq is executed by a stack VM. The whole pipeline lives in
the binary, so it is available both ahead-of-time and at runtime (for `eval` and
dynamic `require`).

## The pipeline

```text
source.rb → lexer → parser → AST → compiler → bytecode (ISeq) → VM
```

Each stage has a single responsibility:

- **lexer** turns the source text into a token stream.
- **parser** turns tokens into an abstract syntax tree (AST).
- **compiler** lowers the AST into bytecode — an ISeq.
- **VM** executes the ISeq against the object model.

## Two run modes

`rbgo` exposes the pipeline two ways:

- **`rbgo run`** runs the program *in memory*: it compiles the source to an ISeq
  and immediately interprets it on the VM. This is the fast, dynamic path —
  `eval` and runtime `require` re-enter the front-end here.
- **`rbgo build`** emits a **static binary**: it compiles the application and the
  reached standard library to bytecode, embeds that bytecode, and links one
  self-contained executable. See [Build & single binary](build.md).

## Packages

The interpreter (`github.com/go-embedded-ruby/ruby`) is organized as a chain of
small packages mirroring the pipeline stages, plus the object model the VM
operates on:

| Package | Responsibility |
| --- | --- |
| `token` | token kinds and source positions |
| `lexer` | source text → token stream |
| `ast` | abstract syntax tree node types |
| `parser` | tokens → AST |
| `compiler` | AST → bytecode (ISeq), local-slot resolution |
| `bytecode` | the ISeq representation and instruction set |
| `vm` | the stack machine that executes ISeqs |
| `object` | the value/object model the VM manipulates |

The detail pages cover the three load-bearing pieces:
[Values & object model](object-model.md),
[Bytecode & VM](bytecode-vm.md), and the
[Front-end](frontend.md).
