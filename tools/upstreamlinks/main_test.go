package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-embedded-ruby/docs/internal/surface"
)

// A go.mod in every shape this has to read: a block, a single-line require, an
// indirect marker and a comment.
const goMod = `module github.com/go-embedded-ruby/ruby

go 1.27.1

require github.com/go-fft/fft v0.23.0

require (
	github.com/go-composites/bag v0.0.0-20261001005725-fcf324b93f95
	github.com/go-composites/time v0.0.0-20260929013028-aa3c57ae1371 // a comment
	github.com/go-ndarray/ndarray v0.12.0
	github.com/go-ruby-bigdecimal/bigdecimal v0.0.0-20260916091144-b31c0a1caac2
)

require (
	github.com/go-composites/array v0.0.0-20260929012314-b34dd2208c46 // indirect
	github.com/go-images/png v0.1.0 // indirect
)
`

func TestParse(t *testing.T) {
	d, err := parse("go.mod", []byte(goMod))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if d.main != "github.com/go-embedded-ruby/ruby" {
		t.Errorf("main module %q", d.main)
	}
	for _, want := range []string{
		"github.com/go-fft/fft",                    // single-line require
		"github.com/go-composites/bag",             // in a block
		"github.com/go-composites/time",            // with a trailing comment
		"github.com/go-ruby-bigdecimal/bigdecimal", // the real replacement
	} {
		if !d.direct[want] {
			t.Errorf("%s is not read as a direct requirement", want)
		}
	}
	// ⛔ An INDIRECT dependency is not evidence that a binding exists, and
	// accepting one would be the quiet direction to be wrong in: it would let
	// a stale attribution pass whenever the old library is still reachable
	// through something else.
	for _, no := range []string{"github.com/go-composites/array", "github.com/go-images/png"} {
		if d.direct[no] {
			t.Errorf("%s is indirect but was read as direct", no)
		}
	}
}

// A go.mod that parses but says nothing is a refusal, not an empty answer:
// otherwise every link on the page would be reported as stale at once.
func TestParseRefusesAFileWithNoRequirement(t *testing.T) {
	if _, err := parse("go.mod", []byte("module example.com/x\n\ngo 1.27.1\n")); err == nil {
		t.Fatal("a go.mod with no requirement was accepted")
	}
}

func TestParseRefusesSomethingElse(t *testing.T) {
	if _, err := parse("go.mod", []byte("<!DOCTYPE html>\n<html>404</html>\n")); err == nil {
		t.Fatal("an HTML error page was accepted as a go.mod")
	}
}

// ⛔ Verified to BITE, on the three real cases of 2026-10-20 and on the nine
// that were right.
func TestItBites(t *testing.T) {
	gomodPath := filepath.Join(t.TempDir(), "go.mod")
	if err := os.WriteFile(gomodPath, []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}

	// A page naming every module section, so none of them is missing for the
	// wrong reason; only the sections under test carry a link.
	page := func(extra string) string {
		var b strings.Builder
		for _, m := range surface.Modules {
			b.WriteString("## " + m.Heading + "\n\n")
			switch m.Heading {
			case "FFT":
				b.WriteString("[go-fft](https://github.com/go-fft/fft)\n\n")
			case "Bag":
				b.WriteString("[go-composites/bag](https://github.com/go-composites/bag)\n\n")
			case "BigDecimal":
				b.WriteString(extra + "\n\n")
			default:
				b.WriteString("nothing here\n\n")
			}
		}
		return b.String()
	}

	cases := []struct {
		name     string
		link     string
		wantCode int
		wantMsg  string
	}{
		{
			name: "the library rbgo really depends on",
			link: "[x](https://github.com/go-ruby-bigdecimal/bigdecimal)",
		},
		{
			name:     "the library it was swapped away from",
			link:     "[x](https://github.com/go-composites/bigfloat)",
			wantCode: 1,
			wantMsg:  "github.com/go-composites/bigfloat, which rbgo does not depend on",
		},
		{
			name:     "a library reachable only indirectly is not a binding",
			link:     "[x](https://github.com/go-images/png)",
			wantCode: 1,
			wantMsg:  "github.com/go-images/png",
		},
		{
			name: "a link to rbgo itself is not a claim about an upstream",
			link: "[web/](https://github.com/go-embedded-ruby/ruby/tree/main/web)",
		},
		{
			name: "a section that links nothing claims nothing",
			link: "It binds no outside library.",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "page.md"), []byte(page(c.link)), 0o644); err != nil {
				t.Fatal(err)
			}
			var out, errb bytes.Buffer
			cmd := RootCmd()
			cmd.SetArgs([]string{"--root", dir, "--gomod", gomodPath})
			cmd.SetOut(&out)
			cmd.SetErr(&errb)
			err := cmd.Execute()

			if c.wantCode == 0 {
				if err != nil {
					t.Fatalf("Execute: %v\nstderr: %s", err, errb.String())
				}
				if !strings.Contains(out.String(), "0 disagree") {
					t.Errorf("stdout %q, want 0 disagreements", out.String())
				}
				return
			}
			var ee *exitError
			if !errors.As(err, &ee) {
				t.Fatalf("error %v, want an exitError\nstderr: %s", err, errb.String())
			}
			if ee.code != c.wantCode {
				t.Errorf("status %d, want %d", ee.code, c.wantCode)
			}
			if said := errb.String(); !strings.Contains(said, c.wantMsg) {
				t.Errorf("it said %q, want it to name %q", said, c.wantMsg)
			}
		})
	}
}

// ⛔ A page with sections but no link at all exits 3 rather than reporting a
// clean run: for a site that attributes every module, that means the extraction
// broke.
func TestNoLinkAtAllIsNotCleanliness(t *testing.T) {
	gomodPath := filepath.Join(t.TempDir(), "go.mod")
	if err := os.WriteFile(gomodPath, []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var b strings.Builder
	for _, m := range surface.Modules {
		b.WriteString("## " + m.Heading + "\n\nno links here\n\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "page.md"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	cmd := RootCmd()
	cmd.SetArgs([]string{"--root", dir, "--gomod", gomodPath})
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute()
	var ee *exitError
	if !errors.As(err, &ee) || ee.code != 3 {
		t.Fatalf("error %v, want status 3", err)
	}
}

// An unreadable go.mod is a refusal (2), never a pass: a dependency set nobody
// could read finds no stale link either, and that silence reads as agreement.
func TestAnUnreadableGoModRefuses(t *testing.T) {
	var out, errb bytes.Buffer
	cmd := RootCmd()
	cmd.SetArgs([]string{"--root", "testdata", "--gomod", "/nonexistent/go.mod"})
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute()
	var ee *exitError
	if !errors.As(err, &ee) || ee.code != 2 {
		t.Fatalf("error %v, want status 2", err)
	}
}

// The fetch has a timeout at all, and it is the flag's. The run before this
// tool existed failed on sum.golang.org dropping a stream, so a hang here must
// end as a refusal rather than as a lane that never finishes.
func TestTheTimeoutIsUsed(t *testing.T) {
	var out, errb bytes.Buffer
	cmd := RootCmd()
	cmd.SetArgs([]string{"--root", "testdata", "--timeout", "1ns"})
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	start := time.Now()
	err := cmd.Execute()
	var ee *exitError
	if !errors.As(err, &ee) || ee.code != 2 {
		t.Fatalf("error %v, want status 2", err)
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Errorf("took %v; the timeout flag did not reach the client", elapsed)
	}
}
