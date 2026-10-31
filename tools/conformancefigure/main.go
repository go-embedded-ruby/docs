// conformancefigure checks that every CURRENT claim about rbgo's ruby/spec
// total — and about the Ruby version that total was counted at — agrees with
// what the gate actually records.
//
// Why it exists: on 2026-10-11 this site said rbgo passed **22 488** examples on
// its front page and **23 481** "today" in two other places, while the gate's
// own file summed to 23 511 — three different figures, none of them right, in a
// repository that nothing checked. The rbgo README has a test that reads the
// baseline mechanically; the docs are a separate repository and had no
// equivalent, so the number rotted every release.
//
// WHAT IT JUDGES. Not every number that looks like a total. A **dated** sweep
// going stale is not an error — it is what it says it is, and this site keeps
// one on purpose ("measured 2026-09-26", with its own failing/erroring/skipped
// breakdown). What must track the gate is a claim written in the present tense,
// and those are recognisable by the words around the figure: "it passes N",
// "the sum is N", "whose sum is N". So the match is on a VOCABULARY, and the
// vocabulary has its own self-test below — a phrasing nobody tested is a guess,
// and a guess that matches nothing looks exactly like a clean tree.
//
// Exit status: 0 agree, 1 disagree (or nothing found to check), 2 could not get
// the authoritative number — which is a refusal to answer, not a pass.
package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// baselineURL is the gate's own file in the ruby repository. The figure is read
// from the file CI enforces, not from a release note or a badge, because those
// are themselves downstream of it.
const baselineURL = "https://raw.githubusercontent.com/go-embedded-ruby/ruby/main/scripts/conformance/rubyspec/BASELINE"

// The total is only half of what a reader needs. `23,511 examples pass` is a
// count over a POPULATION, and the population is selected by the Ruby version
// the shim resolves `ruby_version_is` guards at: at 3.4.1 every spec behind
// `ruby_version_is "4.0"` is removed before anything is counted, so the figure
// is a 3.4-shaped result whichever MRI the project says it targets. Measured
// 2026-10-27 (ruby#799, ruby#836): 61 block guards and 5 inline sites sit at
// the "4.0" boundary, and 41 guards assert what 4.0 changed away from.
//
// So the version is a claim of the same kind as the number, and it is read from
// BOTH places that spell it:
//
//	internal/vm/kernel_introspect.go  rubyVersion  -- what the runtime REPORTS
//	scripts/.../spec_helper.rb        CUR_VERSION  -- what the gate MEASURES AT
//
// ⛔ These are two hardcoded copies of one fact. ruby#799 asks for the second
// to be derived from the first precisely because "today they happen to agree"
// is the defect waiting to happen: a parity change that moves the runtime and
// forgets the shim would leave the gate measuring the old population while
// every page claimed the new version, and nothing would say so. Reading both
// and refusing when they differ is why this tool exists at all -- two readers
// of one fact cannot drift, where two transcriptions already had.
const (
	runtimeURL = "https://raw.githubusercontent.com/go-embedded-ruby/ruby/main/internal/vm/kernel_introspect.go"
	shimURL    = "https://raw.githubusercontent.com/go-embedded-ruby/ruby/main/scripts/conformance/rubyspec/spec_helper.rb"
)

// Anchored at the start of a line so a mention inside a comment or a string --
// both files discuss these constants at length -- cannot be read as the
// definition. The runtime's is indented inside a const block; the shim's is at
// column zero.
var (
	runtimeVersionRe = regexp.MustCompile(`(?m)^\trubyVersion\s*=\s*"(\d+\.\d+\.\d+)"`)
	shimVersionRe    = regexp.MustCompile(`(?m)^CUR_VERSION\s*=\s*"(\d+\.\d+\.\d+)"`)
)

