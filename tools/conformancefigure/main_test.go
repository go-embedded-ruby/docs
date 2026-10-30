package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The vocabulary agrees with itself today. This is the same control the tool
// runs on every invocation; having it here as well means a failure is reported
// per case by `go test` rather than as one line from a CI step.
func TestTheVocabularyIsRight(t *testing.T) {
	if err := checkVocabulary(); err != nil {
		t.Fatalf("checkVocabulary: %v", err)
	}
	if len(vocabularyCases) < 8 {
		t.Errorf("only %d vocabulary case(s); the control is thinner than the two phrasings it has to cover", len(vocabularyCases))
	}
}

// ⛔ And it BITES. A control that cannot fail is the defect it exists to refuse:
// a wrong vocabulary matches nothing, and matching nothing looks exactly like a
// site with no stale figures.
func TestTheVocabularyControlBites(t *testing.T) {
	saved := vocabularyCases
	t.Cleanup(func() { vocabularyCases = saved })

	t.Run("a line that should match and does not", func(t *testing.T) {
		vocabularyCases = []struct {
			line string
			want bool
			n    string
		}{{line: "nothing here resembles a total", want: true, n: "23,511"}}
		if err := checkVocabulary(); err == nil {
			t.Fatal("the control passed on a case it cannot match")
		}
	})

	t.Run("a line read as the wrong figure", func(t *testing.T) {
		vocabularyCases = []struct {
			line string
			want bool
			n    string
		}{{line: "It passes **23,511** of ruby/spec's", want: true, n: "99,999"}}
		if err := checkVocabulary(); err == nil {
			t.Fatal("the control passed on a figure it read differently")
		}
	})
}

// ⛔ An unreadable baseline is a refusal (2), never a pass: it finds no
// disagreement either, and that silence reads exactly like agreement.
func TestAnUnreadableBaselineRefuses(t *testing.T) {
	var out, errb bytes.Buffer
	cmd := RootCmd()
	cmd.SetArgs([]string{"--root", "testdata", "--baseline", "/nonexistent/BASELINE"})
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute()
	var ee *exitError
	if !errors.As(err, &ee) || ee.code != 2 {
		t.Fatalf("error %v, want status 2", err)
	}
}

// A baseline with no OK rows sums to 0, and 0 is a number: it would be compared
// against the pages and reported as a disagreement with a confident figure.
func TestABaselineWithNoRowsRefuses(t *testing.T) {
	p := filepath.Join(t.TempDir(), "BASELINE")
	if err := os.WriteFile(p, []byte("# a comment and nothing else\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	cmd := RootCmd()
	cmd.SetArgs([]string{"--root", "testdata", "--baseline", p})
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute()
	var ee *exitError
	if !errors.As(err, &ee) || ee.code != 2 {
		t.Fatalf("error %v, want status 2", err)
	}
}

// ⛔ Verified to BITE on the figures themselves, with a baseline whose total is
// known: a page stating the wrong number exits 1 and names both, and a page
// stating none exits 1 too -- because for a site that states the total,
// matching nothing means the vocabulary stopped fitting, not that it is clean.
func TestItBites(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "BASELINE")
	if err := os.WriteFile(base, []byte("OK\ta_spec.rb\t20000\nOK\tb_spec.rb\t3511\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		md       string
		wantCode int
		wantMsg  string
	}{
		{
			name: "a page stating the baseline's own total",
			md:   "It passes **23,511** of ruby/spec's examples.\n",
		},
		{
			name:     "a page stating a different total",
			md:       "It passes **22,488** of ruby/spec's examples.\n",
			wantCode: 1,
			wantMsg:  "claims 22488, the gate records 23511",
		},
		{
			name:     "a page stating no total at all",
			md:       "nothing numeric here\n",
			wantCode: 1,
			wantMsg:  "found no statement of the total",
		},
		{
			name: "a DATED sweep going stale is not an error",
			md:   "It passes **23,511** today.\n\n**22,488 examples pass**, measured 2026-09-26 on main\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "page.md"), []byte(c.md), 0o644); err != nil {
				t.Fatal(err)
			}
			var out, errb bytes.Buffer
			cmd := RootCmd()
			cmd.SetArgs([]string{"--root", root, "--baseline", base, "--timeout", "5s"})
			cmd.SetOut(&out)
			cmd.SetErr(&errb)
			err := cmd.Execute()

			if c.wantCode == 0 {
				if err != nil {
					t.Fatalf("Execute: %v\nstdout: %s\nstderr: %s", err, out.String(), errb.String())
				}
				return
			}
			var ee *exitError
			if !errors.As(err, &ee) {
				t.Fatalf("error %v, want an exitError\nstdout: %s", err, out.String())
			}
			if ee.code != c.wantCode {
				t.Errorf("status %d, want %d", ee.code, c.wantCode)
			}
			if said := errb.String() + ee.msg; !bytes.Contains([]byte(said), []byte(c.wantMsg)) {
				t.Errorf("it said %q, want it to name %q", said, c.wantMsg)
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

// The timeout flag reaches the HTTP client, so a hang ends as a refusal rather
// than as a lane that never finishes.
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

// ⛔ A Hugo site keeps prose in its configuration, and until 2026-10-30 this
// tool could not see it. go-embedded-ruby.github.io stated the total three
// times: twice under layouts/, correct, because they were walked — and once in
// hugo.toml, thirty behind and drifting since 2026-10-04, because it was not.
// This is that file's shape.
func TestATomlFileIsWalked(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "BASELINE")
	if err := os.WriteFile(base, []byte("OK\ta_spec.rb\t23511\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	// A params.phases blurb, as the landing writes it.
	toml := "[[params.phases]]\n  name = \"Phase 7\"\n" +
		"  blurb = \"measured against ruby/spec: **23,481 examples pass** — the total that BASELINE sums to.\"\n"
	if err := os.WriteFile(filepath.Join(root, "hugo.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	cmd := RootCmd()
	cmd.SetArgs([]string{"--root", root, "--baseline", base, "--timeout", "5s"})
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute()

	var ee *exitError
	if !errors.As(err, &ee) {
		t.Fatalf("a stale figure in hugo.toml was not caught: %v\nstdout: %s", err, out.String())
	}
	if ee.code != 1 {
		t.Errorf("status %d, want 1", ee.code)
	}
	if said := errb.String(); !bytes.Contains([]byte(said), []byte("claims 23481")) {
		t.Errorf("it said %q, want it to name the stale figure", said)
	}
}
