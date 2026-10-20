// numericsurface checks that every method name this site PRINTS for the
// scientific stack is a method the interpreter actually answers.
//
// Why it exists: on 2026-10-20 the "Scientific stack" page said the Special
// module's names "are scipy's own", while the ruby repository's own README --
// which has a test holding it to the source -- says 63 of the 67 are, and names
// the four that are not. The README had been corrected after one of those four,
// `in`, turned out to be a name I had invented and attributed to scipy. The
// correction never crossed the repository boundary, because nothing here reads
// the ruby repository. That is the gap: tools/conformancefigure guards the one
// NUMBER this site states, tools/docexample guards the OUTPUTS of examples that
// claim one, and the long prose lists of method names -- around 120 of them --
// were guarded by nothing at all.
//
// WHAT IT JUDGES. For each module in the table below, it takes the page section
// whose heading names that module, collects every token written in backticks
// that could be a Ruby method name, and asks a real interpreter whether the
// module answers it. A name the page prints and the interpreter does not have
// is a promise the first reader to try it will find broken.
//
// It judges only `like_this` -- a backtick holding one bare lower-case word.
// `points:` is a keyword argument, `(value, abserr)` a return shape, `Log1pExp`
// a Go name the prose contrasts with the Ruby one, and none of them claims to
// be a method of the module. That is a deliberately narrow rule: it catches the
// defect that happened and it cannot fire on the prose around it.
//
// WHAT IT CANNOT JUDGE, and why that is said out loud: a word in backticks that
// IS a real method of some other object -- `round`, `map`, `real` -- is exempted
// by name with a reason, because the alternative is accepting every English word
// as a method and catching nothing. Each exemption below carries why it is one.
//
// Exit status: 0 every name is real; 1 one or more are not (or a stated count
// disagrees); 2 could not run anything -- a refusal to answer, not a pass;
// 3 nothing to check was found, which for a page full of names means the
// extraction broke rather than that the page is clean.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// module is one Ruby module or class the site documents, and how to find both
// its section on the page and its real method names in the interpreter.
type module struct {
	konst   string // the Ruby constant: Special, Stats, …
	heading string // what its `## ` heading contains
	// instances are Ruby expressions whose INSTANCE methods also count as
	// names of this module's surface. Stats documents `pdf`/`cdf`/`ppf` in its
	// own section although they belong to the distribution object it answers,
	// and a reader meets them there; so the check has to know about them.
	instances []string
	// counted is a phrase on the page stating how many names the module has,
	// e.g. "66 Ruby names". Empty when the page states no count. When it is
	// there, the figure must equal what the interpreter answers.
	counted string
	// prose exempts a backticked word that is not a method of this module, with
	// the reason it appears. A bare list would rot into a way of silencing the
	// tool; a reason makes each one reviewable.
	prose map[string]string
}

var modules = []module{
	{
		konst:   "Special",
		heading: "Special",
		counted: `(\d+) Ruby names`,
	},
	{
		konst:     "Stats",
		heading:   "Stats",
		instances: []string{"Stats.norm", "Stats.poisson(3.0)"},
		counted:   `(\d+) univariate distributions`,
		prose: map[string]string{
			"loc":   "scipy's ARGUMENT name, in the sentence giving the argument order",
			"scale": "scipy's ARGUMENT name, in the sentence giving the argument order",
		},
	},
	{
		konst:   "Integrate",
		heading: "Integrate",
		prose: map[string]string{
			"round": "Float#round, which the FMA paragraph says the two quadrature examples carry",
		},
	},
	{
		konst:   "Optimize",
		heading: "Optimize",
	},
	{
		konst:     "NDArray",
		heading:   "NDArray",
		instances: []string{"NDArray.arange(0, 6)"},
	},
	{
		konst:   "FFT",
		heading: "FFT",
	},
}

// candidate is a backticked token that claims to be a method name: one bare
// lower-case word, optionally ending in ? or !. Anything with a dot, slash,
// colon, space, bracket, capital or digit-leading shape is not one.
var candidate = regexp.MustCompile("`([a-z][a-z0-9_]*[?!]?)`")