// Two shapes of CURRENT claim, because two sites phrase it two ways round.
//
// claimBefore: the words come first -- "It passes 23,511", "whose sum is
// 23 511". This is the docs' prose style.
//
// claimAfter: the figure comes first -- "<b>23,511</b> ruby/spec … examples
// passing", "a per-file baseline summing 23,511". This is the landing page's
// style, and matching only the first shape meant the tool reported
// "0 current claim(s) checked" there -- honest, since it refuses to call an
// unmatched vocabulary clean, but useless.
//
// Thousands may be separated by a space (the docs' style) or a comma (the
// landing page's and the README's), and the figure may be wrapped in Markdown
// bold or HTML tags.
var (
	claimBefore = regexp.MustCompile(`(?i)\b(it passes|the sum is|its sum is|whose sum is|summing|passes)\b[^0-9]{0,24}\*{0,2}(\d{1,3}[ ,]?\d{3})\*{0,2}`)
	claimAfter  = regexp.MustCompile(`(?i)\**(\d{1,3}[ ,]?\d{3})\**(?:</b>|</div>)?[^.!?0-9]{0,90}?\bexamples?\s+(?:pass|passing)\b`)
)

// datedLine exempts a line that says when it was measured. On the landing page
// the dated block and the present-tense claims have the SAME shape -- both are
// "<figure> examples pass" -- and only "measured <date>" tells them apart:
//
//	<b>22,488 examples pass</b> in … measured 2026-09-26 on main (3e8e3cc)   <- dated, exempt
//	<b>23,511</b> ruby/spec … examples passing                               <- current, checked
//
// It is a convention rather than a proof, and it is the convention both sites
// already follow. A line that happened to contain "measured <date>" for another
// reason would be exempted wrongly; that is the failure mode, stated here so it
// is a known limit and not a surprise.
var datedLine = regexp.MustCompile(`(?i)measured\s+\d{4}-\d{2}-\d{2}`)

// The version claim, in the three shapes these sites use for it.
//
// ⛔ THE ORACLE IS A DIFFERENT CLAIM AND MUST NOT MATCH. These pages say
// "differential-tested against MRI Ruby 4.0.5" and "against MRI 4.0.7 on the
// same host": that is the interpreter rbgo's behaviour was COMPARED WITH, which
// is free to be newer than the version the shim resolves guards at -- and is,
// deliberately. A pattern that read "against MRI 4.0.5" as the conformance
// version would compare the oracle against CUR_VERSION and report a
// disagreement that is not one; worse, once both were wrong in the same
// direction it would report agreement. So "against" is deliberately absent from
// every alternative below, and three oracle sentences are negative cases.
var versionClaim = []*regexp.Regexp{
	// "those guards resolve at Ruby 3.4.1"
	regexp.MustCompile(`(?i)\bresolve[sd]?\s+at\b[^0-9]{0,24}?\**v?(\d+\.\d+\.\d+)\**`),
	// "the suite runs as Ruby 3.4.1"
	regexp.MustCompile(`(?i)\bruns?\s+as\b[^0-9]{0,24}?\**v?(\d+\.\d+\.\d+)\**`),
	// "`RUBY_VERSION` reports `\"3.4.1\"`" and "`CUR_VERSION` is 3.4.1" -- the
	// constants named outright, which is how the known-defect tables phrase it.
	regexp.MustCompile("(?i)`?(?:RUBY_VERSION|CUR_VERSION)`?\\s*(?:reports|is|=)\\s*\\**`?\"?(\\d+\\.\\d+\\.\\d+)"),
}

