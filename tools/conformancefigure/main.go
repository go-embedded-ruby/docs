// conformancefigure checks that every CURRENT claim about rbgo's ruby/spec
// total agrees with the number the gate actually records.
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
	"flag"
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
)

// baselineURL is the gate's own file in the ruby repository. The figure is read
// from the file CI enforces, not from a release note or a badge, because those
// are themselves downstream of it.
const baselineURL = "https://raw.githubusercontent.com/go-embedded-ruby/ruby/main/scripts/conformance/rubyspec/BASELINE"

// claim matches a figure stated as a CURRENT total. The leading phrase is what
// separates it from the dated table, whose row is `| **passing examples** |
// **22 488** |` and carries none of these words.
//
// Thousands may be separated by a space (the docs' style) or a comma (the
// README's), and the figure may be wrapped in Markdown bold.
var claim = regexp.MustCompile(`(?i)\b(it passes|the sum is|its sum is|whose sum is|passes)\b[^0-9]{0,24}\*{0,2}(\d{1,3}[ ,]?\d{3})\*{0,2}`)

// vocabularyCases is the guard's own control, run before it judges anything.
// Each is a line this tool will meet; `want` is whether a current claim should
// be recognised in it.
var vocabularyCases = []struct {
	line string
	want bool
	n    string
}{
	{"**How complete is it?** It passes **23,511** of ruby/spec's `language/`", true, "23,511"},
	{"baseline, whose sum is **23 511** at v0.3.0 (2026-10-11).", true, "23 511"},
	{"as a guarantee overstates it by the same amount. The sum is **23 511** as of", true, "23 511"},
	// The dated sweep's own row and its prose: no claim about now, so no match.
	{"| **passing examples** | **22 488** |", false, ""},
	{"sweep that tallies all four counters. All three reported **22 488** passing", false, ""},
	// A number that is about something else entirely.
	{"the floor went from **6 000** when the ratchet landed on 2026-08-03", false, ""},
	{"climbed to **22 475** by the time it was replaced", false, ""},
}

func main() {
	var (
		root     = flag.String("root", "docs", "the directory of Markdown to check")
		baseline = flag.String("baseline", "", "read the authoritative BASELINE from this path instead of fetching it")
		timeout  = flag.Duration("timeout", 30*time.Second, "how long to wait for the fetch")
	)
	flag.Parse()

	// The control first: if the vocabulary is wrong, every verdict after it is
	// worthless, and a wrong vocabulary that matches nothing is indistinguishable
	// from a site with no stale figures.
	for _, c := range vocabularyCases {
		m := claim.FindStringSubmatch(c.line)
		got := m != nil
		if got != c.want {
			fmt.Fprintf(os.Stderr, "conformancefigure: the vocabulary itself is wrong: %q matched=%v, want %v\n", c.line, got, c.want)
			os.Exit(2)
		}
		if c.want && m[2] != c.n {
			fmt.Fprintf(os.Stderr, "conformancefigure: the vocabulary read %q as %q, want %q\n", c.line, m[2], c.n)
			os.Exit(2)
		}
	}

	want, err := authoritative(*baseline, *timeout)
	if err != nil {
		// A refusal, not a pass: an unreadable baseline finds no disagreement
		// either, and that silence reads exactly like agreement.
		fmt.Fprintf(os.Stderr, "conformancefigure: cannot read the gate's total: %v\n", err)
		os.Exit(2)
	}

	type finding struct {
		where string
		got   int
		text  string
	}
	var wrong []finding
	checked := 0

	err = filepath.WalkDir(*root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		f, oerr := os.Open(path)
		if oerr != nil {
			return oerr
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20) // roadmap.md has very long lines
		for line := 1; sc.Scan(); line++ {
			for _, m := range claim.FindAllStringSubmatch(sc.Text(), -1) {
				n, cerr := strconv.Atoi(strings.NewReplacer(" ", "", ",", "").Replace(m[2]))
				if cerr != nil {
					continue
				}
				checked++
				if n != want {
					wrong = append(wrong, finding{
						where: fmt.Sprintf("%s:%d", path, line),
						got:   n,
						text:  strings.TrimSpace(m[0]),
					})
				}
			}
		}
		return sc.Err()
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "conformancefigure: walking %s: %v\n", *root, err)
		os.Exit(2)
	}

	// "N of M, never N alone": a walk that matched nothing reports no
	// disagreement, which is the same output as a correct site.
	fmt.Printf("%d current claim(s) checked against the gate's %d\n", checked, want)
	if checked == 0 {
		fmt.Fprintf(os.Stderr, "conformancefigure: found no statement of the total in %s; "+
			"either the vocabulary no longer matches how the docs phrase it, or the figure was dropped\n", *root)
		os.Exit(1)
	}
	for _, w := range wrong {
		fmt.Fprintf(os.Stderr, "%s: claims %d, the gate records %d\n   %s\n", w.where, w.got, want, w.text)
	}
	if len(wrong) > 0 {
		fmt.Fprintf(os.Stderr, "\n%d claim(s) disagree with %s\n", len(wrong), baselineURL)
		os.Exit(1)
	}
}

// authoritative sums the gate's per-file baseline: one `OK<TAB>path<TAB>passing`
// record per spec file. It is the same arithmetic the ruby repository's own test
// does, deliberately — two readers of one file cannot drift, where two
// transcriptions of one number already have.
func authoritative(path string, timeout time.Duration) (int, error) {
	var r io.Reader
	if path != "" {
		f, err := os.Open(path)
		if err != nil {
			return 0, err
		}
		defer f.Close()
		r = f
	} else {
		cl := &http.Client{Timeout: timeout}
		resp, err := cl.Get(baselineURL)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			// A 404 body is still a body: without this the scan below would read
			// GitHub's error page, find no OK rows, and report a total of 0.
			return 0, fmt.Errorf("%s: HTTP %d", baselineURL, resp.StatusCode)
		}
		r = resp.Body
	}

	total, rows := 0, 0
	sc := bufio.NewScanner(r)
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
