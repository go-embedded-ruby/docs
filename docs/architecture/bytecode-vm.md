# Bytecode & VM

The compiler lowers the AST to a **YARV-style stack instruction set**, and the
VM executes it. The design follows MRI's YARV and mruby closely so that
behaviour can be reasoned about against a known model.

## The instruction set

The ISA is a classic stack machine. The instruction families in play are:

- **stack** — `push` / `pop`
- **locals** — load and store local-variable slots
- **arithmetic** — with fast paths for the common `Integer`/`Float` cases
- **control** — conditional and unconditional `jump`s
- **calls** — `call` (method dispatch through the object model) and `return`
- **definition** — `define_method` (and class/module definition as the model
  grows)

## The ISeq

The unit of compiled code is the **instruction sequence (ISeq)**. An ISeq is not
just a flat list of opcodes; it bundles everything the VM needs to run one method
or block:

- **insns** — the instructions themselves
- **consts** — the constant pool (literals referenced by index)
- **names** — symbol/method/constant names
- **params** — parameter descriptors
- **locals** — the local-variable slot map
- **children** — nested ISeqs (for blocks and inner methods)

## Frames, control flow, and errors (Phase 0)

In Phase 0 the VM **recurses on the Go stack**: a Ruby method call is a Go call
into the interpreter loop, with an explicit **frame** capturing the receiver,
local slots, and the ISeq being run. Non-trivial control flow is handled by
**catch tables** attached to ISeqs, covering:

- `rescue` / `ensure`, and
- non-local `break` / `next` / `redo` / `return`.

**Fibers** are not on the Go stack: the plan is **Fiber-on-goroutine**, with each
Ruby `Fiber` backed by a goroutine, arriving in
[Phase 3](../roadmap.md).

!!! note "Error propagation will change"
    In Phase 0, Ruby errors travel as a Go `panic(RubyError)` and are recovered
    at frame boundaries — this is the quickest way to get `rescue`/`ensure`
    correct early. When exceptions land properly (Phase 3), error propagation
    moves to explicit **status-returns** from the interpreter loop, removing the
    panic/recover hot path.