// versionCases is the version vocabulary's own control, in the same spirit as
// vocabularyCases below and run on every invocation for the same reason.
var versionCases = []struct {
	line string
	want bool
	v    string
}{
	// The three shapes, as the pages will carry them.
	{"and those guards **resolve at Ruby 3.4.1**, not at the version the oracle runs.", true, "3.4.1"},
	{"the suite runs as Ruby **3.4.1** while the oracle is newer", true, "3.4.1"},
	{"| `RUBY_VERSION` reports `\"3.4.1\"`, though the differential oracle is MRI 4.0.7 |", true, "3.4.1"},
	{"`CUR_VERSION` is 3.4.1, which selects the population before anything is counted", true, "3.4.1"},
	// ⛔ The oracle. Three real sentences from these pages, all of which name a
	// DIFFERENT and newer MRI on purpose.
	{"All behaviour is differential-tested against MRI Ruby 4.0.5 (where MRI has an", false, ""},
	{"Each entry below was re-run on `4872e01` against **MRI 4.0.7 on the same host**", false, ""},
	{"- with **every feature differential-tested against MRI 4.0.5**.", false, ""},
	// A target stated without a patch level is a statement of intent about
	// semantics, not of the version a guard resolves at.
	{"go-embedded-ruby targets **Ruby 4.0** semantics. Correctness is not asserted by", false, ""},
	// The total, and a benchmark table: numbers that are not versions.
	{`<p><b>23,511</b> ruby/spec <code>language/</code> + <code>core/</code> examples pass`, false, ""},
	{"| module.op | rbgo | MRI 4.0.5 | MRI+YJIT | JRuby 10.1 | TruffleRuby 34.0.1 | rbgo/MRI |", false, ""},
}

// vocabularyCases is the guard's own control, run before it judges anything.
// Each is a line this tool will meet; `want` is whether a current claim should
// be recognised in it.
var vocabularyCases = []struct {
	line string
	want bool
	n    string
}{
	// Words first: the docs' prose style.
	{"**How complete is it?** It passes **23,511** of ruby/spec's `language/`", true, "23,511"},
	{"baseline, whose sum is **23 511** at v0.3.0 (2026-10-11).", true, "23 511"},
	{"as a guarantee overstates it by the same amount. The sum is **23 511** as of", true, "23 511"},
	// Figure first: the landing page's style, in all three shapes it uses.
	{`<div class="trust"><span><b>23,511</b> ruby/spec <code>language/</code>+<code>core/</code> examples passing</span>`, true, "23,511"},
	{`<p><b>23,511</b> ruby/spec <code>language/</code> + <code>core/</code> examples pass, gated in CI <b>file by file</b>`, true, "23,511"},
	{"a per-file baseline summing <b>23,511</b>.", true, "23,511"},
	// The dated sweep's own row and its prose: no claim about now, so no match.
	{"| **passing examples** | **22 488** |", false, ""},
	{"sweep that tallies all four counters. All three reported **22 488** passing", false, ""},
	// The landing page's DATED block has the same shape as a current claim and
	// is exempted by its date, not by its wording. Without this case the
	// extension above would have started failing the landing page on a figure
	// that is correct for the run it names.
	{`<p><b>22,488 examples pass</b> in ruby/spec's suites, measured 2026-09-26 on <code>main</code>`, false, ""},
	// A number that is about something else entirely.
	{"the floor went from **6 000** when the ratchet landed on 2026-08-03", false, ""},
	{"climbed to **22 475** by the time it was replaced", false, ""},
	// The landing page's ratio line, VERBATIM -- it carries no date, so it is not
	// exempt, and it must miss on its own. It does: the figure is followed by
	// "of 24,116" rather than by "examples pass", and the words before it are
	// "93.3% pass" rather than "passes". That is a narrow escape and it is
	// recorded as a case so a later widening of either pattern cannot start
	// reading a ratio as a total without this failing first.
	{`<li>Of the examples that ran, <b>93.3%</b> pass (22,488 of 24,116); 477 more are skipped because our shim stubs the matcher they need, and 10 of 2,206 spec files produce no result.</li>`, false, ""},
}

// match returns the figure a line claims as the CURRENT total, if any.
func match(line string) (string, bool) {
	if datedLine.MatchString(line) {
		return "", false
	}
	if m := claimBefore.FindStringSubmatch(line); m != nil {
		return m[2], true
	}
	if m := claimAfter.FindStringSubmatch(line); m != nil {
		return m[1], true
	}
	return "", false
}

