# WebAssembly (js/wasm)

**`GOOS=js GOARCH=wasm` is a first-class target.** go-embedded-ruby is pure Go
with cgo disabled, so the interpreter, the numeric stack and the cgo-free image
pipeline compile to a single WebAssembly module and run **entirely in the
browser** — there is no server-side code.

There are two distinct ways to ship Ruby to the browser, and they answer
different needs:

| | Playground (`cmd/wasm`) | Closed-world app (`rbgo build --target wasm`) |
| --- | --- | --- |
| What runs | **arbitrary** Ruby the user types | **one** program you baked in |
| Front-end | linked (lexer/parser/compiler) | **dropped** |
| Built by | `web/build.sh` | `rbgo build --closed --target wasm` |
| Entry | `rbgoEval` / `rbgoImage` JS functions | the embedded program, then `select{}` |
| Use it for | a REPL / live demo page | a self-contained Ruby browser app |

Both produce a `.wasm` module that you serve next to Go's `wasm_exec.js` loader.

## 1. The playground — the full interpreter in the browser

The playground links the **whole** interpreter — front-end (lexer, parser,
compiler), VM, the numeric stack and the cgo-free image pipeline — so the page
can evaluate *any* Ruby the user types. It lives in
[`web/`](https://github.com/go-embedded-ruby/ruby/tree/main/web) and is a Ruby
REPL plus an image pipeline (load an image → `gaussian_blur`/`sobel`/`canny` →
render to a `<canvas>`).

```sh
./web/build.sh serve     # build web/rbgo.wasm and serve http://localhost:8080
```

`build.sh` cross-compiles [`cmd/wasm`](https://github.com/go-embedded-ruby/ruby/tree/main/cmd/wasm)
with `GOOS=js GOARCH=wasm` and copies Go's `wasm_exec.js` loader from the active
toolchain. Build without `serve` to produce just the artifacts for static hosting
(e.g. GitHub Pages):

```sh
./web/build.sh           # produces web/rbgo.wasm + web/wasm_exec.js
```

The module publishes two functions on the JS global object:

| function | returns | used by |
| --- | --- | --- |
| `rbgoEval(src)` | `{output, value, error}` | the REPL |
| `rbgoImage(src, bytes)` | `{output, value, error, bytes}` | the image demo |

`rbgoImage` binds the input image's raw bytes to the Ruby constant `INPUT` (a
`String`), runs `src`, and — when the program's result is a `String` (e.g. the
output of `Image#to_png`) — hands those bytes back as a `Uint8Array` for the page
to paint. The image demo's pipeline is plain Ruby:

```ruby
Image.decode(INPUT).gaussian_blur(2.0).sobel_mag.to_png
```

Arbitrary REPL input can never crash the interpreter: a native binding that
faults on bad arguments is converted to a rescuable Ruby `ArgumentError`.

## 2. `rbgo build --target wasm` — a closed-world wasm app

To ship a *single* program rather than a REPL, AOT-bake it into a closed-world
wasm module that **drops the front-end**:

```sh
rbgo build --closed --target wasm -o app.wasm app.rb
```

- `--target wasm` **requires `--closed`** — the wasm entry point runs the
  embedded program; there is no command-line file to read in a browser tab. The
  program is frozen to bytecode at build time and the lexer/parser/compiler are
  left out of the link (see [the AOT compiler](https://github.com/go-embedded-ruby/ruby/blob/main/docs/aot-compiler.md)
  for how closed-world builds work).
- The build sets `GOOS=js GOARCH=wasm` on the nested `go build`, so it links the
  wasm closed-world entry. After the embedded program returns it does **not**
  exit: it parks on `select{}`, because a Go wasm module that returns from
  `main()` is torn down — it must stay alive for any JS event or animation
  callbacks the program registered.

Serve the emitted `app.wasm` next to `wasm_exec.js` (copy it from
`$(go env GOROOT)/lib/wasm/wasm_exec.js`).

### The `JS` module — DOM, Canvas and events from Ruby

A closed-world wasm program reaches the page through the built-in **`JS`
module**, which the VM registers automatically on wasm. So a Ruby browser app can
render and handle events with **no hand-written JavaScript**:

```ruby
doc = JS.document
canvas = doc.call("getElementById", "screen")
ctx = canvas.call("getContext", "2d")

ctx.set("fillStyle", "tomato")
ctx.call("fillRect", 10, 10, 120, 80)

doc.call("getElementById", "go").on("click") do |_event|
  JS.log("clicked!")
end

# an animation loop: reschedule from inside the block
draw = nil
draw = ->(t) {
  ctx.call("clearRect", 0, 0, 320, 240)
  ctx.call("fillRect", (t % 300), 50, 20, 20)
  JS.raf(&draw)
}
JS.raf(&draw)
```

The surface:

| call | does |
| --- | --- |
| `JS.global` / `JS.window` / `JS.document` | the JS globals, as `JS::Ref` handles |
| `JS.log(*args)` | `console.log` |
| `JS.raf { \|t\| … }` | `requestAnimationFrame` (reschedule inside for a loop) |
| `ref.get(name)` / `ref[name]` | read a property |
| `ref.set(name, value)` | write a property |
| `ref.call(method, *args)` | invoke a method |
| `ref.on(event) { \|e\| … }` | `addEventListener` |

JS values flow both ways: primitives convert directly (numbers, strings,
booleans, null/undefined → `nil`), and objects/functions become opaque `JS::Ref`
handles.

## Verifying a wasm build

The closed-world wasm path is covered end to end by an integration test
([`cmd/rbgo/wasm_build_test.go`](https://github.com/go-embedded-ruby/ruby/blob/main/cmd/rbgo/wasm_build_test.go),
gated behind `RBGO_BUILD_IT=1` because it shells out to the Go toolchain). It
bakes a `JS`-using program, then asserts the output carries the WebAssembly magic
(`\0asm`) and that the linker dropped the front-end (no `go-ruby-parser/parser`
or `internal/compiler` symbols in the module):

```sh
RBGO_BUILD_IT=1 go test ./cmd/rbgo -run TestClosedWasmBuildIntegration
```

Both targets also build directly:

```sh
GOOS=js GOARCH=wasm go build ./cmd/wasm    # the playground
GOOS=js GOARCH=wasm go build ./cmd/rbgo    # the rbgo CLI / closed-world entry
```
