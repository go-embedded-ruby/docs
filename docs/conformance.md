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

## The conformance & benchmark ladder

Conformance is grown in three rungs, each a real corpus rather than a synthetic
suite:

1. **The three-way oracle** — rbgo vs MRI vs JRuby (**done**).
2. **Pure-Ruby gem test suites as corpora** — ActiveSupport `core_ext`
   (**in progress**).
3. **Real-world workloads** — OpenVox / Puppet and ultimately Rails — serving as
   both conformance corpora and performance baselines.

!!! note "On Rails and Puppet"
    Full Rails and Puppet execution depends on C-extension and threading depth
    that is **out of scope** for a pure-Go embeddable. The achievable targets are
    their **pure-Ruby subsets and unit suites**, which is what the ladder above
    aims at.

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
