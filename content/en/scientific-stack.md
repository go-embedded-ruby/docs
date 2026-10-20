---
title: "Scientific stack"
weight: 70
---

# Scientific stack & WebAssembly {#scientific-stack-webassembly}

Beyond the language core, go-embedded-ruby ships a **cgo-free numeric and
scientific stack** as built-in Ruby classes. Each numeric extension binds a
pure-Go sibling library, so the whole stack keeps the project's **CGO=0**
guarantee, cross-compiles to all six 64-bit architectures, and — because it is
pure Go — also compiles to **WebAssembly and runs in the browser**.

All behaviour is differential-tested against MRI Ruby 4.0.5 (where MRI has an
equivalent) and the bindings hold the org-wide **100% coverage** gate.

## Numeric core {#numeric-core}

| Type | Highlights |
| --- | --- |
| `Complex` | `Complex(re, im)`, arithmetic, `real`/`imaginary`/`abs`/`conjugate`, `==` |
| `Rational` | exact `big.Rat`-backed arithmetic, Float contamination, coercion, `**`, `<=>` via `Comparable` |
| `Math` | `sqrt`/`cbrt`/`exp`/`log`/`log2`/`log10`, trig & hyperbolic, `atan2`/`hypot`/`pow`, `Math::PI`/`Math::E` |

```ruby
p Rational(1, 2) + Rational(1, 3)   # => (5/6)
p Complex(1, 1) * Complex(2, -1)    # => (3+1i)
p Math.hypot(3, 4)                  # => 5.0
```

## NDArray — NumPy-style arrays {#ndarray-numpy-style-arrays}

