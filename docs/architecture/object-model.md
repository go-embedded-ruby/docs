# Values & object model

Everything in Ruby is an object, and in go-embedded-ruby every Ruby object is a
Go value. The `object` package defines a single `Value` interface implemented by
concrete Go types. Because these are ordinary Go heap allocations, **Go's
garbage collector manages Ruby's heap** — there is no separate collector to
write.

## The `Value` interface

`Value` is an interface; each Ruby type is a concrete implementation. In Phase 0
the set is deliberately small:

- `Integer` — a 64-bit integer
- `Float` — a 64-bit float
- `String`
- `Bool` — `true` / `false`
- `Nil`

The user-defined object types arrive later: `RObject` (instances), `RClass`
(classes), and `RModule` (modules) land as the object model fills out in
[Phase 1](../roadmap.md).

## Dynamic dispatch — the project's `objc_msgSend`

Method calls are resolved at runtime through **mutable, per-class method
tables**. Every class carries a table mapping method names to their compiled
ISeqs (or built-in implementations). A call:

1. starts at the receiver's class,
2. **walks the ancestor chain** (superclasses and included modules) looking for
   the method, and
3. if nothing matches, **falls back to `method_missing`**.

This one mechanism is the analogue of Objective-C's `objc_msgSend`: because the
tables are *mutable* and lookup is *dynamic*, a whole family of Ruby features
falls out for free —

- **monkey-patching** (reopening a class mutates its table),
- **`define_method`** (insert an entry at runtime),
- **singleton classes** (a per-object table inserted into the chain), and
- **reflection** (the tables *are* the metadata).

## Numbers, tags, and a real tension with Go's GC

Ruby's integers are conceptually unbounded. The plan is the usual MRI-style
**`Fixnum`/`Bignum` split**: small integers stay machine-width and promote to a
big-integer representation on overflow. (Phase 0 uses plain `int64` and does
*not* promote yet — see [Phase 0](../phases/phase0.md).)

There is a genuine design tension here. The fastest representation for small
integers is an **immediate / tagged value** — stuffing the integer into the
pointer-sized slot with a tag bit, as MRI does. But tagged non-pointer values
**fight Go's garbage collector**, which expects interface slots to hold real
pointers it can scan. The Phase 0 choice is therefore the safe one: **interface
boxing** (each value is a real Go object behind the `Value` interface).
Performance work on representation — including whether any form of tagging is
worth the complexity — is deferred to [Phase 8](../roadmap.md).
