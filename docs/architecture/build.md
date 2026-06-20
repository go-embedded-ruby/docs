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

## Ahead-of-time method compilation (landed)

`rbgo build` already does more than embed bytecode: it **compiles the program's
lowerable methods to native Go** and links them in, so dispatch runs machine
code instead of the bytecode loop. Each method's bytecode is lowered to a Go
function (straight-line control flow, locals as Go variables, a direct call for
self-recursion); a method using something the compiler cannot lower yet simply
stays interpreted.

A method that is **pure integer arithmetic** on integer parameters (with
self-recursion) is specialised further, to an **unboxed `int64` kernel**: the
boxed entry guards the arguments are integers and runs the kernel, and a
**deopt** edge recovers any signed overflow or divide-by-zero by re-running the
sound interpreted body — so the fast path stays correct for *every* input
(overflow still promotes to the identical Bignum). Because the whole program is
visible at build time and the Go compiler (plus PGO) does the backend, the
generated `fib(30)` runs **~4× faster than MRI + YJIT** — an ahead-of-time
compiler with an unlimited budget beats a runtime JIT on compute-bound code, and
falls back to the interpreter for the genuinely dynamic parts. Design notes:
[`docs/aot-compiler.md`](https://github.com/go-embedded-ruby/ruby/blob/main/docs/aot-compiler.md).

Still ahead (the *single-binary* half of `rbgo build`): the require-graph scan,
build-tag/`go:embed` stdlib selection and closed-world mode described above.

## Precedent

This is not a new idea: **mruby** does essentially the same thing with
*mrbgems*, where the set of compiled-in components is selected at build time via
`build_config.rb`. go-embedded-ruby's require-graph scan plus build-tag/`go:embed`
selection is the same closed-world philosophy expressed through the Go
toolchain.

The build toolchain is developed in full in
[Phase 7](../roadmap.md).
