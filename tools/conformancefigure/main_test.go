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

// versionFixtures writes the two files the version check reads, in the shape
// the ruby repository spells them.
//
// ⛔ Every test that gets as far as the version check must pass these. Without
// them the check fetches raw.githubusercontent.com, and a test that reaches the
// network is a test whose verdict depends on someone else's main branch and on
// having a connection — it read "at Ruby 3.4.1" from the live repository when
// this was first written, which is the right answer arrived at the wrong way.
func versionFixtures(t *testing.T, runtimeV, shimV string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	rp := filepath.Join(dir, "kernel_introspect.go")
	// A tab, because the pattern is anchored to the indented const block: the
	// file discusses `rubyVersion` in prose above it.
	body := "// the comment above mentions rubyVersion = \"9.9.9\" in prose\nconst (\n" +
		"\trubyVersion = \"" + runtimeV + "\"\n\treleaseDate = \"2024-12-25\"\n)\n"
	if err := os.WriteFile(rp, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	sp := filepath.Join(dir, "spec_helper.rb")
	if err := os.WriteFile(sp, []byte("CUR_VERSION = \""+shimV+"\"\nPLATFORMS = [:darwin]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return rp, sp
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
			md:   "It passes **23,511** of ruby/spec's examples, and those guards resolve at Ruby 3.4.1.\n",
		},
		{
			name:     "a page stating a different total",
			md:       "It passes **22,488** of ruby/spec's examples, and those guards resolve at Ruby 3.4.1.\n",
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
			md: "It passes **23,511** today, and those guards resolve at Ruby 3.4.1.\n\n" +
				"**22,488 examples pass**, measured 2026-09-26 on main\n",
		},
		// ⛔ The reason the disagreements are printed before the absences. Written
		// the other way round this case answered "found no statement of the
		// version" and never mentioned that the total was wrong by 1,023 — a new
		// check swallowing the finding of the old one.
		{
			name:     "a wrong total is named even when the version is not stated",
			md:       "It passes **22,488** of ruby/spec's examples.\n",
			wantCode: 1,
			wantMsg:  "claims 22488, the gate records 23511",
		},
		{
			name:     "a page stating the total but not the version it was measured at",
			md:       "It passes **23,511** of ruby/spec's examples.\n",
			wantCode: 1,
			wantMsg:  "found no statement of the version the gate measures at",
		},
		{
			name:     "a page stating a stale version",
			md:       "It passes **23,511** of ruby/spec's examples, and those guards resolve at Ruby 4.0.9.\n",
			wantCode: 1,
			wantMsg:  "says the gate measures at 4.0.9, it measures at 3.4.1",
		},
		// The oracle is a different claim and is named on these pages on purpose.
		// It must not be read as the version the gate measures at.
		{
			name: "the differential oracle may be newer than the gate's version",
			md: "It passes **23,511** of ruby/spec's examples, and those guards resolve at Ruby 3.4.1.\n\n" +
				"All behaviour is differential-tested against MRI Ruby 4.0.5.\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "page.md"), []byte(c.md), 0o644); err != nil {
				t.Fatal(err)
			}
			rp, sp := versionFixtures(t, "3.4.1", "3.4.1")
			var out, errb bytes.Buffer
			cmd := RootCmd()
			cmd.SetArgs([]string{"--root", root, "--baseline", base,
				"--runtime", rp, "--shim", sp, "--timeout", "5s"})
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

// ⛔ THE CASE THIS CHECK EXISTS FOR. ruby#799: the version is spelled twice,
// once in the runtime and once in the conformance shim, and "today they happen
// to agree" is the defect waiting to happen. A parity change that moves
// `rubyVersion` to 4.0.9 and forgets `CUR_VERSION` leaves the gate measuring the
// 3.4 population while every page claims 4.0.9 — and both halves look fine on
// their own. Reading both and REFUSING (2, not 1) is the only moment anything
// says so.
func TestTheTwoCopiesOfTheVersionMustAgree(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "BASELINE")
	if err := os.WriteFile(base, []byte("OK\ta_spec.rb\t23511\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "page.md"),
		[]byte("It passes **23,511**, and those guards resolve at Ruby 4.0.9.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The runtime moved, the shim did not.
	rp, sp := versionFixtures(t, "4.0.9", "3.4.1")
	var out, errb bytes.Buffer
	cmd := RootCmd()
	cmd.SetArgs([]string{"--root", root, "--baseline", base,
		"--runtime", rp, "--shim", sp, "--timeout", "5s"})
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute()

	var ee *exitError
	if !errors.As(err, &ee) {
		t.Fatalf("two disagreeing copies were accepted: %v\nstdout: %s", err, out.String())
	}
	// 2, not 1: it is a refusal to answer. With the two copies apart there is no
	// single version for the pages to be judged against, so calling the page
	// wrong would be picking one arbitrarily.
	if ee.code != 2 {
		t.Errorf("status %d, want 2", ee.code)
	}
	for _, want := range []string{"4.0.9", "3.4.1", "ruby#799"} {
		if !bytes.Contains([]byte(ee.msg), []byte(want)) {
			t.Errorf("the refusal %q does not name %q", ee.msg, want)
		}
	}
}

// ⛔ A claim a line break cut in half. This is the sentence this tool missed on
// its own first run over the real pages, verbatim: it reported two version
// claims where three had been written, and two looks exactly like a clean
// answer. Markdown prose is re-wrapped by every editor that touches it, so the
// scan must read across the break.
func TestAVersionClaimSplitAcrossLinesIsSeen(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "BASELINE")
	if err := os.WriteFile(base, []byte("OK\ta_spec.rb\t23511\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	md := "It passes **23,511** of ruby/spec's examples. That count is taken\n" +
		"with ruby/spec's own version guards set to resolve at\n" +
		"Ruby **4.0.9**, the version this runtime reports.\n"
	if err := os.WriteFile(filepath.Join(root, "page.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	// The fixtures say 3.4.1, the split sentence says 4.0.9: seeing the claim at
	// all is what makes this fail, which is the point. A tool blind to the break
	// would exit 1 saying no version was stated — a different message, so the
	// assertion below cannot pass by accident.
	rp, sp := versionFixtures(t, "3.4.1", "3.4.1")
	var out, errb bytes.Buffer
	cmd := RootCmd()
	cmd.SetArgs([]string{"--root", root, "--baseline", base,
		"--runtime", rp, "--shim", sp, "--timeout", "5s"})
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute()

	var ee *exitError
	if !errors.As(err, &ee) {
		t.Fatalf("a split version claim was not seen: %v\nstdout: %s", err, out.String())
	}
	if said := errb.String(); !bytes.Contains([]byte(said), []byte("says the gate measures at 4.0.9")) {
		t.Errorf("it said %q, want it to name the split claim", said)
	}
	// And it is counted once, not once per line of the join.
	if got := out.String(); !bytes.Contains([]byte(got), []byte("1 version claim(s)")) {
		t.Errorf("counted %q, want exactly one version claim", got)
	}
}

// A renamed constant must read as "I cannot tell", not as a version. A pattern
// that silently matches nothing is how a guard stops guarding.
func TestAMissingVersionConstantRefuses(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "BASELINE")
	if err := os.WriteFile(base, []byte("OK\ta_spec.rb\t23511\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fix := t.TempDir()
	rp := filepath.Join(fix, "kernel_introspect.go")
	if err := os.WriteFile(rp, []byte("const (\n\tmriVersion = \"4.0.9\"\n)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sp := filepath.Join(fix, "spec_helper.rb")
	if err := os.WriteFile(sp, []byte("CUR_VERSION = \"4.0.9\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := RootCmd()
	cmd.SetArgs([]string{"--root", "testdata", "--baseline", base,
		"--runtime", rp, "--shim", sp, "--timeout", "5s"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	var ee *exitError
	if !errors.As(err, &ee) || ee.code != 2 {
		t.Fatalf("error %v, want status 2", err)
	}
	if !bytes.Contains([]byte(ee.msg), []byte("rubyVersion")) {
		t.Errorf("the refusal %q does not name the constant it looked for", ee.msg)
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
		"  blurb = \"measured against ruby/spec: **23,481 examples pass** — the total that BASELINE sums to, " +
		"and those guards resolve at Ruby 3.4.1.\"\n"
	if err := os.WriteFile(filepath.Join(root, "hugo.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}

	rp, sp := versionFixtures(t, "3.4.1", "3.4.1")
	var out, errb bytes.Buffer
	cmd := RootCmd()
	cmd.SetArgs([]string{"--root", root, "--baseline", base,
		"--runtime", rp, "--shim", sp, "--timeout", "5s"})
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
