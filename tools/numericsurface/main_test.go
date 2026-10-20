package main

import (
	"bytes"
	"errors"
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
		if !strings.Contains(p, m.konst+".singleton_methods") {
			t.Errorf("the probe does not ask about %s", m.konst)
		}
		for _, inst := range m.instances {
			if !strings.Contains(p, "("+inst+").methods") {
				t.Errorf("the probe does not ask about the instance %s", inst)
			}
		}
	}
}

// Every exemption carries a reason, because a bare list becomes a way of
// silencing the tool rather than a record of a judgement.
func TestEveryExemptionGivesAReason(t *testing.T) {
	for _, m := range modules {
		for word, why := range m.prose {
			if strings.TrimSpace(why) == "" {
				t.Errorf("%s exempts %q with no reason", m.konst, word)
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
	const answer = "Special\tsingleton\tgamma\n" +
		"Special\tsingleton\terf\n" +
		"Stats\tsingleton\tnorm\n" +
		"Stats\tinstance\tpdf\n" +
		"Integrate\tsingleton\tquad\n" +
		"Optimize\tsingleton\tbrentq\n" +
		"NDArray\tsingleton\tarange\n" +
		"FFT\tsingleton\tfft\n"

	// One page naming every module, so no module makes the run refuse for the
	// wrong reason.
	const clean = "## Special\n\n`gamma` and `erf`, 2 Ruby names.\n\n" +
		"## Stats\n\n`norm`, `pdf`, the 1 univariate distributions.\n\n" +
		"## Integrate\n\n`quad`\n\n## Optimize\n\n`brentq`\n\n" +
		"## NDArray\n\n`arange`\n\n## FFT\n\n`fft`\n"

	cases := []struct {
		name     string
		md       string
		wantCode int // 0 means Execute returns nil
		wantMsg  string
	}{
		{"the pages agree with the interpreter", clean, 0, ""},
		{
			name:     "a name the interpreter does not answer",
			md:       strings.Replace(clean, "`gamma` and `erf`", "`gamma`, `erf` and `nosuchfunc`", 1),
			wantCode: 1,
			wantMsg:  "nosuchfunc",
		},
		{
			name:     "a stated count that is not the one the module has",
			md:       strings.Replace(clean, "2 Ruby names", "9 Ruby names", 1),
			wantCode: 1,
			wantMsg:  "states 9, the interpreter answers 2",
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
