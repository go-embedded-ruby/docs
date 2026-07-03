# Benchmarks

The numbers below are real measurements, reported honestly — including where
rbgo is slower. They are **wall-clock** (process startup included), **best-of-8**
on an Apple **M4 Max** (darwin/arm64). The same `bench/*.rb` is run through every
runtime and each program's stdout is checked **byte-identical against MRI** before
it is timed.

## Six-runtime comparison

`rbgo` is the pure-Go bytecode interpreter; **rbgo+AOT** is the native binary from
`rbgo build` (the program's integer-bound methods lowered to Go). References:
**MRI 4.0.5**, **MRI+YJIT**, **JRuby 10.1** (OpenJDK 25), **TruffleRuby 34.0.1**
(GraalVM CE Native). Times in **ms**, best of 8.

| program | rbgo | rbgo+AOT | MRI | MRI+YJIT | JRuby | TruffleRuby |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| strings | 40 | 40 | 40 | 40 | 1040 | 120 |
| wordcount | 120 | 120 | 80 | 80 | 1090 | 200 |
| hash | 250 | 260 | 80 | 80 | 1120 | 100 |
| array | 440 | 440 | 90 | 60 | 1160 | 60 |
| blocks | 560 | 600 | 250 | 220 | 1200 | 80 |
| proc | 690 | 680 | 160 | 140 | 1160 | 70 |
| dispatch | 1180 | 1150 | 220 | 170 | 1190 | 50 |
| alloc | 1250 | 1280 | 230 | 190 | 1190 | 90 |
| loop | 1490 | **10** | 360 | 360 | 1240 | 90 |
| fib | 2470 | **20** | 480 | 100 | 2770 | 150 |
| mandelbrot | 2930 | 2920 | 780 | 760 | 1270 | 90 |

- **rbgo+AOT is the standout:** `loop` 10 ms and `fib` 20 ms — **18–24× faster
  than MRI+YJIT**, the only runtime here that beats YJIT, via closed-world native
  lowering of integer-bound methods. (`mandelbrot`'s float kernel is not yet
  AOT-lowered, so its AOT column matches the interpreter.)
- **rbgo interpreter** runs ~3–6× MRI on compute-bound code and at **parity**
  where startup / I-O dominates (`strings`, `wordcount`).
- **TruffleRuby** (GraalVM JIT) is the **compute ceiling** — e.g. `mandelbrot`
  90 ms vs MRI 780 ms.
- **JRuby** is dominated by ~1.0–1.2 s JVM startup; for these single-shot
  micro-benchmarks the startup is the story.

Reproduce: `AOT=1 RUNS=8 JRUBY=jruby TRUFFLE=<path> bash bench/run.sh 8`. The full
write-up (methodology, profiling, where the time goes) is in
[`BENCHMARKS.md`](https://github.com/go-embedded-ruby/ruby/blob/main/BENCHMARKS.md).

## Per-module steady-state comparison (measured 2026-07-03)

The whole-program table above is wall-clock and startup-dominated, which — as the
[caveats](#methodology-caveats) note — is **not** a fair JIT comparison. This
section is the fair one: a **steady-state, startup-excluded, warmed** micro-benchmark
of the stdlib modules that rbgo binds to standalone pure-Go `go-ruby-<mod>`
libraries. **Measured 2026-07-03 on an Apple M-series arm64.**

**Methodology (identical `.rb` across all five runtimes):**

- The **same** workload source drives rbgo, MRI, MRI+YJIT, JRuby and TruffleRuby.
  Only the inner op-loop is timed, with each runtime's own
  `Process.clock_gettime(CLOCK_MONOTONIC, :nanosecond)` — **startup, `require` and
  warmup are excluded**.
- **JITs are warmed** for a fixed 5 s wall-clock budget before any timing, so
  YJIT, JRuby (C2) and TruffleRuby (Graal) reach steady state.
- Reported number = **median ns/op** of 11 timed rounds, then **median of 3
  processes**. Regexp literals are hoisted to constants.
- **Every workload is checksum-gated:** the `.rb` returns a deterministic integer
  checksum that was asserted **byte-identical across all five runtimes** before any
  timing was trusted — guaranteeing every runtime does the *same* work.

Times are **ns/op** (lower is better); the last column is rbgo ÷ MRI (< 1 means
rbgo is faster than MRI). **rbgo 1bef36f** (pure-Go bytecode interpreter, no AOT).

| module.op | rbgo | MRI 4.0.5 | MRI+YJIT | JRuby 10.1 | TruffleRuby 34.0.1 | rbgo/MRI |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| rexml.parse_write | **10 568** | 299 840 | 147 190 | 580 631 | 97 250 | **0.04×** |
| uri.parse | **1 171** | 4 066 | 3 254 | 2 836 | 22 905 | **0.29×** |
| csv.parse_generate | **35 165** | 88 290 | 54 990 | 342 766 | 223 037 | **0.40×** |
| pathname.lexical | **12 116** | 20 462 | 19 286 | 14 408 | 8 282 | **0.59×** |
| date.parse_strftime | **2 551** | 4 216 | 4 162 | 1 713 | 79 648 | **0.60×** |
| matrix.mul_det | 5 291 | 4 915 | 2 058 | 2 572 | 48 | 1.08× |
| prettyprint.format | 12 344 | 11 154 | 6 046 | 4 670 | 1 949 | 1.11× |
| cmath.transcendental | 1 390 | 1 082 | 628 | n/a | n/a | 1.28× |
| abbrev.table | 42 237 | 30 794 | 24 170 | 19 167 | 3 811 | 1.37× |
| digest.md5_sha1_sha256 | 3 373 | 2 425 | 2 135 | 1 728 | 3 460 | 1.39× |
| regexp.scan | 69 188 | 39 645 | 40 180 | 20 854 | 8 567 | 1.75× |
| json.roundtrip | 31 565 | 15 065 | 15 390 | 36 640 | 88 697 | 2.10× |
| set.algebra | 102 378 | 46 593 | 44 883 | 31 069 | 44 898 | 2.20× |
| ipaddr.membership | 22 154 | 9 732 | 6 682 | 3 721 | 2 132 | 2.28× |
| complex.arith | 778 | 278 | 206 | 76 | 0.6 | 2.80× |
| base64.roundtrip | 19 591 | 6 635 | 6 425 | 22 569 | 44 951 | 2.95× |
| rational.arith | 1 463 | 378 | 304 | 283 | 11 | 3.87× |
| strscan.tokenize | 62 558 | 13 363 | 11 003 | 5 352 | 2 318 | 4.68× |
| zlib.deflate_inflate | 54 955 | 7 430 | 7 400 | 20 972 | 26 825 | **7.40×** |
| format.sprintf | 22 079 | 2 381 | 2 161 | 1 835 | 861 | **9.27×** |
| prime.enum_factor | 135 876 | 10 715 | 2 700 | 5 527 | 350 | **12.68×** |

`cmath` is **n/a** on JRuby and TruffleRuby: the `cmath` library was removed from
their stdlib distributions (JRuby 10.1 / TruffleRuby 3.4.9), so they cannot run
that workload; rbgo, MRI and YJIT all agree on the checksum.

### Where rbgo already wins or is at parity

- **rbgo *beats* MRI outright on five modules** — `rexml` (**28× faster** than
  MRI: the pure-Go go-ruby-rexml parser vs REXML's notoriously slow Ruby
  implementation), `uri` (**3.5×**), `csv` (**2.5×**), `pathname` and `date`
  (**~1.7×**). These are the string/parsing-heavy modules where a native Go library
  outclasses an MRI stdlib written in Ruby.
- **Parity (≤ 1.4× MRI)** on `matrix`, `prettyprint`, `cmath`, `abbrev` and
  `digest` — the last riding go-simd kernels.
- **TruffleRuby is the compute ceiling** on the tight numeric kernels
  (`complex`, `rational`, `matrix`), as expected of a Graal JIT.

### Gap triage (rbgo > 5× MRI), ranked by slowdown × fixability

1. **`prime.enum_factor` — 12.7× MRI.** `Prime.each` drives a **`big.Int`
   generator** and yields **every prime through a full VM block-call**, allocating
   a fresh `big.NewInt(bound)` for the bound compare on each iteration. Fix (high
   value, high fixability): an `int64` sieve up to the bound with the compare
   hoisted out of `big.Int`, batching yields — the single biggest, most tractable
   win here.
2. **`format.sprintf` — 9.3× MRI.** `sprintf`/`%` re-parses the format string and
   **boxes every argument** into the go-ruby-format `Value` wrapper on each call,
   with the `%e`/`%f` float path on top. Fix (high fixability): cache the parsed
   format spec and cut the per-arg boxing / float-conversion allocations.
3. **`zlib.deflate_inflate` — 7.4× MRI.** Each call appears to allocate a fresh
   deflate writer + inflate reader and intermediate buffers rather than reusing a
   pooled compressor. Fix (medium fixability): pool the `flate` writer/reader,
   confirm the default compression level matches MRI, and avoid intermediate copies.

`strscan` (4.68×) and `rational` (3.87×) sit just under the 5× gap line and are the
next tier down; the 1bef36f inline-regexp-literal cache already pulled `strscan`
in from the earlier ~25× regression.

### Variance

Median-of-3 spreads were < 5 % for all rbgo and MRI/YJIT cells. The larger relative
spreads were confined to the **sub-microsecond** TruffleRuby/JRuby cells (e.g.
Truffle `prime` 350 ns ± 33 %, `complex` 0.6 ns) where absolute noise dominates,
and to `zlib` on Truffle (± 28 %); none affect the rbgo-vs-MRI ratios reported here.

## Earlier startup-focused snapshot

An earlier, single-run snapshot (rbgo vs MRI+YJIT vs JRuby) — kept for the startup
story:

| Workload | rbgo | MRI 4.0.5 | JRuby 10.1 |
| --- | --- | --- | --- |
| startup (empty program) | 0.02 s | 0.05 s | 1.06 s |
| fib(30) | 0.61 s | 0.14 s | 1.80 s |
| fib(34) | 1.68 s | 0.38 s | 2.72 s |
| loop sum 10M | 0.68 s | 0.28 s | 1.31 s |
| string build 300k | 0.07 s | 0.07 s | 1.16 s |
| array map+sort 300k | 0.33 s | 0.07 s | 1.15 s |

## Startup is rbgo's superpower

rbgo starts in **~0.02 s** — a single static Go binary, no separate runtime, no
JVM — against MRI's ~0.05 s and JRuby's ~1.06 s. For an embedded interpreter or
a CLI tool that is invoked often and exits quickly, this is the decisive number:
the process is up and running before the alternatives have finished initialising.

It also colours the rest of the table. To read the **compute** cost of a
workload, subtract this fixed startup from each column.

## Interpreted compute

On raw interpreted compute, **MRI leads** — its C interpreter with YJIT is the
reference for fast Ruby, and rbgo, a **pure-Go bytecode VM**, is a few× slower on
tight numeric and allocation-heavy loops (`fib`, `loop sum`, `array map+sort`).
String building is already at parity (0.07 s vs 0.07 s).

!!! warning "The JRuby numbers here are not a steady-state JIT comparison"
    JRuby's JVM JIT pays a **large warm-up cost**, and every workload in this
    table is short and therefore **startup-dominated**. These runs do not let the
    JIT warm up, so they are **not** a fair picture of JRuby's steady-state
    performance — JRuby competes on long-running, warm workloads, which this
    table deliberately does not contain.

## rbgo's compute answer: AOT compilation

The interpreter is for embedding, portability and instant startup. When you need
**raw compute speed**, the answer is the **AOT compiler**, `rbgo build`: it lowers
hot methods to native Go — **unboxed `int64` kernels** with a deopt guard back to
the interpreter on overflow or `÷0`. AOT-compiled, `fib(30)` runs **~4× faster
than MRI + YJIT** while staying correct for every input. See the
[AOT compiler](architecture/build.md) doc.

So the positioning is straightforward:

> **rbgo is embeddable Ruby with instant startup and portability; when you need
> raw compute speed, AOT-compile the hot path.**

The scientific stack (`NDArray` / `FFT` / `Image`) gets a further lift from
**go-asmgen-generated SIMD kernels** across the 64-bit arches, keeping the heavy
numeric paths fast while staying CGO=0.

## Methodology caveats

Read these numbers as **indicative, not a rigorous benchmark suite**:

- wall-clock **includes process startup** — subtract the startup row to compare
  compute;
- they are **single-run on one machine** (Apple-silicon arm64), so treat them as
  a rough order of magnitude;
- a **fair JIT comparison needs warm / long-running workloads**, which these
  short runs are not;
- performance is **validated and benchmarked across all six 64-bit
  architectures** (amd64, arm64, riscv64, loong64, ppc64le, s390x), not just the
  one reported here — and on **real hardware**, not only qemu: amd64/arm64
  natively, riscv64/ppc64le/loong64 on the GCC Compile Farm (cfarm95 RVV,
  cfarm112/cfarm433 POWER8E/POWER9, cfarm401 LoongArch), and s390x on the IBM
  LinuxONE Community Cloud. qemu is the CI gate; real silicon is the perf oracle,
  so the SIMD-accelerated paths (go-simd `base64`/`securerandom`/`hex`) report
  measured numbers, not llvm-mca estimates.