// matchVersion returns the version a line claims the gate resolves guards at.
// A dated line is exempt for the same reason a dated total is.
func matchVersion(line string) (string, bool) {
	if datedLine.MatchString(line) {
		return "", false
	}
	for _, re := range versionClaim {
		if m := re.FindStringSubmatch(line); m != nil {
			return m[1], true
		}
	}
	return "", false
}

// allVersions returns every version claim on a line.
func allVersions(line string) []string {
	if datedLine.MatchString(line) {
		return nil
	}
	var out []string
	for _, re := range versionClaim {
		for _, m := range re.FindAllStringSubmatch(line, -1) {
			out = append(out, m[1])
		}
	}
	return out
}

// versionsAcross reads a claim that a line break cut in half.
//
// ⛔ Measured 2026-10-31, on this tool's own first run over the real pages: a
// sentence was written as
//
//	… version guards set to resolve at
//	Ruby **3.4.1**, the version this runtime reports …
//
// and the scan, which reads one line at a time, saw neither half. The tool
// reported "2 version claim(s)" where three had been written, and 2 is a
// perfectly clean-looking number — the miss was found only by insisting on
// explaining why it was not 3. Markdown prose is re-wrapped by every editor
// that touches it, so this is not a one-off typo; it is the normal state of a
// paragraph.
//
// It is attempted only when NEITHER line claims anything on its own, so a claim
// is never counted twice, and the join is attributed to the second line because
// that is where the version itself is written. The caller passes what each line
// already yielded rather than having this recompute it: three patterns over
// every line of every page, twice, for a result already in hand.
func versionsAcross(prev string, prevHad bool, line string, lineHad bool) []string {
	if prev == "" || prevHad || lineHad {
		return nil
	}
	return allVersions(prev + " " + line)
}

// allMatches returns every current-total figure on a line, so a line carrying
// two claims is not judged on the first alone.
func allMatches(line string) []string {
	if datedLine.MatchString(line) {
		return nil
	}
	var out []string
	for _, m := range claimBefore.FindAllStringSubmatch(line, -1) {
		out = append(out, m[2])
	}
	for _, m := range claimAfter.FindAllStringSubmatch(line, -1) {
		out = append(out, m[1])
	}
	return out
}

// exitError carries the status the command means, so the documented 0/1/2 are
// not flattened into cobra's single "an error happened" exit of 1.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

// checkVocabulary runs the guard's own control.
//
// ⛔ It runs on EVERY invocation, not only under `go test`. This tool is used
// from another repository as `go run github.com/go-embedded-ruby/docs/tools/
// conformancefigure@vX`, and that consumer never runs this module's tests — so
// a control that lived only in a _test.go would silently stop protecting the
// landing page, which is the site whose HTML phrasing the vocabulary was
// extended for. A wrong vocabulary that matches nothing is indistinguishable
// from a site with no stale figures.
func checkVocabulary() error {
	for _, c := range vocabularyCases {
		n, got := match(c.line)
		if got != c.want {
			return &exitError{2, fmt.Sprintf("the vocabulary itself is wrong: %q matched=%v, want %v", c.line, got, c.want)}
		}
		if c.want && n != c.n {
			return &exitError{2, fmt.Sprintf("the vocabulary read %q as %q, want %q", c.line, n, c.n)}
		}
	}
	for _, c := range versionCases {
		v, got := matchVersion(c.line)
		if got != c.want {
			return &exitError{2, fmt.Sprintf("the version vocabulary is wrong: %q matched=%v, want %v", c.line, got, c.want)}
		}
		if c.want && v != c.v {
			return &exitError{2, fmt.Sprintf("the version vocabulary read %q as %q, want %q", c.line, v, c.v)}
		}
	}
	return nil
}

