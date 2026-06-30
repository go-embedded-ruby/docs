# go-embedded-ruby documentation

**A pure-Go implementation of Ruby** — a bytecode VM with the lexer, parser, and
compiler embedded in the binary, compiling to a single static executable with
**zero cgo**.

go-embedded-ruby compiles Ruby source to bytecode and runs it on a stack VM in
the **mruby/YARV lineage**. Because the front-end (lexer + parser + compiler)
ships *inside* the binary, dynamic features like `eval` and runtime `require`
work without a separate toolchain. Ruby objects are ordinary Go heap objects, so
**Go's garbage collector is reused** rather than reimplemented.

Being pure Go buys three things at once: trivial cross-compilation, a single
static binary, and — most importantly — it is the **first Ruby embeddable in a
Go program with no C toolchain** ([go-mruby][go-mruby] needs cgo). The project
targets **Ruby 4.0** semantics and is grown test-first against a *differential
oracle* — every snippet's output is checked against **two independent reference
implementations, MRI (CRuby) 4.0.5 and JRuby**
([`scripts/oracle.sh`](https://github.com/go-embedded-ruby/ruby/blob/main/scripts/oracle.sh)
runs all three side by side) — with **100% coverage enforced in CI across all six
64-bit architectures** (amd64, arm64, riscv64, loong64, ppc64le, s390x) and three
OSes. Beyond synthetic tests, the bar is real-world Ruby: idioms and test suites
from **reference applications such as Rails** (ActiveSupport's pure-Ruby
`core_ext`) and **OpenVox**/Puppet drive the remaining work and double as
performance baselines against MRI and JRuby.

## Supported today

Every feature below is **differential-tested against MRI Ruby 4.0.5**:

- **Values:** integers (`int64`, with automatic **Bignum** promotion on int64
  overflow and arbitrary-precision integer literals, **radix literals** `0x`/`0o`/
  bare-`0`-octal/`0b`/`0d` with underscores), floats (incl. **scientific
  notation** `1.5e3`/`1e-3`), strings (**single- and double-quoted**, with
  interpolation and heredocs), symbols (incl.
  **operator-method symbols** `:+`/`:<<`/`:[]=`/`:<=>`), arrays, hashes, ranges
  (incl. beginless/endless), `true`/`false`/`nil`, `self`, `Proc`/lambda,
  `Regexp`/`MatchData`, `Struct`.
- **Operators:** arithmetic (`+ - * / %`, Ruby floor division, `**`),
  comparison/`<=>`, `==`/`===`, bitwise/shift (`<< >> & | ^ ~`, arbitrary
  precision), `&&`/`||`, ternary, ranges; correct negative-literal precedence
  (`-2.abs == 2`, `-2**2 == -4`).
- **Control flow:** `if`/`elsif`/`else`, `unless`, `while`/`until`,
  `case`/`when`, statement modifiers (incl. modifier `rescue`,
  `expr rescue fallback`), `begin`/`rescue`/`ensure`/`else`/`retry`,
  `break`/`next`, `Kernel#loop`, and **`Fiber`** (cooperative coroutines on
  goroutines — `Fiber.new`/`resume`/`Fiber.yield`/`alive?`).
- **Concurrency:** **`Thread`** (`new`/`join`/`value`/`status`/`current`/`main`/
  `list`/`pass`, thread-locals, exception propagation on join), **`Mutex`**
  (`lock`/`try_lock`/`synchronize`/`owned?`) and **`Queue`** (blocking
  `push`/`pop`/`close`), on an **emulated GVL** — one Ruby thread runs at a time,
  matching MRI's memory model (race-free under the Go race detector).
- **Pattern matching (`case`/`in`):** value, variable-binding, class/constant,
  array (incl. splat and nested), hash (`deconstruct_keys`, `**rest`/`**nil`),
  find (`[*pre, x, *post]`), pin (`^x`) and alternative (`a | b`) patterns;
  the `=> name` binding suffix, guards (`if`/`unless`), the one-line forms
  `expr => pattern` and `expr in pattern`, and `NoMatchingPatternError`.
- **Assignment:** multiple assignment / destructuring (`a, b = 1, 2`,
  `x, *rest = …`, `*init, last = …`) and compound assignment
  (`+= -= *= /= %= <<= ||= &&=`).
- **Methods:** required / optional / `*splat` / keyword (`a:`, `b: 2`) /
  `**rest` / `&block` parameters, setter defs (`def name=`), **endless methods**
  (`def foo = expr`), **singleton method defs** (`def obj.foo` / `def Const.foo`),
  recursion, `return`, `super`.
- **Blocks / Procs / lambdas:** `{ }` / `do…end` closures, `yield`,
  `block_given?`, `&block` capture, **block params** with destructuring
  (`|(a, b)|`) and **rest** (`|*rest|`, `|head, *rest|`), **numbered params
  (`_1`/`_2`) and `it`**, `Proc`/`lambda`/**stabby `->(){}`**, `&proc` block-pass
  and `Symbol#to_proc` (the `&:sym` shorthand).
- **Classes & modules:** inheritance, `@ivars`, **`@@class variables`** (shared
  down the hierarchy), `new`/`initialize`, constants and constant assignment,
  **class methods** (`def self.foo`), modules + **`include`/`prepend`** (mixins,
  with ancestor-chain `super` through included/prepended modules and the
  singleton chain; `Module#ancestors`/`include?`),
  **`attr_accessor`/`reader`/`writer`**, **`Struct.new`**.
- **Metaprogramming:** dynamic dispatch via mutable method tables,
  `method_missing`, `send`/`public_send`, `respond_to?`, **`define_method`**,
  **`instance_eval`/`instance_exec`**, **`class_eval`/`module_eval`/`class_exec`**,
  `instance_variable_get`/`set`/`defined?`, **string `eval`** — the embedded
  front-end compiling Ruby at runtime (against the caller's `self`) — **`Binding`**
  (`binding`, `Binding#eval`, `eval(str, binding)`, `local_variable_get`/`set`/
  `defined?`, `local_variables`, `receiver` — capturing a frame's locals so
  eval'd code reads and writes them), the
  class/module **hooks** `inherited`/`included`/`method_added`/`extended`,
  **`define_singleton_method`/`extend`**, and **`$global variables`**.
- **Runtime loading:** **`require`/`require_relative`** load, compile and run a
  `.rb` file once (relative + search-path resolution, `LoadError` on miss) — the
  embedded front-end loading code at runtime.
- **Strings:** mutable (reference semantics) with `<<`/concat/replace/prepend/
  insert/`[]=`/slice!/the bang methods and `freeze`/`FrozenError`;
  interpolation, heredocs (`<<`/`<<-`/`<<~`), `%w`/`%i` literals,
  `%`/`format`/`sprintf`, case/strip/`split`/`each_char`/`lines`/`succ`(`next`)
  and friends.
- **Regular expressions:** `/re/imx` literals, `Regexp`/`MatchData`, `=~` /
  `match` / `match?` / `scan` / `gsub` / `sub` / `split`, and the match globals
  `$~` / `$1`..`$N` / `$&` / `` $` `` / `$'` — running on the standalone pure-Go
  [go-ruby-regexp][go-ruby-regexp] engine, so the build stays **CGO=0**.
- **Standard library leaves:** **`JSON`** (`generate`/`dump`/`pretty_generate`/
  `parse` + `Object#to_json`, key order preserved), **`Digest`** (MD5/SHA1/
  SHA256/SHA512), **`Base64`**, and **`Zlib`** (crc32/adler32 + Deflate/Inflate)
  — all `require`-able and pure-Go.
- **`Marshal`** (`dump`/`load`/`restore` + `MAJOR_VERSION`/`MINOR_VERSION`):
  Ruby's binary serialization, **byte-for-byte identical to MRI** across
  Integer/Bignum, Float, Symbol, String, Array and Hash (incl. defaults), with
  the symbol and object-link tables so shared objects and cycles round-trip.
  Runs on the standalone pure-Go [go-ruby-marshal][go-ruby-marshal] engine, so
  the build stays **CGO=0**.
- **File & Random:** **`File`** — path helpers and `read`/`write`/`exist?`/
  `size`/`delete` (`Errno::ENOENT` on a missing path); **`Random`** — a bit-exact
  reimplementation of MRI's seeded MT19937.
- **IO & Dir:** **`IO`** with `$stdout`/`$stderr`/`STDOUT`/`STDERR`/`$stdin` as
  real objects, **`StringIO`** (`require "stringio"`) with the full read/write
  surface, and `Kernel#warn` — `puts`/`print`/`p` route through the current
  `$stdout` so reassigning it to a `StringIO` captures output, as in MRI;
  **`Dir`** — `entries`/`children`/`glob`, `exist?`/`empty?`/`pwd`/`home`,
  `mkdir`/`rmdir`/`chdir`, raising `Errno::ENOENT`/`Errno::EEXIST`.
- **Collections:** Array / Hash / Range with `Enumerable` (map/select/reduce/
  `minmax`/…) and `Comparable`, both written once in embedded Ruby; Array **bang
  methods** (`map!`/`sort!`/`select!`/`reject!`/`compact!`/`uniq!`/`reverse!`),
  **structural/combinatorial ops** (`transpose`/`product`/`combination`/`to_h`),
  the **`Hash[…]`** constructor, **String ranges** (`("a".."e")` via
  `String#succ`); **`Range#step`/`Integer#step`**; an eager **`Enumerator`** — every blockless
  iterator returns one, with `next`/`peek`/`rewind`/`with_index`/`to_a` and full
  `Enumerable` chaining — and **`Enumerator::Lazy`** (`lazy`) for deferred chains
  over finite or infinite sources.
- **Objects:** `dup`/`clone`/`freeze`/`frozen?`, `equal?`,
  `object_id`/`__id__` (MRI's deterministic immediate-value ids, stable per
  reference object), `instance_variable_get`/`set`.
- **Numeric & scientific stack:** `Complex`, `Rational`, the `Math` module, and a
  **cgo-free scientific stack** — `NDArray` (NumPy-style arrays), an `FFT` module
  (numpy.fft-style transforms), `Image` (scikit-image-style processing) — plus
  container/value types `Set`, `Bag` (multiset), `Time`, `Date` and
  `BigDecimal`, each binding a pure-Go sibling library (go-ndarray / go-fft /
  go-images / go-composites). See
  [Scientific stack](scientific-stack.md).
- **WebAssembly (js/wasm):** a first-class target — the interpreter and the whole
  stack compile to `GOOS=js GOARCH=wasm` and run **in the browser**, both as a
  REPL playground and as closed-world apps (`rbgo build --closed --target wasm`)
  that drive the DOM/Canvas via the built-in `JS` module. See
  [WebAssembly](webassembly.md).

Closed-world builds have landed (`rbgo build --closed` bakes the program in as
bytecode and drops the front-end; `--target wasm` cross-compiles it to the
browser). Still ahead (see the [roadmap](roadmap.md)):
conformance and representation/perf tuning (Phase 8).

## Running real-world Ruby: Puppet

Beyond parsing real-world Ruby, **rbgo runs the real [Puppet](https://github.com/puppetlabs/puppet)
`puppet apply` CLI end-to-end**. `require "puppet"` **fully boots** the framework
(Puppet 8.11.0) on a pure-Go CGO=0 `rbgo` — its pure-Ruby gem dependencies
(`semantic_puppet`, `concurrent-ruby`, `facter`, …) load on the `$LOAD_PATH` — and
a manifest then travels the **complete** Puppet path: the genuine
`Puppet::Util::CommandLine` → `Puppet::Application::Apply` entry point (real
`OptionParser`), all types + providers loaded, the settings catalog applied to
disk, then the user catalog applied through the transaction / RAL. The real CLI
emits genuine Puppet output and exits `0` — `apply -e 'notify { "hello": message
=> "hi from rbgo cli" }'` prints `Notice: hi from rbgo cli` and `Notice:
…/Notify[hello]/message: defined 'message' as 'hi from rbgo cli'`.

This is the **C-extension → pure-Go shim** strategy validated end to end: a real
Ruby app ships as one static CGO=0 binary because its C-backed gem APIs are backed
by pure Go (Puppet's deps are pure Ruby, so they load as-is). Reaching the CLI
added a real `OptionParser`, `File::Stat`/`FileTest` + on-disk filesystem
operations, and deep Ruby fixes (`class_eval` lexical scope, `return` in
`define_method`, class-method `super`, …) on top of the earlier boot surface
(`autoload`, `ERB`, `openssl` with real crypto, `net/http`, `StringScanner`,
`fileutils`, …). Three resource types now converge end-to-end through the
transaction / RAL: **`notify`**, **`file`** (managing a real file on disk), and
**`exec`** — which runs its command via pure-Go process execution, honouring the
`onlyif` / `unless` / `creates` / `path` guards — and `puppet apply` exits
cleanly, the YAML run report / state round-tripping through the pure-Go Psych
emitter / loader. The honest frontier is the **broader resource providers**:
`user` / `group` are provider-ready, while `package` / `service` need a host
package manager / systemd and root. See
[Conformance](conformance.md#running-puppet-puppet-apply-runs-end-to-end).

## Repositories

| Repo | What it is |
| --- | --- |
| [`ruby`](https://github.com/go-embedded-ruby/ruby) | the interpreter — the bytecode VM, the compiler, and the `rbgo` CLI binary (it imports the front-end below) |
| [`docs`](https://github.com/go-embedded-ruby/docs) | this documentation site (MkDocs Material, versioned with mike) |
| [`go-embedded-ruby.github.io`](https://github.com/go-embedded-ruby/go-embedded-ruby.github.io) | the organization landing page (Hugo) |

## Ecosystem — the `go-ruby-*` family

rbgo is increasingly assembled from a family of **standalone pure-Go (CGO=0),
MRI-compatible libraries** living in **sibling orgs**, each with **100% coverage
and 6-arch CI**, that the interpreter composes and binds as native modules. The
design principle is a clean seam: a stdlib piece whose work is **pure compute and
needs no interpreter** — a regexp matcher, an ERB compiler, a YAML emitter, a
Marshal codec, an OptionParser argv engine — becomes a reusable standalone
library, while only the thin **interpreter-dependent glue** stays in rbgo. rbgo
still ships as a **single CGO=0 static binary**; every extracted piece is
independently reusable, tested and 6-arch by any Go program.

The family is **66 standalone pure-Go libraries**, every one **non-empty,
CI-green, 100% coverage and 6-arch**. The `go.mod` currently **binds 40 of them**
as native modules — including the front-end ([go-ruby-parser](https://github.com/go-ruby-parser)),
the regexp engine ([go-ruby-regexp](https://github.com/go-ruby-regexp)), and
[go-ruby-erb](https://github.com/go-ruby-erb),
[go-ruby-marshal](https://github.com/go-ruby-marshal),
[go-ruby-yaml](https://github.com/go-ruby-yaml),
[go-ruby-format](https://github.com/go-ruby-format),
[go-ruby-optparse](https://github.com/go-ruby-optparse),
[go-ruby-strscan](https://github.com/go-ruby-strscan).

The full 66-library family (alphabetical):

[`abbrev`](https://github.com/go-ruby-abbrev/abbrev), [`base64`](https://github.com/go-ruby-base64/base64), [`benchmark`](https://github.com/go-ruby-benchmark/benchmark), [`bigdecimal`](https://github.com/go-ruby-bigdecimal/bigdecimal), [`cgi`](https://github.com/go-ruby-cgi/cgi), [`cmath`](https://github.com/go-ruby-cmath/cmath), [`complex`](https://github.com/go-ruby-complex/complex), [`csv`](https://github.com/go-ruby-csv/csv), [`date`](https://github.com/go-ruby-date/date), [`did-you-mean`](https://github.com/go-ruby-did-you-mean/did-you-mean), [`digest`](https://github.com/go-ruby-digest/digest), [`erb`](https://github.com/go-ruby-erb/erb), [`find`](https://github.com/go-ruby-find/find), [`format`](https://github.com/go-ruby-format/format), [`getoptlong`](https://github.com/go-ruby-getoptlong/getoptlong), [`i18n`](https://github.com/go-ruby-i18n/i18n), [`ipaddr`](https://github.com/go-ruby-ipaddr/ipaddr), [`json`](https://github.com/go-ruby-json/json), [`logger`](https://github.com/go-ruby-logger/logger), [`marshal`](https://github.com/go-ruby-marshal/marshal), [`matrix`](https://github.com/go-ruby-matrix/matrix), [`mime-types`](https://github.com/go-ruby-mime-types/mime-types), [`minitest`](https://github.com/go-ruby-minitest/minitest), [`msgpack`](https://github.com/go-ruby-msgpack/msgpack), [`net-ftp`](https://github.com/go-ruby-net-ftp/net-ftp), [`net-http`](https://github.com/go-ruby-net-http/net-http), [`net-pop`](https://github.com/go-ruby-net-pop/net-pop), [`net-s3`](https://github.com/go-ruby-net-s3/net-s3), [`net-sftp`](https://github.com/go-ruby-net-sftp/net-sftp), [`net-smtp`](https://github.com/go-ruby-net-smtp/net-smtp), [`observer`](https://github.com/go-ruby-observer/observer), [`openssl`](https://github.com/go-ruby-openssl/openssl), [`optparse`](https://github.com/go-ruby-optparse/optparse), [`ostruct`](https://github.com/go-ruby-ostruct/ostruct), [`parser`](https://github.com/go-ruby-parser/parser), [`pathname`](https://github.com/go-ruby-pathname/pathname), [`prettyprint`](https://github.com/go-ruby-prettyprint/prettyprint), [`prime`](https://github.com/go-ruby-prime/prime), [`pstore`](https://github.com/go-ruby-pstore/pstore), [`public-suffix`](https://github.com/go-ruby-public-suffix/public-suffix), [`racc`](https://github.com/go-ruby-racc/racc), [`rack`](https://github.com/go-ruby-rack/rack), [`rake`](https://github.com/go-ruby-rake/rake), [`rational`](https://github.com/go-ruby-rational/rational), [`regexp`](https://github.com/go-ruby-regexp/regexp), [`reline`](https://github.com/go-ruby-reline/reline), [`resolv`](https://github.com/go-ruby-resolv/resolv), [`rexml`](https://github.com/go-ruby-rexml/rexml), [`rss`](https://github.com/go-ruby-rss/rss), [`rubygems`](https://github.com/go-ruby-rubygems/rubygems), [`scanf`](https://github.com/go-ruby-scanf/scanf), [`securerandom`](https://github.com/go-ruby-securerandom/securerandom), [`set`](https://github.com/go-ruby-set/set), [`shellwords`](https://github.com/go-ruby-shellwords/shellwords), [`sinatra`](https://github.com/go-ruby-sinatra/sinatra), [`stringio`](https://github.com/go-ruby-stringio/stringio), [`strscan`](https://github.com/go-ruby-strscan/strscan), [`syslog`](https://github.com/go-ruby-syslog/syslog), [`time`](https://github.com/go-ruby-time/time), [`toml`](https://github.com/go-ruby-toml/toml), [`tsort`](https://github.com/go-ruby-tsort/tsort), [`unicode-normalize`](https://github.com/go-ruby-unicode-normalize/unicode-normalize), [`uri`](https://github.com/go-ruby-uri/uri), [`webrick`](https://github.com/go-ruby-webrick/webrick), [`yaml`](https://github.com/go-ruby-yaml/yaml), [`zlib`](https://github.com/go-ruby-zlib/zlib) — each at `github.com/go-ruby-<name>/<name>`.

The scientific / container stack ([go-ndarray](https://github.com/go-ndarray/ndarray),
[go-fft](https://github.com/go-fft/fft), [go-images](https://github.com/go-images/images),
[go-composites](https://github.com/go-composites)) binds in the same way.

## How it differs from goruby

There is an existing Go project also called [goruby][goruby]. go-embedded-ruby
takes a deliberately different path:

- **Bytecode VM, not an AST-walking interpreter.** Source is compiled to a YARV
  -style instruction sequence (ISeq) and executed on a stack machine, rather
  than re-walking the syntax tree on every call.
- **Single, tree-shaken static binary.** `rbgo build` selects only the reached
  stdlib and links one self-contained executable.
- **Dynamic `eval` / `require` via an embedded front-end.** The full compiler
  pipeline is in the binary, so code can be parsed and compiled at runtime.
- **Ruby 4.0 as the target.** Behaviour is checked against MRI rather than
  approximated.

## Where to go next

- [Architecture overview](architecture/index.md) — the compilation pipeline and
  the packages that make it up.
- [Roadmap (phases)](roadmap.md) — the nine phases from vertical slice to
  conformance and performance.
- [Phase 0 — Vertical slice](phases/phase0.md) — what runs today and how it is
  tested.

Source lives at
[github.com/go-embedded-ruby/ruby](https://github.com/go-embedded-ruby/ruby).

[go-mruby]: https://github.com/mitchellh/go-mruby
[go-ruby-regexp]: https://github.com/go-ruby-regexp
[go-ruby-marshal]: https://github.com/go-ruby-marshal/marshal
[go-ruby-parser]: https://github.com/go-ruby-parser/parser
[goruby]: https://github.com/goruby/goruby
