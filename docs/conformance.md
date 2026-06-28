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

- **100% statement coverage**, enforced as a CI gate;
- across **all six 64-bit architectures** — amd64, arm64, riscv64, loong64,
  ppc64le, s390x;
- across **three OSes**;
- with **every feature differential-tested against MRI 4.0.5**.

A feature does not count as landed until it agrees with the reference and the
lines that implement it are covered.

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
5 rbgo activation rounds), the final figures against the MRI 4.0.5 oracle:

| Repo   | `.rb` files | rbgo **parses** | rbgo **parses + compiles** |
|--------|------------:|----------------:|---------------------------:|
| Rails  |       3 423 |     **100.00 %** |                **99.82 %** |
| Puppet |       2 156 |     **100.00 %** (of valid) | **100.00 %** |
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

## Heavyweight front-end sweep: Rails & Puppet

The two largest reference Ruby codebases — **Rails** and **Puppet** — are run
through the pure-Go front-end (`parser.Parse` → `compiler.Compile`, no execution)
as a conformance stress test, with **MRI `ruby -c`** as the oracle for "is this
valid Ruby?".

| Repo   | Parse | Parse + compile |
| ------ | ----: | --------------: |
| Rails  | 100.00 % (3 423 / 3 423) | **99.82 %** (3 417 / 3 423) |
| Puppet | 100.00 % (2 154 / 2 154) | **100.00 %** (2 154 / 2 154) |

Across the corpus rbgo parses **100 % of all valid Ruby** (the only 2 misses are
intentional syntax-error fixtures MRI itself rejects), with **0 over-permissive**
(rbgo never accepts Ruby MRI rejects). See
[`CONFORMANCE-RAILS-PUPPET.md`](https://github.com/go-embedded-ruby/ruby/blob/main/CONFORMANCE-RAILS-PUPPET.md)
in the interpreter repo for the full report and the reproducible sweep.

These are **front-end (parse + compile) acceptance** figures, not whole-app
execution. Running a real application additionally needs the runtime stdlib
surface and C-extension equivalents — which is now real enough to boot one.

## Running Puppet — boots, compiles, and evaluates manifests

Beyond parsing real-world Ruby, **rbgo runs Puppet**.
`require "puppet"` **fully boots** the framework (Puppet 8.11.0) on a pure-Go
CGO=0 `rbgo` — Puppet's pure-Ruby gem dependencies (`semantic_puppet`,
`concurrent-ruby`, `facter`, `fast_gettext`, `racc`, …) load on the
`$LOAD_PATH` — and a manifest then travels the real Puppet pipeline: it **parses
to the Pops AST, compiles to a catalog, and evaluates**. A trivial manifest emits
genuine Puppet log output:

```ruby
notice("hi from puppet")
# => Notice: Scope(Class[main]): hi from puppet
```

Reaching this implemented a wide runtime surface — Ruby-conformance fixes
(`autoload`, `ERB`, frame-based `Exception#backtrace`, interpolated regexp
literals, non-local block `return`, `Module.new` / `extend` transitivity, …) plus
pure-Go stdlib modules (`openssl` with real crypto, `net/http`, `resolv`,
`StringScanner`, `fileutils`, …) — and validates the **C-extension → pure-Go
shim** strategy: a real Ruby application ships as one static CGO=0 binary because
its C-backed gem APIs are backed by pure Go. Puppet's dependency tree is pure
Ruby, so it loads as-is.

!!! note "The honest frontier"
    What works today is **boot → parse → compile → evaluate** a manifest (the Pops
    evaluator emits `Notice:` and friends). Full **`puppet apply`** — the
    transaction / RAL / resource-provider layer that mutates real host state — is
    the **active next milestone**, not done. The `notice(...)` example above is a
    real evaluation; an `apply` that converges resources against a host is in
    progress.

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
