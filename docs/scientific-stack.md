# Scientific stack & WebAssembly

Beyond the language core, go-embedded-ruby ships a **cgo-free numeric and
scientific stack** as built-in Ruby classes. Each numeric extension binds a
pure-Go sibling library, so the whole stack keeps the project's **CGO=0**
guarantee, cross-compiles to all six 64-bit architectures, and — because it is
pure Go — also compiles to **WebAssembly and runs in the browser**.

All behaviour is differential-tested against MRI Ruby 4.0.5 (where MRI has an
equivalent) and the bindings hold the org-wide **100% coverage** gate.

## Numeric core

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

## NDArray — NumPy-style arrays

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

## FFT — numpy.fft-style transforms

Binds [go-fft](https://github.com/go-fft/fft) — no FFTW. 1-D
(`fft`/`ifft`/`rfft`/`irfft`), N-D and 2-D (`fftn`/`ifftn`/`fft2`/`ifft2`),
bin-frequency helpers (`fftfreq`/`rfftfreq`), windows
(`hann`/`hamming`/`blackman`/`blackman_harris`/`bartlett`) and spectral helpers
(`psd`/`spectrogram`). Spectra come back as `Complex` arrays.

```ruby
p FFT.fft([1, 2, 3, 4]).map { |c| c.real.round(3) }   # => [10.0, -2.0, -2.0, -2.0]
```

## Image — scikit-image-style processing

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

## Set

Binds [go-composites/set](https://github.com/go-composites/set): `Set.new`/`[]`,
`add`(`<<`)/`add?`/`delete`/`merge`/`clear`,
`include?`/`member?`/`size`/`length`/`count`/`empty?`, `subset?`/`superset?`,
`union`(`|`)/`intersection`(`&`)/`difference`(`-`), and `each`/`to_a`/`to_set`.

```ruby
p (Set.new([1, 2, 3]) & Set.new([2, 3, 4])).to_a.sort   # => [2, 3]
```

## In the browser (WebAssembly)

The interpreter and the whole stack above compile to `GOOS=js GOARCH=wasm` and
run **entirely in the browser** — there is no server-side code. The
[`web/`](https://github.com/go-embedded-ruby/ruby/tree/main/web) directory holds
a self-contained playground: a Ruby REPL and an image pipeline that loads an
image, runs `gaussian_blur`/`sobel`/`canny`, and renders the result to a
`<canvas>`.

```sh
./web/build.sh serve     # build web/rbgo.wasm and serve http://localhost:8080
```

The wasm module ([`cmd/wasm`](https://github.com/go-embedded-ruby/ruby/tree/main/cmd/wasm))
publishes two JS functions:

| function | returns | used by |
| --- | --- | --- |
| `rbgoEval(src)` | `{output, value, error}` | the REPL |
| `rbgoImage(src, bytes)` | `{output, value, error, bytes}` | the image demo |

`rbgoImage` binds the input image's raw bytes to the Ruby constant `INPUT`, runs
`src`, and returns the result `String`'s bytes (e.g. from `Image#to_png`) as a
`Uint8Array`. Arbitrary REPL input can never crash the interpreter: a native
binding that faults on bad arguments is converted to a rescuable Ruby
`ArgumentError`.
