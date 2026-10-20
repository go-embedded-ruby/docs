package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ⛔ The extraction is a VOCABULARY, and a vocabulary nobody tested is a guess.
// A guess that matches nothing looks exactly like a clean site, which is the
// one way this tool could go quiet about something that matters.
func TestExtraction(t *testing.T) {
	cases := []struct {
		name  string
		md    string
		want  string // the heading substring to look for
		names []string
		count int // -1 when the section states none
		patt  string
	}{
		{
			name:  "a section's backticked method names are collected in order",
			md:    "## Special — functions\n\nGamma (`gamma`/`gammaln`) and `erf`.\n\n## Stats\n\n`norm`\n",
			want:  "Special",
			names: []string{"gamma", "gammaln", "erf"},
			count: -1,
		},
		{
			name:  "the next heading ends the section",
			md:    "## Stats\n\n`norm`\n\n## Integrate\n\n`quad`\n",
			want:  "Stats",
			names: []string{"norm"},
			count: -1,
		},
		{
			name:  "a keyword argument is not a method name",
			md:    "## Integrate\n\n`quad` with `points:` for a kink, `x:` or `dx:`.\n",
			want:  "Integrate",
			names: []string{"quad"},
			count: -1,
		},
		{
			name:  "a return shape and a Go name are not method names",
			md:    "## Special\n\n`(value, abserr)`, and `Log1pExp` is `softplus`.\n",
			want:  "Special",
			names: []string{"softplus"},
			count: -1,
		},
		{
			name:  "a path and a command are not method names",
			md:    "## Special\n\n`bench/scipy_server.py`, `testdata/gen/fam_*.py`, `rbgo build --closed`.\n",
			want:  "Special",
			names: nil,
			count: -1,
		},
		{
			name:  "a repeated name is collected once",
			md:    "## FFT\n\n`fft` and again `fft`.\n",
			want:  "FFT",
			names: []string{"fft"},
			count: -1,
		},
		{
			name:  "a stated count is read",
			md:    "## Special\n\nall 67 of its functions under 66 Ruby names.\n",
			want:  "Special",
			names: nil,
			count: 66,
			patt:  `(\d+) Ruby names`,
		},
		{
			// ⛔ The page also says "63 of the 67 carry such a pairing". A
			// looser pattern would read that as the count and report a
			// disagreement nobody wrote.
			name:  "the count pattern misses a figure that is about something else",
			md:    "## Special\n\n63 of the 67 carry such a pairing.\n",
			want:  "Special",
			names: nil,
			count: -1,
			patt:  `(\d+) Ruby names`,
		},
		{
			name:  "a missing section is not an empty one",
			md:    "## Stats\n\n`norm`\n",
			want:  "Optimize",
			names: nil,
			count: -1,
		},
		{
			// A Windows checkout serves CRLF; a `$`-anchored reading of it
			// silently matches nothing, which has bitten this fleet before.
			name:  "CRLF is folded, as a Windows checkout serves it",
			md:    "## Stats\r\n\r\n`norm` and `pdf`.\r\n",
			want:  "Stats",
			names: []string{"norm", "pdf"},
			count: -1,
		},
		{
			name:  "a predicate keeps its question mark",
			md:    "## Stats\n\n`empty?` and `add!`.\n",
			want:  "Stats",
			names: []string{"empty?", "add!"},
			count: -1,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := sectionFor([]byte(c.md), c.want)
			got := namesIn(body)
			if strings.Join(got, " ") != strings.Join(c.names, " ") {
				t.Errorf("names %v, want %v", got, c.names)
			}
			n, ok := countIn(body, c.patt)
			switch {
			case c.count < 0 && ok:
				t.Errorf("read a count of %d, want none", n)
			case c.count >= 0 && (!ok || n != c.count):
				t.Errorf("count %d (found=%v), want %d", n, ok, c.count)
			}
		})
	}
}