// heading matches a level-2 ATX heading and captures its text.
var heading = regexp.MustCompile(`^##\s+(.*)$`)

// sectionFor returns the lines of the `## ` section whose heading contains want.
func sectionFor(md []byte, want string) []string {
	lines := strings.Split(strings.ReplaceAll(string(md), "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		m := heading.FindStringSubmatch(lines[i])
		if m == nil || !strings.Contains(m[1], want) {
			continue
		}
		var body []string
		for i++; i < len(lines); i++ {
			if heading.MatchString(lines[i]) {
				break
			}
			body = append(body, lines[i])
		}
		return body
	}
	return nil
}

// namesIn collects the candidate method names of a section, deduplicated and
// in the order a reader meets them.
func namesIn(body []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, l := range body {
		for _, m := range candidate.FindAllStringSubmatch(l, -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				out = append(out, m[1])
			}
		}
	}
	return out
}

// countIn reads the figure a section states about itself, if it states one.
func countIn(body []string, pattern string) (int, bool) {
	if pattern == "" {
		return 0, false
	}
	re := regexp.MustCompile(pattern)
	for _, l := range body {
		if m := re.FindStringSubmatch(l); m != nil {
			n, err := strconv.Atoi(m[1])
			if err == nil {
				return n, true
			}
		}
	}
	return 0, false
}

// probe is the Ruby program that answers what each module really has. It is
// built from the table rather than written out, so a module added above cannot
// be forgotten here.
func probe(mods []module) string {
	var b strings.Builder
	b.WriteString("# generated by tools/numericsurface\n")
	for _, m := range mods {
		fmt.Fprintf(&b, "%s.singleton_methods.each { |n| puts \"%s\\tsingleton\\t#{n}\" }\n", m.konst, m.konst)
		for _, inst := range m.instances {
			fmt.Fprintf(&b, "(%s).methods.each { |n| puts \"%s\\tinstance\\t#{n}\" }\n", inst, m.konst)
		}
	}
	return b.String()
}

// ask runs the probe and returns, per module, the set of names it answers and
// the number of SINGLETON names (which is what a stated count means).
func ask(rbgo string, mods []module) (have map[string]map[string]bool, singles map[string]int, err error) {
	f, err := os.CreateTemp("", "numericsurface-*.rb")
	if err != nil {
		return nil, nil, err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(probe(mods)); err != nil {
		return nil, nil, err
	}
	if err := f.Close(); err != nil {
		return nil, nil, err
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(rbgo, f.Name())
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	have = map[string]map[string]bool{}
	singles = map[string]int{}
	for _, line := range strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			continue
		}
		mod, kind, name := parts[0], parts[1], parts[2]
		if have[mod] == nil {
			have[mod] = map[string]bool{}
		}
		have[mod][name] = true
		if kind == "singleton" {
			singles[mod]++
		}
	}
	return have, singles, nil
}