Binds [go-ndarray](https://github.com/go-ndarray/ndarray): constructors
`zeros`/`ones`/`full`/`arange`/`from`, element-wise `+ - * /` with scalar
broadcasting, ufuncs (`sqrt`/`exp`/`log`/`sin`/`cos`/`abs`), reductions
(`sum`/`mean`/`max`/`min`/`prod`/`argmax`/`argmin`), `matmul`/`dot`,
`transpose`/`reshape`/`flatten`, and `shape`/`to_a`/`[]`.

```ruby
a = NDArray.arange(0, 6).reshape(2, 3)
p a.sum                 # => 15.0
p a.transpose.shape     # => [3, 2]
```

## FFT — numpy.fft-style transforms {#fft-numpyfft-style-transforms}

Binds [go-fft](https://github.com/go-fft/fft) — no FFTW. 1-D
(`fft`/`ifft`/`rfft`/`irfft`), N-D and 2-D (`fftn`/`ifftn`/`fft2`/`ifft2`),
bin-frequency helpers (`fftfreq`/`rfftfreq`), windows
(`hann`/`hamming`/`blackman`/`blackman_harris`/`bartlett`) and spectral helpers
(`psd`/`spectrogram`). Spectra come back as `Complex` arrays.

```ruby
p FFT.fft([1, 2, 3, 4]).map { |c| c.real.round(3) }   # => [10.0, -2.0, -2.0, -2.0]
```

## Special — scipy.special-style functions {#special-scipyspecial-style-functions}

Binds [go-numerics/special](https://github.com/go-numerics/special) — no SciPy:
**all 67** of its functions under 66 Ruby names. **63 of the 67** names are
scipy's own: they come from the correspondence that module measures itself
against in ULPs (`bench/scipy_server.py` and `testdata/gen/fam_*.py`), so
`Log1pExp` is `softplus` and `LogGamma` is `gammaln` rather than a
transliteration of the Go name. The four that are not say so — `gammasgn` and
`kn` exist in scipy but are unpaired in the module; `log_factorial` is scipy's
`gammaln(n+1)` and equals it exactly; `loggamma_sign` has no scipy counterpart,
answering the pair \[log |Γ(x)|, sign] that Go's `math.Lgamma` returns and scipy
splits between `gammaln` and `gammasgn`; and **`in` is not a scipy name at
all**, because scipy cannot have one — `in` is a Python keyword — so `iv(0, x)`
is scipy's spelling, `in(0, x)` is ours, and the two are equal exactly. Gamma (`gamma`/`gammaln`/`gammasgn`/`digamma`/`polygamma`/`factorial`),
beta (`beta`/`betaln`/`binom`/`comb`/`perm`), error and normal
(`erf`/`erfc`/`erfcx`/`erfinv`/`erfcinv`/`ndtr`/`ndtri`/`log_ndtr`), logistic
(`expit`/`logit`/`softplus`/`xlogy`), reductions
(`logsumexp`/`softmax`/`log_softmax`), `zeta` (Riemann, or Hurwitz with a second
argument), the incomplete gamma and beta functions with their inverses, Bessel
(`j0`…`k1e`, `jv`/`yv`/`iv`/`kv`, `jn_zeros`) and the complete elliptic
integrals.

Each is a **ufunc**, as scipy's is: a Float answers a Float, an `NDArray`
answers an `NDArray` of the same shape, and a Float mixed with an array
broadcasts.

```ruby
p Special.gamma(5.0)                                        # => 24.0
p Special.erf(1.0)                                          # => 0.8427007929497149
p Special.zeta(2.0)                                         # => 1.6449340668482264
p Special.gamma(NDArray.from([1.0, 2.0, 3.0, 4.0], [2, 2])).to_a  # => [1.0, 1.0, 2.0, 6.0]
```

## Stats — scipy.stats-style distributions {#stats-scipystats-style-distributions}

Binds [go-statistics/stats](https://github.com/go-statistics/stats) — no SciPy:
the **32 univariate distributions**, again under scipy's names, which is why
`StudentT` is `t`, `ChiSquared` is `chi2`, `DiscreteUniform` is `randint` and
`Weibull` is `weibull_min`. Continuous
`norm`/`cauchy`/`expon`/`gumbel_r`/`halfnorm`/`laplace`/`logistic`/`rayleigh`/
`uniform`/`chi`/`chi2`/`gamma`/`invgamma`/`lognorm`/`lomax`/`pareto`/`t`/
`triang`/`vonmises`/`weibull_min`/`beta`/`f`/`ncx2`/`reciprocal`; discrete
`bernoulli`/`geom`/`poisson`/`zipf`/`binom`/`nbinom`/`randint`/`hypergeom`.

Arguments follow scipy's order — the shape parameters, then `loc`, then `scale`
— and every distribution answers the same method set, `Stats::Continuous` and
`Stats::Discrete` differing only in the density: `pdf`/`logpdf` against
`pmf`/`logpmf`, plus `cdf`/`logcdf`/`sf`/`logsf`/`ppf`/`isf`, the moments
`mean`/`var`/`std`/`skew`/`kurtosis`/`median`/`entropy`/`moment`,
`interval`/`support`, and `rvs(size…, seed:)`. The per-point methods are ufuncs
too. A parameter outside its domain is refused at construction.

```ruby
p Stats.norm.pdf(0.0)            # => 0.3989422804014327
p Stats.norm.ppf(0.975)          # => 1.9599639845400538
p Stats.poisson(3.0).pmf(2)      # => 0.22404180765538775
p Stats.binom(10, 0.5).mean      # => 5.0
```

## Integrate — scipy.integrate-style quadrature {#integrate-scipyintegrate-style-quadrature}

Binds [go-numerics/integrate](https://github.com/go-numerics/integrate) — no
SciPy: adaptive `quad` (finite **or infinite** limits, `points:` for a known
kink), the fixed rules `fixed_quad` and `romberg`, the sampled rules
`trapezoid`/`simpson`/`romb`/`cumulative_trapezoid` (`x:` or `dx:`), the
multiple integrals `dblquad` and `nquad`, and `gauss_legendre` for a rule's
nodes and weights.

The integrand is a **block**, which is how Ruby passes a callable — and which
takes `&f` for a Proc or lambda without any further surface. `quad` answers
scipy's `(value, abserr)` pair, so it destructures the same way.

The two quadrature results in the example below are rounded, and the others on
this page are not. Quadrature sums products, which Go fuses into an FMA on
arm64 and not on amd64, so the last bit of `quad`'s answer differs between
architectures: `0.3333333333333333` on one and `0.33333333333333337` on the
other. The special functions do not drift that way — their kernels are kept
FMA-free on purpose — which is why only these two carry a `round`.

```ruby
v, err = Integrate.quad(0.0, 1.0) { |x| x * x }
p v.round(12)                                             # => 0.333333333333
p Integrate.quad(0.0, Float::INFINITY) { |x| Math.exp(-x) }[0].round(12)  # => 1.0
p Integrate.trapezoid([0.0, 1.0, 4.0], dx: 1.0)           # => 3.0
```

## Optimize — scipy.optimize-style root finding and minimisation {#optimize-scipyoptimize-style-root-finding-and-minimisation}

Binds [go-numerics/optimize](https://github.com/go-numerics/optimize) — no
SciPy: the bracketing root finders `brentq`/`brenth`/`bisect`/`ridder` and
`newton` (secant, Newton–Raphson with `fprime:`, Halley with `fprime2:`),
`minimize_scalar` and `fminbound`, `minimize` over several variables
(`BFGS`/`CG`/`Nelder-Mead`/`Powell`/`L-BFGS-B`, with `bounds:`), plus
`curve_fit`, `approx_fprime` and `bracket`.

Return shapes are scipy's: one number is a Float, a tuple an Array (`curve_fit`
is `(popt, pcov)`), and an `OptimizeResult` or `RootResults` a Hash with symbol
keys — which `full_output: true` asks a root finder for.

```ruby
p Optimize.brentq(0.0, 2.0) { |x| x * x - 2 }             # => 1.4142135623731364
p Optimize.minimize_scalar { |x| (x - 3.0) ** 2 }[:x]     # => 3.0
r = Optimize.minimize([0.0, 0.0]) { |v| (v[0] - 1.0) ** 2 + (v[1] - 2.0) ** 2 }
p r[:x].map { |z| z.round(6) }                            # => [1.0, 2.0]
```

## Image — scikit-image-style processing {#image-scikit-image-style-processing}

Binds [go-images](https://github.com/go-images/images): `Image.new`/`load`/`save`
plus in-memory `decode`/`to_png`/`to_jpeg`, pixel `get`/`set`, filters
(`gaussian_blur`/`box_blur`/`median`/`sharpen`), edges
(`sobel`/`sobel_mag`/`prewitt`/`scharr`/`laplacian`/`canny`), morphology
(`erode`/`dilate`), geometry (`resize`/`rotate90`/`crop`/`flip_*`) and colour
(`grayscale`/`invert`/`rgb_to_hsv`/`otsu`).

```ruby
edges = Image.load("photo.png").gaussian_blur(1.0).sobel_mag
edges.save("edges.png")
```

## Set {#set}

Binds [go-composites/set](https://github.com/go-composites/set): `Set.new`/`[]`,
`add`(`<<`)/`add?`/`delete`/`merge`/`clear`,
`include?`/`member?`/`size`/`length`/`count`/`empty?`, `subset?`/`superset?`,
`union`(`|`)/`intersection`(`&`)/`difference`(`-`), and `each`/`to_a`/`to_set`.

```ruby
p (Set.new([1, 2, 3]) & Set.new([2, 3, 4])).to_a.sort   # => [2, 3]
```

## Bag {#bag}

A multiset / counter (element → multiplicity), binding
[go-composites/bag](https://github.com/go-composites/bag): `add`(`<<`)/`delete`,
`count`/`size`/`distinct`/`distinct_size`/`most_common`,
`union`(`+`)/`difference`(`-`)/`intersection`, `include?`/`member?`/`empty?`,
and `each`/`to_a`.

```ruby
b = Bag.new
b << :a << :a << :b
p b.count(:a)        # => 2
p b.most_common(1)   # => [[:a, 2]]
```

## Time & Date {#time-date}

`Time` binds [go-composites/time](https://github.com/go-composites/time) and
`Date` binds [go-composites/date](https://github.com/go-composites/date):
construction (`Time.now`/`at`/`parse`, `Date.new`/`parse`), arithmetic,
`strftime`/`strptime` (Time), the field accessors
(`year`/`month`/`mday`/`hour`/`min`/`sec`/`wday`/`yday`), weekday predicates and
`leap?`, day/month stepping (`next_day`/`prev_month`/`<<`/`>>`), and
comparisons via `<=>`.

```ruby
p Date.new(2026, 6, 21).next_month.to_s   # => "2026-07-21"
```

## BigDecimal {#bigdecimal}

Arbitrary-precision decimal, binding
[go-composites/bigfloat](https://github.com/go-composites/bigfloat): `+ - * / **`,
`abs`/`ceil`/`floor`/`round`/`truncate`/`pow`, the integer divisions
`div`/`divmod`/`modulo`/`remainder`, the parts `frac`/`fix`/`exponent`/
`precision`/`sign`/`split`, `to_f`/`to_i`/`to_r`/`to_s`, the predicates
`zero?`/`nan?`/`finite?`/`infinite?`, and comparisons — exact decimals without
the float rounding of `Float`.

MRI's `BigDecimal#sqrt(n)` is **not bound yet**:
[ruby#825](https://github.com/go-embedded-ruby/ruby/issues/825). This page said
it was until 2026-10-20, which is the first broken promise
`tools/numericsurface` found.

```ruby
p (BigDecimal("0.1") + BigDecimal("0.2")).to_s   # exact, no 0.30000000000000004
```

## In the browser (WebAssembly) {#in-the-browser-webassembly}

The interpreter and the whole stack above compile to `GOOS=js GOARCH=wasm` and
run **entirely in the browser** — there is no server-side code. The
[`web/`](https://github.com/go-embedded-ruby/ruby/tree/main/web) directory holds
a self-contained playground (a Ruby REPL and an image pipeline that loads an
image, runs `gaussian_blur`/`sobel`/`canny`, and renders it to a `<canvas>`):

```sh
./web/build.sh serve     # build web/rbgo.wasm and serve http://localhost:8080
```

{{< callout type="info" >}}
**Both wasm targets build**

The playground above and `rbgo build --closed --target wasm` target
`GOOS=js GOARCH=wasm`, which this page used to say failed to compile — true
on `3e8e3cc`, fixed by
[ruby#684](https://github.com/go-embedded-ruby/ruby/pull/684) and gated since
by the `wasm (js, browser)` lane. The WASI target
(`GOOS=wasip1 GOARCH=wasm`) builds and runs as before. See
**[WebAssembly]({{< relref "webassembly.md" >}})**.
{{< /callout >}}
