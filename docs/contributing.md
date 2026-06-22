# Contributing

Contributions are welcome. go-embedded-ruby is built to a small set of
non-negotiable rules — they are what keep the project pure-Go, correct, and
self-contained. Please read these before opening a pull request.

## Hard rules

- **Build from source — no vendoring.** Everything compiles from source. Do not
  reach for prebuilt binaries or vendored blobs as a shortcut; being able to
  compile from source is a guarantee of independence.
- **100% test coverage target, enforced in CI.** New code ships with tests, and
  coverage is a CI gate. Fill the error branches, not just the happy path.
- **All GitHub content in English.** Issues, pull requests, commits, comments,
  and discussions are English-only.
- **Behaviour verified against an MRI oracle and ruby/spec.** Correctness is
  defined by reference Ruby. New behaviour is **differential-tested** against MRI
  and checked against ruby/spec where applicable — not approximated from memory.
- **Pure Go, cgo disabled.** The whole point is a single static binary with no C
  toolchain. Code must build with `CGO_ENABLED=0`. If a feature seems to need C,
  it needs a pure-Go path instead (as the regexp engine does in the
  [go-ruby-regexp](https://github.com/go-ruby-regexp) sibling org).

## Workflow

1. Pick or open an issue describing the change.
2. Work test-first: add the differential / unit tests, then make them pass.
3. Run the full suite with coverage and confirm the gate is green.
4. Open a PR in English, referencing the issue.

## Where things live

The interpreter — VM, front-end, and the `rbgo` CLI — is in
[`github.com/go-embedded-ruby/ruby`](https://github.com/go-embedded-ruby/ruby).
This documentation site is in
[`github.com/go-embedded-ruby/docs`](https://github.com/go-embedded-ruby/docs).
Start from the [Architecture overview](architecture/index.md) and the
[Roadmap](roadmap.md) to find the right place for your change.
