# Build & single binary

`rbgo build` turns a Ruby program into **one static, self-contained
executable** — no Ruby installation, no C toolchain, no shared libraries. The
trick is to compile everything reachable to bytecode, embed it, and let the Go
linker drop everything else.

## How `rbgo build` works

1. **Scan the require graph.** Starting from the entry script, `rbgo` follows
   `require`s to discover exactly which standard-library files the program can
   reach.
2. **Select only the reached stdlib.** Each stdlib component is guarded by Go
   **build tags** and embedded with **`go:embed`**, so only the parts the program
   actually uses are compiled in — and the **Go linker drops the rest** as dead
   code.
3. **Compile to bytecode.** The application *and* the selected stdlib are
   compiled to ISeqs ahead of time.
4. **Embed and link.** The bytecode is embedded into a small Go program that
   bundles the VM, and a plain **`go build`** produces a single static binary.

## Closed-world mode

A program that never calls `eval` and never does a *dynamic* `require` does not
need the front-end at runtime. **Closed-world mode** takes advantage of this: it
**drops the embedded lexer/parser/compiler** from the binary, since all code was
already compiled to bytecode at build time. The result is a noticeably **smaller
binary**, at the cost of giving up runtime code loading.

The open-world default keeps the front-end embedded so `eval` and dynamic
`require` keep working (see [Front-end](frontend.md)).

## Precedent

This is not a new idea: **mruby** does essentially the same thing with
*mrbgems*, where the set of compiled-in components is selected at build time via
`build_config.rb`. go-embedded-ruby's require-graph scan plus build-tag/`go:embed`
selection is the same closed-world philosophy expressed through the Go
toolchain.

The build toolchain is developed in full in
[Phase 7](../roadmap.md).