func main() {
	rbgo := flag.String("rbgo", "", "path to an rbgo binary (required)")
	root := flag.String("root", "content/en", "directory of Markdown files to read")
	verbose := flag.Bool("v", false, "also list the names that check out")
	selftest := flag.Bool("selftest", false, "check the extraction against known cases and exit")
	flag.Parse()

	if *selftest {
		os.Exit(runSelfTest())
	}
	if *rbgo == "" {
		fmt.Fprintln(os.Stderr, "numericsurface: -rbgo is required: without an interpreter this cannot check anything,")
		fmt.Fprintln(os.Stderr, "and reporting success would be a pass it did not earn.")
		os.Exit(2)
	}
	if _, err := os.Stat(*rbgo); err != nil {
		fmt.Fprintf(os.Stderr, "numericsurface: -rbgo %q: %v\n", *rbgo, err)
		os.Exit(2)
	}

	have, singles, err := ask(*rbgo, modules)
	if err != nil {
		fmt.Fprintf(os.Stderr, "numericsurface: asking the interpreter: %v\n", err)
		os.Exit(2)
	}
	for _, m := range modules {
		if len(have[m.konst]) == 0 {
			fmt.Fprintf(os.Stderr, "numericsurface: the interpreter answered no names for %s; "+
				"either the module is gone or the probe no longer reaches it\n", m.konst)
			os.Exit(2)
		}
	}

	// Read every Markdown file once; a module's section may move between pages.
	type page struct {
		path string
		body []byte
	}
	var pages []page
	err = filepath.WalkDir(*root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(p) != ".md" {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		pages = append(pages, page{p, b})
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "numericsurface: walking %s: %v\n", *root, err)
		os.Exit(2)
	}
	if len(pages) == 0 {
		fmt.Fprintf(os.Stderr, "numericsurface: read no Markdown under %s -- nothing was checked\n", *root)
		os.Exit(3)
	}

	checked, bad, sections := 0, 0, 0
	for _, m := range modules {
		for _, pg := range pages {
			body := sectionFor(pg.body, m.heading)
			if body == nil {
				continue
			}
			sections++
			names := namesIn(body)
			var unknown []string
			for _, n := range names {
				checked++
				if have[m.konst][n] {
					if *verbose {
						fmt.Printf("%s: %s.%s\n", pg.path, m.konst, n)
					}
					continue
				}
				if _, ok := m.prose[n]; ok {
					continue
				}
				unknown = append(unknown, n)
			}
			if len(unknown) > 0 {
				sort.Strings(unknown)
				fmt.Fprintf(os.Stderr, "%s: the %s section prints %d name(s) the interpreter does not answer: %s\n",
					pg.path, m.konst, len(unknown), strings.Join(unknown, " "))
				bad += len(unknown)
			}
			if want, stated := countIn(body, m.counted); stated && want != singles[m.konst] {
				fmt.Fprintf(os.Stderr, "%s: the %s section states %d, the interpreter answers %d\n",
					pg.path, m.konst, want, singles[m.konst])
				bad++
			}
		}
	}

	// "N of M, never N alone": a walk that found no section reports no
	// disagreement, which is the same output as a correct site.
	fmt.Printf("%d name(s) checked over %d module section(s) against a real interpreter; %d disagree\n",
		checked, sections, bad)
	if sections == 0 || checked == 0 {
		fmt.Fprintf(os.Stderr, "numericsurface: found no documented module surface under %s; "+
			"either the headings changed or the pages lost their method lists\n", *root)
		os.Exit(3)
	}
	if bad > 0 {
		os.Exit(1)
	}
}

// runSelfTest checks the extraction on cases whose answer is known, because a
// pattern nobody tested is a guess and a guess that matches nothing looks
// exactly like a clean page. Returns the process exit status.
func runSelfTest() int {
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
	bad := 0
	for _, c := range cases {
		body := sectionFor([]byte(c.md), c.want)
		got := namesIn(body)
		if strings.Join(got, " ") != strings.Join(c.names, " ") {
			fmt.Printf("selftest %q: names %v, want %v\n", c.name, got, c.names)
			bad++
		}
		n, ok := countIn(body, c.patt)
		switch {
		case c.count < 0 && ok:
			fmt.Printf("selftest %q: read a count of %d, want none\n", c.name, n)
			bad++
		case c.count >= 0 && (!ok || n != c.count):
			fmt.Printf("selftest %q: count %d (found=%v), want %d\n", c.name, n, ok, c.count)
			bad++
		}
	}
	// The probe is generated, so a module added to the table cannot be
	// forgotten in it -- assert that it really does name every one.
	p := probe(modules)
	for _, m := range modules {
		if !strings.Contains(p, m.konst+".singleton_methods") {
			fmt.Printf("selftest: the probe does not ask about %s\n", m.konst)
			bad++
		}
		for _, inst := range m.instances {
			if !strings.Contains(p, "("+inst+").methods") {
				fmt.Printf("selftest: the probe does not ask about the instance %s\n", inst)
				bad++
			}
		}
	}
	fmt.Printf("selftest: %d case(s) and %d module(s), %d failure(s)\n", len(cases), len(modules), bad)
	if bad > 0 {
		return 1
	}
	return 0
}
