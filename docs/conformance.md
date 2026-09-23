# Conformance

go-embedded-ruby targets **Ruby 4.0** semantics. Correctness is not asserted by
reading the spec and hoping — it is *measured* against running reference
implementations, snippet by snippet, with the result checked into CI.

## Two independent reference implementations

Every behaviour is judged against **two independent reference implementations** of
Ruby 4.0, so an agreement between them is strong evidence the behaviour is the
language's and not an accident of one VM:

- **MRI (CRuby) 4.0.5** — the canonical C interpreter;
- **JRuby 10.1** — the JVM implementation.

A subset of **ruby/spec** is run on top of this to cover behaviours the corpora
do not yet reach.

!!! note "TruffleRuby being added"
    A third reference — **TruffleRuby** (the GraalVM implementation) — is being
    added to both the conformance oracle and the performance baselines
    ([interpreter PR #1](https://github.com/go-embedded-ruby/ruby/pull/1)). The
    oracle and corpora below describe the rbgo / MRI / JRuby comparison that runs
    on `main` today.

## The three-way differential oracle

The tool that establishes correctness is a **three-way differential oracle**,
[`scripts/oracle.sh`](https://github.com/go-embedded-ruby/ruby/blob/main/scripts/oracle.sh)
in the interpreter repo. It runs the same snippet through **rbgo**, **MRI** and
**JRuby**, compares their output, and flags any divergence. It **exits non-zero on
a mismatch**, so it drops straight into CI and into a pre-commit loop.

It has two modes.

A single expression, evaluated in all three and diffed:

```sh
scripts/oracle.sh -e 'p (1..10).select(&:even?).map { |x| x**2 }'
```

A batch corpus — a file with one snippet per line, every line run through all
three implementations, with a summary of how many agree (`N/total`):

```sh
scripts/oracle.sh -b scripts/conformance/core_ext.txt
```

## What CI enforces

On top of the oracle, the interpreter holds a hard CI gate:

- **100% statement coverage**, gated on the two **POSIX** lanes — ubuntu and
  macOS. The full `-race` suite also runs on **windows**, but the gate is not
  applied there: a handful of POSIX-only paths (opening `/dev/null` as a
  character device) are unreachable, so its measured coverage is structurally
  below 100 %;
- **per-PR architecture cover is the two native lanes**, `amd64` and `arm64`,
  which run `go test ./...` — not the coverage gate. The four exotic 64-bit
  targets (riscv64, loong64, ppc64le, s390x) are validated **off** the per-PR
  critical path: nightly under QEMU and on scheduled real hardware (below);
- a **`wasip1/wasm`** lane builds the interpreter and runs it under wazero;
- with **every feature differential-tested against MRI 4.0.5**.

[`ci.yml`](https://github.com/go-embedded-ruby/ruby/blob/main/.github/workflows/ci.yml)
is the authority on which lane runs what.

A feature does not count as landed until it agrees with the reference and the
lines that implement it are covered.

### Real hardware, not just qemu

qemu is the CI gate; **real silicon is the correctness-and-performance oracle**,
so every one of the six arches is validated on real hardware:

- **amd64 / arm64** — natively (an x86_64 VM with AVX2; Apple-Silicon arm64);
- **riscv64 (RVV)** — the GCC Compile Farm node cfarm95 (a SiFive-class RVV host),
  for vector / SIMD validation;
- **ppc64le** — cfarm112 (POWER8E) and cfarm433 (POWER9); POWER8 is the ISA floor
  the VSX paths are gated on;
- **loong64** — cfarm401 (LoongArch; air-gapped, artifacts staged in);
- **s390x** — the **IBM LinuxONE Community Cloud (L1CC)** (big-endian, vector
  facility), since no cfarm node exists for it.

This real-hardware access is what turns the **SIMD-accelerated modules**
(`base64` / `securerandom` / `hex`, via go-simd) and the
[performance benchmarks](benchmarks.md) into measured numbers rather than
llvm-mca estimates.

## The ruby/spec ratchet

On top of the oracle, rbgo runs the `language/` and `core/` suites of
[ruby/spec](https://github.com/ruby/spec) — the **executable specification** of
the language and core library — through `rbgo` under a minimal MSpec-compatible
shim
([`scripts/conformance/rubyspec/`](https://github.com/go-embedded-ruby/ruby/tree/main/scripts/conformance/rubyspec)).
Each spec file runs in its own `rbgo` process; the runner sums the passing
examples and compares the total against a **frozen floor** (`FLOOR`):

- a total **below** the floor fails the run (a conformance regression);
- an improvement raises the floor **in the same PR** (`UPDATE_FLOOR=1`).

Because the floor can only be raised, measured language conformance moves in one
direction only. The shim stubs a few MSpec matchers (`complain`, `output`,
`ruby_exe`, …), so their examples count as *skipped* rather than passing — the
floor is a deliberately **conservative lower bound**. The
[`rubyspec-ratchet`](https://github.com/go-embedded-ruby/ruby/blob/main/.github/workflows/rubyspec-ratchet.yml)
workflow enforces it on every push and pull request.

### Where it stands — measured 2026-09-23

On `main` (68cb53a), darwin/arm64, against the pinned corpus
(`SPEC_SHA=87b1631992bd00cf0c4934474766d54dad088191`):

| | |
| --- | --- |
| **passing examples** | **21 901** — four consecutive runs: 21 845 · 21 903 · 21 901 · 21 902 |
| fail / error | 1 349 / 794 |
| skipped | 473 |
| pass rate of the examples that ran | **91.1 %** (21 901 of 24 044) |
| spec files | 2 191 of 2 206 produce a result; 15 produce none |
| **`FLOOR`** | **21 740** |

!!! warning "The floor is a gate, not a score"
    `FLOOR` says *"no run may come in below this"*. It is raised deliberately, in
    its own PR, after a wave lands — so the measured total normally sits **above**
    it. Today the gap is ~160 examples. Quoting the floor as the conformance
    figure understates rbgo; quoting the measured total as a guarantee overstates
    it. The floor is what CI enforces; the measured total is what rbgo does.

The 58-example spread across those four runs is **one file**.
`core/module/autoload_spec.rb` crashes on roughly one run in four while popping a
frame ([#615](https://github.com/go-embedded-ruby/ruby/issues/615)), and the whole
file's examples are lost when it does. Read the **low** run as the guaranteed
figure.

There is no honest denominator for "percent of Ruby". The 91.1 % above is the
share of the examples *this shim actually ran*: the shim is not mspec, 473 skips
sit outside the ratio entirely, and 15 files produce no result at all.

### Not every gain is VM conformance

Two of the recent jumps came from fixing the **measurement**, not the
interpreter, and blurring the two would be misleading:

- **The mspec shim** ([#624](https://github.com/go-embedded-ruby/ruby/pull/624),
  refs [#621](https://github.com/go-embedded-ruby/ruby/issues/621)) ran each
  example under `instance_eval`, so a helper defined with `def` in an outer
  `describe` was unreachable from a nested one.
  `core/string/valid_encoding/utf_8_spec.rb` scored **0 of 28 — and 0 of 28 under
  MRI 4.0.5 as well**, run through the same shim. Fixing the shim moved the
  corpus 20 905 → 20 936, of which **28 are attributable**.
- **The parser upgrade to v0.2.0**
  ([#625](https://github.com/go-embedded-ruby/ruby/pull/625)) made **13
  `language/*_spec.rb` files parseable that had never parsed at all**. They had
  been contributing zero to *both* columns — not failing, invisible. `language/`
  went 1 443 → 1 776 passing (**+333**), while `fail+error` rose 283 → 436 at the
  same time, because those files brought their own failures with them.

Both are gains in what the measurement can **see**, not in what the VM can
**do**. The rest of the climb is the VM: the floor went from **6 000** when the
ratchet landed on 2026-08-03
([#263](https://github.com/go-embedded-ruby/ruby/pull/263)) to **21 740** today,
across 26 conformance waves.

### Known limitations

Named plainly, each with an open issue. **`__LINE__` is always `0`** — rbgo
records no line map — which costs `const_source_location`, `warn(uplevel:)` and
every backtrace line number entirely:

```ruby
puts __LINE__                                 # rbgo: 0     MRI 4.0.5: 1
Object.const_source_location(:Comparable)     # rbgo: nil   MRI 4.0.5: []
# e.backtrace.first -> "probe.rb:0:in '<main>'" against MRI's "probe.rb:4:…"
```

| | |
| --- | --- |
| `Errno` has **28** constants, not MRI's 158 | [#633](https://github.com/go-embedded-ruby/ruby/issues/633) |
| `FileTest` carries **11** of MRI's 26 predicates | — |
| `Thread#backtrace` answers for the **current** thread only; another thread raises `NotImplementedError` | — |
| `Process.fork` **does not exist** — Go's runtime cannot be forked safely | — |
| `Numeric#to_int` is not defined | [#631](https://github.com/go-embedded-ruby/ruby/issues/631) |
| `File::Stat#==` answers identity, not `Comparable#==` | [#632](https://github.com/go-embedded-ruby/ruby/issues/632) |
| `File#stat` cannot answer for a file unlinked while open | [#635](https://github.com/go-embedded-ruby/ruby/issues/635) |
| `BEGIN { }` / `END { }` do not parse | front-end |
| **Windows:** no text-mode newline translation | [#610](https://github.com/go-embedded-ruby/ruby/issues/610) |
| **Windows:** `File::Stat#atime`/`#ctime` fall back to mtime | [#635](https://github.com/go-embedded-ruby/ruby/issues/635) |
| **Windows:** `File.realpath` does not expand 8.3 short names | [#636](https://github.com/go-embedded-ruby/ruby/issues/636) |

Two more are **harness**, not interpreter, and are listed apart so they are not
read as VM gaps: the shim's `SPEC_TMP_BASE` is `/tmp`, a symlink on darwin, which
fails 11 `realpath`/`realdirpath` examples on paths that are correct
([#630](https://github.com/go-embedded-ruby/ruby/issues/630)) — **MRI 4.0.5 fails
them the same way through the same shim** — and the `instance_eval` rebinding
above ([#621](https://github.com/go-embedded-ruby/ruby/issues/621)), largely fixed
but not closed.

## Real-world corpora

Synthetic tests prove a feature works in isolation; they do not prove the idioms
real Ruby code actually uses are supported. So beyond synthetic tests, **idioms and
suites from reference applications drive the work by demand**:

- **Rails' ActiveSupport `core_ext`** — the pure-Ruby String / Array / Hash /
  Numeric / Enumerable extensions;
- **OpenVox** / Puppet — Ruby-heavy manifest evaluation.

These surface the gaps that matter. A representative ActiveSupport `core_ext`
idiom sweep started at **9/15** agreeing across the three implementations; as the
gaps it exposed were fixed, it reached **15/15**. The committed corpus
`scripts/conformance/core_ext.txt` currently runs **20/20** in agreement across
rbgo, MRI and JRuby.

## Heavyweight front-end conformance — Rails & Puppet

The two largest reference Ruby codebases double as a front-end stress test. The
metric is **front-end (parse + compile) acceptance**: for every `.rb` file, does
rbgo's `parser.Parse` (and then `compiler.Compile`) accept the same source MRI
considers valid? A released gem / framework file is valid Ruby (`ruby -c` clean),
so a rejection is a genuine front-end gap.

After the 2026-06 conformance campaign (5 go-ruby-parser rounds interleaved with
5 rbgo activation rounds), the final figures against the MRI 4.0.5 oracle.
**Measured 2026-06-27 against clones of `rails/rails` and `puppetlabs/puppet`
taken on 2026-06-25; not re-measured since** — reproduce with
`scripts/conformance/heavyweight/sweep.sh`:

| Repo   | `.rb` files | rbgo **parses** | rbgo **parses + compiles** |
|--------|------------:|----------------:|---------------------------:|
| Rails  |       3 423 |     **100.00 %** |                **99.82 %** |
| Puppet |       2 156 |     **100.00 %** (of valid: 2 154) | **100.00 %** (2 154 / 2 154) |
| **Total** |    5 579 |  **99.96 %** |                **99.89 %** |

- **Parse: 99.96 %** of the combined corpus (5577 / 5579). The only 2 misses are
  intentional syntax-error fixtures **MRI itself rejects** — so rbgo parses
  **100 % of all valid Ruby** in the corpus.
- **Parse + compile: Rails 99.82 %** (3417 / 3423), **Puppet 100.00 %**
  (2154 / 2154).
- **0 over-permissive** — rbgo never accepted Ruby that MRI rejects.

The journey for Rails end-to-end (parse + compile): **20.7 % → 46.3 % → 68.7 % →
81.2 % → 86.7 % → 93.8 % → 98.4 % → 99.82 %**. The biggest levers were `::`
constant-path parsing, paren-less command calls with args/kwargs, `class << self`,
and argument forwarding.

Popular libraries parse just as well — **RuboCop 99.7 %**; **Sinatra, Jekyll,
Thor, Kramdown, dry-struct 100 %**; Homebrew 98.7 %; Chef 99.1 %; concurrent-ruby
98.3 %; Asciidoctor 93.8 % — and **RSpec DSL usage is 10/10** byte-identical to
MRI.

!!! warning "Front-end acceptance is not whole-application execution"
    "Rails 99.82 %" means rbgo's front-end **parses and compiles** that fraction
    of Rails's `.rb` files — **not** that rbgo **runs** Rails. Running a full Rails
    or Puppet application additionally needs the runtime stdlib surface and
    C-extension equivalents, which is **ongoing and unproven**. What these numbers
    establish is that the **Ruby language / front-end is essentially complete** on
    real-world code; whether any *given application boots* end-to-end is separate,
    future work. Full reports:
    [`CONFORMANCE-RAILS-PUPPET.md`](https://github.com/go-embedded-ruby/ruby/blob/main/CONFORMANCE-RAILS-PUPPET.md)
    and
    [`CONFORMANCE-LIBRARIES.md`](https://github.com/go-embedded-ruby/ruby/blob/main/CONFORMANCE-LIBRARIES.md).

## The conformance & benchmark ladder

Conformance is grown in three rungs, each a real corpus rather than a synthetic
suite:

1. **The three-way oracle** — rbgo vs MRI vs JRuby (**done**).
2. **Pure-Ruby gem test suites as corpora** — ActiveSupport `core_ext`
   (**done**, 20/20).
3. **Real-world workloads** — front-end (parse + compile) acceptance on Rails,
   Puppet and popular libraries (**done**: ~100 % of valid Ruby parses); running
   whole applications on the runtime stdlib + C-extension surface is the next,
   ongoing rung.


These are **front-end (parse + compile) acceptance** figures, not whole-app
execution. Running a real application additionally needs the runtime stdlib
surface and C-extension equivalents — which is now real enough to boot one.

## Running Puppet — `puppet apply` runs end-to-end

Beyond parsing real-world Ruby, **rbgo runs the real `puppet apply` CLI
end-to-end**. `require "puppet"` **fully boots** the framework (Puppet 8.11.0) on
a pure-Go CGO=0 `rbgo` — Puppet's pure-Ruby gem dependencies (`semantic_puppet`,
`concurrent-ruby`, `facter`, `fast_gettext`, `racc`, …) load on the
`$LOAD_PATH` — and a manifest then travels the **complete** Puppet path: the
genuine `Puppet::Util::CommandLine` → `Puppet::Application::Apply` entry point
(real `OptionParser`), all Puppet types + providers loaded, the settings catalog
applied (creating Puppet's config dirs on disk), then the user catalog applied
through the transaction / RAL. The real CLI emits genuine Puppet output and exits
`0`:

```
$ rbgo run puppet_apply.rb   # ARGV = apply -e 'notify { "hello": message => "hi from rbgo cli" }'
Notice: Compiled catalog for  in environment production in 0.00 seconds
Notice: hi from rbgo cli
Notice: /Stage[main]/Main/Notify[hello]/message: defined 'message' as 'hi from rbgo cli'
```

That is the actual `notify` resource type applying through the transaction and the
Resource Abstraction Layer — not a `notice(...)` evaluator print. Reaching the CLI
implemented a wide runtime surface — a real `OptionParser` (`optparse`),
`File::Stat` / `FileTest` + on-disk filesystem operations, plus a batch of deep
Ruby fixes (`class_eval` lexical scope, `return` inside `define_method`,
class-method `super`, `String#chomp(sep)`, …) on top of the earlier boot work
(`autoload`, `ERB`, `openssl` with real crypto, `net/http`, `StringScanner`,
`fileutils`, …). It validates the **C-extension → pure-Go shim** strategy: a real
Ruby application ships as one static CGO=0 binary because its C-backed gem APIs are
backed by pure Go; Puppet's dependency tree is pure Ruby, so it loads as-is.

Three resource types now converge end-to-end through the transaction / RAL:
**`notify`**, **`file`** (the catalog manages a real file on disk), and
**`exec`** — which runs its command via pure-Go process execution, honouring the
`onlyif` / `unless` / `creates` / `path` guards, so an already-converged `exec`
is correctly skipped. `puppet apply` exits cleanly, the YAML run report / state
round-tripping through the pure-Go Psych emitter / loader.

!!! note "The honest frontier"
    What converges end-to-end is the `puppet apply` CLI applying **`notify`**,
    **`file`** and **`exec`** resources through the full pipeline (boot → parse →
    compile → transaction / RAL → output → clean exit). The next frontier is the
    **broader resource providers**: `user` / `group` are provider-ready, while
    `package` / `service` need a host package manager / systemd and root. The
    pipeline and the first convergent providers are proven; the remaining
    system-state providers are the active next milestone, not done.

## Run it yourself

From a checkout of the interpreter repo, with MRI 4.0.5 and JRuby 10.1 on
`PATH`:

```sh
# One expression, diffed across rbgo / MRI / JRuby:
scripts/oracle.sh -e 'p (1..10).select(&:even?).map { |x| x**2 }'

# A whole corpus (one snippet per line), with an N/total summary:
scripts/oracle.sh -b scripts/conformance/core_ext.txt
```

Either invocation exits non-zero if any implementation disagrees.

The ruby/spec ratchet, against a corpus you already have (it clones one at
`SPEC_SHA` otherwise):

```sh
CGO_ENABLED=0 SPECDIR=/path/to/ruby-spec CACHE=/path/to/ruby-spec \
  scripts/conformance/rubyspec/run.sh
```

It prints the measured total and the frozen floor. Run it **more than once**:
`core/module/autoload_spec.rb` crashes intermittently
([#615](https://github.com/go-embedded-ruby/ruby/issues/615)) and a single run can
read ~57 examples low.