// RootCmd is the whole command, built so a test can drive it with SetArgs,
// SetOut and SetErr instead of a subprocess.
func RootCmd() *cobra.Command {
	var (
		root        string
		baseline    string
		runtimePath string
		shimPath    string
		timeout     time.Duration
	)
	cmd := &cobra.Command{
		Use:   "conformancefigure",
		Short: "Check the stated ruby/spec total, and the version it was counted at, against the gate",
		Long: "conformancefigure reads the present-tense claims this site makes about\n" +
			"rbgo's ruby/spec total and compares each with the sum of the gate's own\n" +
			"per-file baseline. A dated sweep going stale is not an error and is\n" +
			"exempt by its date.\n\n" +
			"It also checks the Ruby version the gate resolves `ruby_version_is`\n" +
			"guards at, because that version selects the population the total counts\n" +
			"over. The version is read from both files that spell it — the runtime's\n" +
			"`rubyVersion` and the shim's `CUR_VERSION` — and a disagreement between\n" +
			"them is a refusal, not a verdict on these pages.\n\n" +
			"The differential oracle named on these pages is a DIFFERENT claim, free\n" +
			"to be newer, and is deliberately not matched.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return check(cmd.OutOrStdout(), cmd.ErrOrStderr(), root, baseline, runtimePath, shimPath, timeout)
		},
	}
	// content/en since the move to Hugo (#28); it was docs/ under MkDocs, and a
	// default naming a directory that no longer exists meant every hand-run of
	// this tool refused with "lstat docs: no such file".
	cmd.Flags().StringVar(&root, "root", "content/en", "the directory of Markdown and HTML to check")
	cmd.Flags().StringVar(&baseline, "baseline", "", "read the authoritative BASELINE from this path instead of fetching it")
	cmd.Flags().StringVar(&runtimePath, "runtime", "", "read kernel_introspect.go from this path instead of fetching it")
	cmd.Flags().StringVar(&shimPath, "shim", "", "read spec_helper.rb from this path instead of fetching it")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "how long to wait for the fetch")
	return cmd
}