// The probe is GENERATED from the table, so a module added to the table cannot
// be forgotten in it -- which is the failure that would make a whole module's
// section silently unchecked.
func TestProbeAsksAboutEveryModule(t *testing.T) {
	p := probe(modules)
	for _, m := range modules {
		for _, k := range m.konsts {
			if !strings.Contains(p, k+".singleton_methods") {
				t.Errorf("the probe does not ask about %s", k)
			}
		}
		for _, inst := range m.instances {
			if !strings.Contains(p, "("+inst+").methods") {
				t.Errorf("the probe does not ask about the instance %s", inst)
			}
		}
	}
}

// A section that covers two constants must POOL their names: checking each
// against the shared prose reports the other's names as unknown. "Time & Date"
// is the real case -- `leap?` and `next_day` are Date's, `at` is Time's, and a
// reader meets all three in one section.
func TestASharedSectionPoolsItsConstants(t *testing.T) {
	var shared *module
	for i := range modules {
		if len(modules[i].konsts) > 1 {
			shared = &modules[i]
			break
		}
	}
	if shared == nil {
		t.Skip("no section covers more than one constant")
	}
	p := probe(modules)
	for _, k := range shared.konsts {
		want := k + ".singleton_methods.each { |n| puts \"" + shared.name() + "\\tsingleton"
		if !strings.Contains(p, want) {
			t.Errorf("%s does not report under the shared name %q", k, shared.name())
		}
	}
}

// Every exemption carries a reason, because a bare list becomes a way of
// silencing the tool rather than a record of a judgement.
func TestEveryExemptionGivesAReason(t *testing.T) {
	for _, m := range modules {
		for word, why := range m.prose {
			if strings.TrimSpace(why) == "" {
				t.Errorf("%s exempts %q with no reason", m.name(), word)
			}
		}
	}
}

// ⛔ A refusal must not read as a pass. Each of these exits 2 -- "I could not
// judge" -- and none of them reaches the walk.
func TestRefusals(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"no interpreter given", []string{"--root", "testdata"}, 2},
		{"the interpreter is not there", []string{"--rbgo", "/nonexistent/rbgo", "--root", "testdata"}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := RootCmd()
			cmd.SetArgs(c.args)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			err := cmd.Execute()
			var ee *exitError
			if !errors.As(err, &ee) {
				t.Fatalf("error %v, want an exitError", err)
			}
			if ee.code != c.want {
				t.Errorf("status %d, want %d", ee.code, c.want)
			}
		})
	}
}

// A positional argument is a mistake, not an extra root: the flag is --root.
func TestAnArgumentIsRefused(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"content/en"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("a positional argument was accepted")
	}
}

// fakeRbgo makes the TEST BINARY serve as the interpreter: ask() runs it like
// any other program, so this exercises the real exec path, the real parsing of
// its answer and the real verdict, without building an interpreter.
func fakeRbgo(t *testing.T, answer string) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("cannot find the test binary: %v", err)
	}
	// A shim that re-runs this binary in helper mode is not available without
	// a shell, so the helper is selected by the environment instead.
	t.Setenv("NUMERICSURFACE_HELPER_ANSWER", answer)
	return exe
}

