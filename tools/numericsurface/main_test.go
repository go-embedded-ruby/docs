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

	"github.com/go-embedded-ruby/docs/internal/surface"
)

// The probe is GENERATED from the module table, so a module added there cannot
// be forgotten in it -- which is the failure that would make a whole module's
// section silently unchecked.
func TestProbeAsksAboutEveryModule(t *testing.T) {
	p := probe(surface.Modules)
	for _, m := range surface.Modules {
		for _, k := range m.Konsts {
			if !strings.Contains(p, k+".singleton_methods") {
				t.Errorf("the probe does not ask about %s", k)
			}
		}
		for _, inst := range m.Instances {
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
	var shared *surface.Module
	for i := range surface.Modules {
		if len(surface.Modules[i].Konsts) > 1 {
			shared = &surface.Modules[i]
			break
		}
	}
	if shared == nil {
		t.Skip("no section covers more than one constant")
	}
	p := probe(surface.Modules)
	for _, k := range shared.Konsts {
		want := k + ".singleton_methods.each { |n| puts \"" + shared.Name() + "\\tsingleton"
		if !strings.Contains(p, want) {
			t.Errorf("%s does not report under the shared name %q", k, shared.Name())
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
	t.Setenv("NUMERICSURFACE_HELPER_ANSWER", answer)
	return exe
}

// TestMain is the helper mode: when the environment names an answer, the binary
// prints it and exits, which is all an interpreter has to do here.
func TestMain(m *testing.M) {
	if answer, ok := os.LookupEnv("NUMERICSURFACE_HELPER_ANSWER"); ok && os.Getenv("NUMERICSURFACE_HELPER_ACTIVE") == "1" {
		os.Stdout.WriteString(answer)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// countPhrase gives, for each `Counted` pattern in the table, a sentence that
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
// anybody remembering to extend a literal -- and the run refuses outright when a
// module answers nothing, so a literal would have made every later addition
// fail for the wrong reason.
func fixture(t *testing.T) (answer, page string) {
	t.Helper()
	var a, p strings.Builder
	for i, m := range surface.Modules {
		meth := fmt.Sprintf("m%d", i)
		fmt.Fprintf(&a, "%s\tsingleton\t%s\n", m.Name(), meth)
		fmt.Fprintf(&p, "## %s\n\n`%s`", m.Heading, meth)
		if m.Counted != "" {
			phrase, ok := countPhrase[m.Counted]
			if !ok {
				t.Fatalf("the table counts %q but this test has no sentence for that pattern", m.Counted)
			}
			fmt.Fprintf(&p, ", %s", phrase(1))
		}
		p.WriteString("\n\n")
	}
	return a.String(), p.String()
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