func check(stdout, stderr io.Writer, root, baseline, runtimePath, shimPath string, timeout time.Duration) error {
	// The control first: if the vocabulary is wrong, every verdict after it is
	// worthless.
	if err := checkVocabulary(); err != nil {
		return err
	}

	want, err := authoritative(baseline, timeout)
	if err != nil {
		// A refusal, not a pass: an unreadable baseline finds no disagreement
		// either, and that silence reads exactly like agreement.
		return &exitError{2, fmt.Sprintf("cannot read the gate's total: %v", err)}
	}

	wantVersion, err := authoritativeVersion(runtimePath, shimPath, timeout)
	if err != nil {
		return &exitError{2, fmt.Sprintf("cannot read the version the gate measures at: %v", err)}
	}

	type finding struct {
		where string
		got   int
		text  string
	}
	type versionFinding struct {
		where string
		got   string
	}
	var wrong []finding
	var wrongVersion []versionFinding
	checked, versionsChecked := 0, 0

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// Markdown for the docs site, HTML for the landing page's Hugo layouts
		// — and TOML, because a Hugo site keeps prose in its configuration too.
		//
		// ⛔ Measured 2026-10-30: go-embedded-ruby.github.io stated the total in
		// THREE places. The two under layouts/ read 23,511 and were correct,
		// because this tool reads them. The third, in hugo.toml's
		// `params.phases[].blurb`, read 23,481 — thirty behind, and drifting
		// freely since 2026-10-04, because neither this filter nor the lane's
		// `--root layouts` could reach it. A guard is only as good as the
		// population it walks, and prose in a config file is still prose on
		// the page.
		if !hasAnySuffix(path, ".md", ".html", ".toml") {
			return nil
		}
		f, oerr := os.Open(path)
		if oerr != nil {
			return oerr
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20) // roadmap.md has very long lines
		// Carried across lines so a claim a line break cut in half is still read;
		// prevHadVersion keeps that join from counting a claim twice.
		prev, prevHadVersion := "", false
		for line := 1; sc.Scan(); line++ {
			text := sc.Text()
			for _, fig := range allMatches(text) {
				n, cerr := strconv.Atoi(strings.NewReplacer(" ", "", ",", "").Replace(fig))
				if cerr != nil {
					continue
				}
				checked++
				if n != want {
					wrong = append(wrong, finding{
						where: fmt.Sprintf("%s:%d", path, line),
						got:   n,
						text:  fig,
					})
				}
			}
			own := allVersions(text)
			ownHad := len(own) > 0
			versions := append(own, versionsAcross(prev, prevHadVersion, text, ownHad)...)
			prev, prevHadVersion = text, ownHad
			for _, v := range versions {
				versionsChecked++
				if v != wantVersion {
					wrongVersion = append(wrongVersion, versionFinding{
						where: fmt.Sprintf("%s:%d", path, line),
						got:   v,
					})
				}
			}
		}
		return sc.Err()
	})
	if err != nil {
		return &exitError{2, fmt.Sprintf("walking %s: %v", root, err)}
	}

	// "N of M, never N alone": a walk that matched nothing reports no
	// disagreement, which is the same output as a correct site.
	fmt.Fprintf(stdout, "%d total claim(s) and %d version claim(s) checked against the gate's %d at Ruby %s\n",
		checked, versionsChecked, want, wantVersion)
	// ⛔ THE DISAGREEMENTS ARE REPORTED BEFORE THE ABSENCES, and that order is
	// the whole of it. Written the other way round -- the version-absence
	// refusal returning first -- this tool answered "found no statement of the
	// version" on a page whose TOTAL was wrong by 1,023, and the stale figure it
	// exists to catch never reached the output. A new check must not be able to
	// swallow the finding of the old one: a concrete disagreement outranks a
	// structural complaint about what the page does not say.
	for _, w := range wrong {
		fmt.Fprintf(stderr, "%s: claims %d, the gate records %d\n   %s\n", w.where, w.got, want, w.text)
	}
	for _, w := range wrongVersion {
		fmt.Fprintf(stderr, "%s: says the gate measures at %s, it measures at %s\n", w.where, w.got, wantVersion)
	}
	if n := len(wrong) + len(wrongVersion); n > 0 {
		return &exitError{1, fmt.Sprintf("%d claim(s) disagree with the ruby repository", n)}
	}
	if checked == 0 {
		return &exitError{1, fmt.Sprintf("found no statement of the total in %s; "+
			"either the vocabulary no longer matches how the docs phrase it, or the figure was dropped", root)}
	}
	// The same refusal, for the same reason: a site that states a total and
	// never says which version selected the population is stating half a
	// measurement, and a version vocabulary that matches nothing is
	// indistinguishable from a site that states it correctly.
	if versionsChecked == 0 {
		return &exitError{1, fmt.Sprintf("found no statement of the version the gate measures at in %s; "+
			"a total without it is a count over an unnamed population (Ruby %s today)", root, wantVersion)}
	}
	return nil
}

// osExit is a variable so a test can observe the status without ending the
// test binary.
var osExit = os.Exit

func main() {
	err := RootCmd().Execute()
	if err == nil {
		return
	}
	var ee *exitError
	if errors.As(err, &ee) {
		fmt.Fprintf(os.Stderr, "conformancefigure: %s\n", ee.msg)
		osExit(ee.code)
		return
	}
	// A cobra error -- an unknown flag, an argument where none is taken. It is
	// a refusal to run, not a verdict on the pages, so it exits 2.
	fmt.Fprintf(os.Stderr, "conformancefigure: %v\n", err)
	osExit(2)
}