// TestHelperHarness is the helper mode: when the environment names an answer,
// the binary prints it and exits, which is all an interpreter has to do here.
func TestMain(m *testing.M) {
	if answer, ok := os.LookupEnv("NUMERICSURFACE_HELPER_ANSWER"); ok && os.Getenv("NUMERICSURFACE_HELPER_ACTIVE") == "1" {
		os.Stdout.WriteString(answer)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// ⛔ Verified to BITE. A tool that reports "0 disagree" on a page it cannot
// read is the defect this file exists to refuse, so the break cases assert the
// message as well as the status.
func TestItBites(t *testing.T) {
	answer, clean := fixture(t)

	cases := []struct {
		name     string
		md       string
		wantCode int // 0 means Execute returns nil
		wantMsg  string
	}{
		{"the pages agree with the interpreter", clean, 0, ""},
		{
			name:     "a name the interpreter does not answer",
			md:       strings.Replace(clean, "`m0`", "`m0` and `nosuchfunc`", 1),
			wantCode: 1,
			wantMsg:  "nosuchfunc",
		},
		{
			name:     "a stated count that is not the one the module has",
			md:       strings.Replace(clean, "1 Ruby names", "9 Ruby names", 1),
			wantCode: 1,
			wantMsg:  "states 9, the interpreter answers 1",
		},
		{
			name:     "a root holding no module section at all",
			md:       "# nothing here\n",
			wantCode: 3,
			wantMsg:  "found no documented module surface",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "page.md"), []byte(c.md), 0o644); err != nil {
				t.Fatal(err)
			}
			rbgo := fakeRbgo(t, answer)
			t.Setenv("NUMERICSURFACE_HELPER_ACTIVE", "1")

			var out, errb bytes.Buffer
			cmd := RootCmd()
			cmd.SetArgs([]string{"--rbgo", rbgo, "--root", dir})
			cmd.SetOut(&out)
			cmd.SetErr(&errb)
			err := cmd.Execute()

			if c.wantCode == 0 {
				if err != nil {
					t.Fatalf("Execute: %v\nstderr: %s", err, errb.String())
				}
				if !strings.Contains(out.String(), "0 disagree") {
					t.Errorf("stdout %q, want it to report 0 disagreements", out.String())
				}
				return
			}
			var ee *exitError
			if !errors.As(err, &ee) {
				t.Fatalf("error %v, want an exitError\nstdout: %s\nstderr: %s", err, out.String(), errb.String())
			}
			if ee.code != c.wantCode {
				t.Errorf("status %d, want %d", ee.code, c.wantCode)
			}
			said := errb.String() + ee.msg
			if !strings.Contains(said, c.wantMsg) {
				t.Errorf("it said %q, want it to name %q", said, c.wantMsg)
			}
		})
	}
}

// countPhrase gives, for each `counted` pattern in the table, a sentence that
// pattern must read. A module whose pattern has no phrase here FAILS the
// fixture: otherwise a count could be added to the table and never exercised,
// and an unexercised pattern is a guess -- which is how the site ended up
// stating three different conformance totals in the first place.
var countPhrase = map[string]func(int) string{
	`(\d+) Ruby names`:               func(n int) string { return fmt.Sprintf("%d Ruby names", n) },
	`(\d+) univariate distributions`: func(n int) string { return fmt.Sprintf("%d univariate distributions", n) },
}

// fixture builds, FROM THE TABLE, an interpreter answer and a page that agree:
// one invented method per section, plus the stated count where a section states
// one. Generating it means a module added to the table is covered here without
// anybody remembering to extend a literal -- and the run refuses outright if a
// module answers nothing, so a literal would have made every later addition
// fail for the wrong reason.
func fixture(t *testing.T) (answer, page string) {
	t.Helper()
	var a, p strings.Builder
	for i, m := range modules {
		meth := fmt.Sprintf("m%d", i)
		fmt.Fprintf(&a, "%s\tsingleton\t%s\n", m.name(), meth)
		fmt.Fprintf(&p, "## %s\n\n`%s`", m.heading, meth)
		if m.counted != "" {
			phrase, ok := countPhrase[m.counted]
			if !ok {
				t.Fatalf("the table counts %q but this test has no sentence for that pattern", m.counted)
			}
			fmt.Fprintf(&p, ", %s", phrase(1))
		}
		p.WriteString("\n\n")
	}
	return a.String(), p.String()
}

// The helper mode must actually be reachable, or every case above would be
// testing a path nobody takes. A planted positive proves the harness works.
func TestTheFakeInterpreterIsReallyRun(t *testing.T) {
	rbgo := fakeRbgo(t, "HELLO\n")
	cmd := exec.Command(rbgo, "ignored.rb")
	cmd.Env = append(os.Environ(), "NUMERICSURFACE_HELPER_ACTIVE=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running the fake interpreter: %v", err)
	}
	if string(out) != "HELLO\n" {
		t.Errorf("it printed %q, want %q", out, "HELLO\n")
	}
}
