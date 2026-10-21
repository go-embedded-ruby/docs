<p align="center"><img src="https://raw.githubusercontent.com/go-embedded-ruby/brand/main/social/go-embedded-ruby.png" alt="go-embedded-ruby/docs" width="720"></p>

# go-embedded-ruby/docs

Versioned documentation for [go-embedded-ruby](https://github.com/go-embedded-ruby),
built with [Hugo](https://gohugo.io/) and this fleet's [Hextra fork](https://github.com/tannevaled/hextra) — upstream Hextra plus the opt-in features the fleet's sites use. Published to the
`gh-pages` branch and served at <https://go-embedded-ruby.github.io/docs/>.

The organization landing page ([go-embedded-ruby.github.io](https://go-embedded-ruby.github.io))
links here.

## Local preview

```bash
git submodule update --init --recursive   # themes/hextra, pinned at a release tag
hugo server                               # http://localhost:1313
```

## What the build checks

The pages state figures, print code and attribute libraries, and all three go
stale silently. Four Go tools under `tools/` run in the `build` job — which also
covers pull requests, so a wrong figure blocks the change instead of being found
after publication.

| tool | what it judges | its authority |
|---|---|---|
| `conformancefigure` | every **present-tense** claim about the ruby/spec total. A *dated* sweep going stale is not an error, and is exempt by its date. | the gate's own `BASELINE` |
| `docexample` | every fenced `ruby` block that claims an answer in a `# =>` comment is **run**, and the lines it prints compared in order. A block that claims nothing is skipped. | a real interpreter |
| `numericsurface` | every method name the prose **prints** in a module's section, and the counts the pages state. | a real interpreter |
| `upstreamlinks` | every **library a module section links to** is one rbgo still depends on. | rbgo's `go.mod` |

```bash
GOBIN=/tmp go install github.com/go-embedded-ruby/ruby/cmd/rbgo@main

go run ./tools/conformancefigure --root content/en
go run ./tools/docexample        --rbgo /tmp/rbgo --root content/en
go run ./tools/numericsurface    --rbgo /tmp/rbgo --root content/en
go run ./tools/upstreamlinks     --root content/en
```

The authority column is the point. None of these holds the right answer itself:
a table of right answers kept in **this** repository would repeat the page's
mistake and agree with it. `upstreamlinks` asks `go.mod` precisely because a
library that gets swapped out **disappears** from it, which is what made all
three stale attributions of 2026-10-20 detectable without anybody writing the
correct one down.

⚠ **All four use [cobra](https://github.com/spf13/cobra) since `v0.4.0`, so
every flag is POSIX `--long`.** `-root` is no longer accepted by any of them:
`pflag` reads it as the cluster `-r -o -o -t`. A caller pinned to `v0.3.0` or
earlier keeps working until it bumps, and must change the spelling when it does.

The section reader and the module table they share live in
[`internal/surface`](internal/surface), so the two guards cannot end up
disagreeing about which sections exist.

Each refuses rather than passes when it cannot judge: no interpreter, no
Markdown read, no section found. A guard that fails open teaches you to trust a
silence it did not earn.

⛔ `go test ./...` holds the rest: `tools/wired` fails when a tool under
`tools/` is named by no workflow, because **a guard that is not wired is
indistinguishable from one that passes** — both are silent. The move to Hugo
dropped two of them for two releases in exactly that way.

## Releasing a new docs version

Each documented version is its own build under `/docs/<version>/`, and
`/docs/latest/` follows the newest — which is what `mike` did before. The version
comes from `params.docs.version` in `hugo.yaml`: set it to the release the pages
describe (`v0.16.0` → `/docs/0.16/`), or leave it at `main` while no release is
tagged. The `docs` workflow reads it, builds with that `--baseURL`, and replaces
**only** that version's path.

⛔ The build runs with `--panicOnWarning`, so a link to a page that is not there
fails the lane instead of 404ing for a reader.
