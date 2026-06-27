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
  one reported here.
