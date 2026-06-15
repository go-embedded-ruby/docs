# Roadmap (phases)

go-embedded-ruby is grown **test-first**, one vertical capability at a time. Each
phase ends end-to-end runnable and differential-tested against MRI, rather than
building whole subsystems in isolation. There are nine phases, 0 through 8.

| Phase | Name | Goal | Status |
| --- | --- | --- | --- |
| 0 | Vertical slice | Integers, floats, strings, locals, arithmetic, control flow, `def`/recursion, `puts`/`p` — a thin slice running end to end on the VM. | **Done** |
| 1 | Object model & dispatch | `RObject`/`RClass`, per-class method tables, ancestor-chain lookup, `method_missing`, classes + inheritance, `@ivars`, `new`/`initialize`, constants, **modules + `include`**, **`super`**, **blocks & `yield`** (closures, `block_given?`, `Integer#times`). (Remaining: `Proc`/`lambda`, `do…end`.) | **Done (core)** |
| 2 | Core types & mixins | Arrays, hashes, symbols, ranges; `Comparable`/`Enumerable` and module inclusion. **Symbols, Array, ordered Hash, and Range done; `method_missing` now gets a Symbol, matching MRI.** | **In progress** |
| 3 | Control flow, exceptions & Fiber | Real exception objects and status-return propagation; blocks/procs; Fiber-on-goroutine. | Planned |
| 4 | Metaprogramming | `define_method`, `send`, singleton classes, reflection, monkey-patching surfaced as API. | Planned |
| 5 | Full front-end | Complete the lexer/parser/compiler to full Ruby 4.0 syntax (or settle the Prism-on-wazero decision). | Planned |
| 6 | Standard library | Pure-Go stdlib coverage, with the regexp engine from the `go-onigmo` sibling org. | Planned |
| 7 | Build toolchain | `rbgo build`: require-graph scan, build-tag/`go:embed` stdlib selection, closed-world mode. | Planned |
| 8 | Conformance & performance | ruby/spec conformance and representation/perf work (tagging, fast paths, allocation). | Planned |

See [Phase 0 — Vertical slice](phases/phase0.md) for what runs today.