// authoritative sums the gate's per-file baseline: one `OK<TAB>path<TAB>passing`
// record per spec file. It is the same arithmetic the ruby repository's own test
// does, deliberately — two readers of one file cannot drift, where two
// transcriptions of one number already have.
func authoritative(path string, timeout time.Duration) (int, error) {
	// Through the same bounded fetch as the version files: this path used to
	// stream the response straight into a Scanner, which is an unbounded read of
	// a remote answer. The baseline is ~2,200 short rows.
	b, err := fetch(path, baselineURL, timeout)
	if err != nil {
		return 0, err
	}

	total, rows := 0, 0
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		parts := strings.Split(sc.Text(), "\t")
		if len(parts) != 3 || parts[0] != "OK" {
			continue
		}
		n, err := strconv.Atoi(parts[2])
		if err != nil {
			return 0, fmt.Errorf("%s: unreadable count %q", parts[1], parts[2])
		}
		total += n
		rows++
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	// A baseline with no rows sums to 0, and 0 is a number: it would be compared
	// against the docs and reported as a disagreement with a confident figure.
	if rows == 0 {
		return 0, fmt.Errorf("no OK rows found; this is not a per-file baseline")
	}
	return total, nil
}

// fetch returns the bytes of a local path when one is given, and otherwise of
// the URL. The 404 guard is the same one authoritative() needed and for the
// same reason: GitHub answers a missing file with a page, and a page a pattern
// does not match reads as "the constant is not there" rather than as "the
// request was wrong".
// maxFetch bounds what a remote answer may cost us. The three files read here
// are tens of kilobytes; 8 MiB is far above any of them and far below what an
// unbounded io.ReadAll would let a redirected or replaced URL spend. Reading
// one byte past the limit is an error rather than a truncation, because a
// truncated file is a file whose constants may simply be missing — which this
// tool would then report as "no version found", a refusal that points at the
// wrong thing.
const maxFetch = 8 << 20

func fetch(path, url string, timeout time.Duration) ([]byte, error) {
	if path != "" {
		return os.ReadFile(path)
	}
	cl := &http.Client{Timeout: timeout}
	resp, err := cl.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// A 404 body is still a body: without this the callers would read
		// GitHub's error page and find no rows and no constants in it, which
		// reads as an answer rather than as a failed request.
		return nil, fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxFetch+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxFetch {
		return nil, fmt.Errorf("%s: larger than %d bytes", url, maxFetch)
	}
	return b, nil
}

// authoritativeVersion reports the Ruby version the conformance population is
// selected by, read from both files that spell it and refusing when they
// disagree.
//
// It returns an error rather than a best guess in three cases, and each is a
// refusal to answer rather than a verdict: the file could not be read, the
// constant was not found in it (a rename must not read as "no version"), or the
// two copies differ. The third is the one worth having: it is the exact state
// ruby#799 describes as waiting to happen, and on the day half a parity change
// lands this is what says so.
func authoritativeVersion(runtimePath, shimPath string, timeout time.Duration) (string, error) {
	rt, err := fetch(runtimePath, runtimeURL, timeout)
	if err != nil {
		return "", fmt.Errorf("the runtime's version: %w", err)
	}
	sh, err := fetch(shimPath, shimURL, timeout)
	if err != nil {
		return "", fmt.Errorf("the shim's version: %w", err)
	}
	rm := runtimeVersionRe.FindSubmatch(rt)
	if rm == nil {
		return "", fmt.Errorf("no `rubyVersion = \"x.y.z\"` in %s", runtimeURL)
	}
	sm := shimVersionRe.FindSubmatch(sh)
	if sm == nil {
		return "", fmt.Errorf("no `CUR_VERSION = \"x.y.z\"` in %s", shimURL)
	}
	runtime, shim := string(rm[1]), string(sm[1])
	if runtime != shim {
		return "", fmt.Errorf("the two copies of the version disagree: the runtime reports %s "+
			"(kernel_introspect.go) while the gate measures at %s (spec_helper.rb) — "+
			"half a version change has landed; see ruby#799", runtime, shim)
	}
	return runtime, nil
}

// hasAnySuffix is spelled out so the file types this walks are a list in one
// place rather than a chain of && that grows a bug each time it is extended.
func hasAnySuffix(path string, suffixes ...string) bool {
	for _, s := range suffixes {
		if strings.HasSuffix(path, s) {
			return true
		}
	}
	return false
}
