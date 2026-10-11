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
Each spec file runs in its own `rbgo` process, and the runner records **one row
per spec file** — `OK<TAB>path<TAB>passing`, or `FILEFAIL<TAB>path` for a file
that produced no result — in
[`BASELINE`](https://github.com/go-embedded-ruby/ruby/blob/main/scripts/conformance/rubyspec/BASELINE):

- a file that passes **fewer** examples than its row, or stops producing a
  result, fails the run and **is named**;
- an improvement is locked in with `UPDATE_BASELINE=1`, which rewrites the
  record from that sweep.

Judging per file is what makes every move attributable, and it needs no margin.
A single threshold could not have both: its margin had to be wider than the
largest spec file — 384 examples in `core/encoding/compatible_spec.rb`, since a
file that fails to load loses its whole count at once — so a drop inside that
margin could not distinguish *one file failing to load* from *hundreds of specs
regressing*. A run that gains examples overall still fails here if any single
file drops, which a total could not see.

The shim stubs a few MSpec matchers (`complain`, `output`, `ruby_exe`, …), so
their examples count as *skipped* rather than passing — the record is a
deliberately **conservative lower bound**. The
[`rubyspec-ratchet`](https://github.com/go-embedded-ruby/ruby/blob/main/.github/workflows/rubyspec-ratchet.yml)
workflow enforces it on every push and pull request.

### Where it stands — measured 2026-09-26

!!! info "The gate's current total is **23 511** — rbgo v0.3.0, 2026-10-11"
    The table below is one **dated full sweep**, and a dated measurement going
    stale is not an error: it is what it says it is. The number that moves with
    every merge is the per-file baseline's sum, which the badge and the README
    quote, and it is read straight out of the gate's own file:

    ```console
    $ awk -F'\t' '/^OK\t/{n+=$3} END{print n}' scripts/conformance/rubyspec/BASELINE
    23511
    ```

    The breakdown below — failing, erroring, skipped — needs a full sweep to
    refresh and has not been re-run, so it keeps its own date rather than being
    mixed with a newer total.

On darwin/arm64, on `3e8e3cc`, against the pinned corpus
(`SPEC_SHA=87b1631992bd00cf0c4934474766d54dad088191`).

| | |
| --- | --- |
| **passing examples** | **22 488** |
| failing / erroring examples | 1 090 / 538 |
| skipped (a matcher the shim does not implement) | 477 |
| examples that actually ran (pass + fail + error) | 24 116 |
| share of those that passed | **93.3 %** |
| spec files | 2 196 of 2 206 produce a result; 10 produce none |
| **enforced by CI** | a **per-file** baseline, not a single number |

Three independent runs agreed exactly: `run.sh` twice, at different per-file
timeouts (25 s and 60 s) and different parallelism, and once through a separate
sweep that tallies all four counters. All three reported **22 488** passing and the
same ten files producing no result.

!!! warning "The baseline is a gate, not a score"
    `BASELINE` says *"no file may come in below its row"*. Its sum is therefore the
    level **CI guarantees**; a measured total is **what rbgo did on a given run**.
    Quoting the guaranteed figure as the conformance result understates rbgo by
    whatever has landed since the record was last written; quoting a measured total
    as a guarantee overstates it by the same amount. The sum is **23 511** as of
    2026-10-11 (rbgo v0.3.0), and the repository README reads that figure out of
    the file mechanically, with a test that fails if the two disagree.

!!! danger "What this number is not"
    It is **not** "rbgo implements 93 % of Ruby", and it cannot be turned into a
    percentage of the language:

    - **Only `language/` and `core/` are swept.** ruby/spec's `library/`,
      `optional/` and `security/` trees are **not run at all** — most of what people
      mean by "the standard library" is outside this measurement entirely.
    - **The shim is not mspec.** Some matchers are stubbed, so the examples needing
      them are counted as *skipped*, never as passing. The total is a conservative
      lower bound.
    - **Skips are outside the ratio.** The 93.3 % divides passes by the examples
      that ran; the 477 skips and the 10 unreadable files are in neither column.
    - **Examples are not weighted.** A heavily specified method contributes far more
      examples than a rarely used one, so the count does not track "features".

An earlier revision of this page reported run-to-run spread from an intermittent
crash in `core/module/autoload_spec.rb`. That did not reproduce during this
measurement: five consecutive runs of that file returned identical counts, and it
appeared in none of the three full sweeps.
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
([#263](https://github.com/go-embedded-ruby/ruby/pull/263)) to **22 475** by the
time it was replaced by the per-file baseline, whose sum is **23 511** at
v0.3.0 (2026-10-11).

### Known limitations

Each entry below was re-run on `4872e01` against **MRI 4.0.7 on the same host**
(darwin/arm64, 2026-10-11). Entries that no longer reproduced were removed — notably
the former headline limitation, *"`__LINE__` is always `0`"*: rbgo and MRI now agree
exactly on `__LINE__`, on backtrace line numbers, on `warn(uplevel:)` prefixes and on
`const_source_location`.

Re-checked on `4872e01` (2026-10-11) against **MRI 4.0.7** on the same host. Four
entries no longer reproduced and have gone:

- *"The browser WebAssembly target does not build"* — fixed by
  [#684](https://github.com/go-embedded-ruby/ruby/pull/684), which also added the
  `wasm (js, browser)` lane. `GOOS=js GOARCH=wasm go build ./...` exits 0.
- *"`$stderr` is not a separate stream"* — the VM takes two writers, and
  `rbgo -e '$stdout.print "O"; $stderr.print "E"'` now sends one byte to each.
- *"Magic encoding comments are not honoured"* — a file beginning
  `# encoding: ascii-8bit` reports `ASCII-8BIT`, as MRI does.
- *"`BEGIN { }` / `END { }` do not parse"* — both run.
  `BEGIN { puts "B" }; puts "main"; END { puts "E" }` prints `B main E`. `END`
  did something worse than not parsing in between: it killed the process with a
  Go panic, fixed in v0.3.0 via parser v0.13.1
  ([#805](https://github.com/go-embedded-ruby/ruby/issues/805)).

Two limitations matter most to anyone embedding rbgo, and both **fail open** —
the call returns successfully and nothing says the guarantee is missing:

- **`Timeout.timeout` never times out.** The module, `Timeout::Error` and the
  `rescue` path all exist; only the enforcement is absent, so the block runs to
  completion and its value is returned.
  `Timeout.timeout(0.3) { sleep 2 }` returns after 2 s where MRI raises at 0.3 s.
  It is *the* ordinary Ruby way to bound work that might hang, so code relying on
  it for that is unprotected here.
  [#803](https://github.com/go-embedded-ruby/ruby/issues/803).
- **`Kernel#fork` does not fork.** The block runs in the same process, so the
  "child" mutates the parent's globals, `ENV` and working directory, and a
  synthetic pid comes back. `fork` is the standard way to isolate untrusted work
  and to drop privileges; a program forking for either reason gets neither, and
  gets no error. `exit`/`exit!`/`abort` inside the block do now stop only the
  "child" (v0.3.0); an uncaught *exception* in it still takes the parent down.
  [#774](https://github.com/go-embedded-ruby/ruby/issues/774).

| | |
| --- | --- |
| `RUBY_VERSION` reports `"3.4.1"`, though the differential oracle is MRI 4.0.7 and the pinned corpus is 4.0.x. That is not only cosmetic: the shim's version guard is pinned to the same `3.4.1`, so **167 examples behind a `ruby_version_is "4.0"` guard are never reached**, while 40 guards that end *before* 4.0 are being asserted | [#799](https://github.com/go-embedded-ruby/ruby/issues/799) |
| `Errno` carries all **158** of MRI's constant names, but **32** report errno `0` where MRI has a real number (`EAUTH`, `EBADRPC`, `EDEVERR`, …). By set difference on darwin: 81 names are zero-valued under rbgo, 49 under MRI, and every MRI zero is also zero under rbgo | [#633](https://github.com/go-embedded-ruby/ruby/issues/633) |
| `Thread#backtrace` answers for the **current** thread only; another thread raises `NotImplementedError` | — |
| `Process.fork` **does not exist** — Go's runtime cannot be forked safely | — |
| `Numeric#to_int` is not defined — `Complex(2.9, 0).to_int` raises `NoMethodError` where MRI returns `2` | [#631](https://github.com/go-embedded-ruby/ruby/issues/631) |
| `File::Stat#==` answers identity: `a == b` is `false` for two stats of one file where MRI says `true` | [#632](https://github.com/go-embedded-ruby/ruby/issues/632) |
| `File#stat` raises `Errno::ENOENT` for a file unlinked while open; MRI answers from the descriptor | [#635](https://github.com/go-embedded-ruby/ruby/issues/635) |
| Neither `Kernel#pp` nor `require "pp"` exists (`require "prettyprint"` works) | — |
| **Windows:** no text-mode newline translation | [#610](https://github.com/go-embedded-ruby/ruby/issues/610) |
| **Windows:** `File::Stat#atime`/`#ctime` fall back to mtime | [#635](https://github.com/go-embedded-ruby/ruby/issues/635) |
| **Windows:** `File.realpath` does not expand 8.3 short names | [#636](https://github.com/go-embedded-ruby/ruby/issues/636) |

The three Windows rows were **not** re-verified: this measurement ran on darwin.

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

It emits one record per spec file and hands them to the ratchet, which names
any file that regressed and exits non-zero; `UPDATE_BASELINE=1` rewrites the
record instead. Run it **more than once**:
`core/module/autoload_spec.rb` crashes intermittently
([#615](https://github.com/go-embedded-ruby/ruby/issues/615)) and a single run can
read ~57 examples low.
