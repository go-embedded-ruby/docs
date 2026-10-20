// numericsurface checks that every method name this site PRINTS for the
// scientific stack is a method the interpreter actually answers.
//
// Why it exists: on 2026-10-20 the "Scientific stack" page said the Special
// module's names "are scipy's own", while the ruby repository's own README --
// which has a test holding it to the source -- says 63 of the 67 are, and names
// the four that are not. The README had been corrected after one of those four,
// `in`, turned out to be a name invented and attributed to scipy. The
// correction never crossed the repository boundary, because nothing here reads
// the ruby repository. That is the gap: tools/conformancefigure guards the one
// NUMBER this site states, tools/docexample guards the OUTPUTS of examples that
// claim one, and the long prose lists of method names -- around 150 of them --
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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
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

// exitError carries the status the command means, so the documented 1/2/3 are
// not flattened into cobra's single "an error happened" exit of 1.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

// RootCmd is the whole command, built so a test can drive it with SetArgs,
// SetOut and SetErr instead of a subprocess.
func RootCmd() *cobra.Command {
	var (
		rbgo    string
		root    string
		verbose bool
	)
	cmd := &cobra.Command{
		Use:   "numericsurface",
		Short: "Check that every method name the pages print is one the interpreter answers",
		Long: "numericsurface reads the module sections of this site, collects every token\n" +
			"written in backticks that could be a Ruby method name, and asks a real rbgo\n" +
			"whether the module answers it. It also compares the counts the pages state\n" +
			"with the counts the modules have.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return check(cmd.OutOrStdout(), cmd.ErrOrStderr(), rbgo, root, verbose)
		},
	}
	cmd.Flags().StringVar(&rbgo, "rbgo", "", "path to an rbgo binary (required)")
	cmd.Flags().StringVar(&root, "root", "content/en", "directory of Markdown files to read")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "also list the names that check out")
	return cmd
}

// check is the whole decision, with its two streams passed in.
func check(stdout, stderr io.Writer, rbgo, root string, verbose bool) error {
	if rbgo == "" {
		return &exitError{2, "--rbgo is required: without an interpreter this cannot check anything,\n" +
			"and reporting success would be a pass it did not earn."}
	}
	if _, err := os.Stat(rbgo); err != nil {
		return &exitError{2, fmt.Sprintf("--rbgo %q: %v", rbgo, err)}
	}

	have, singles, err := ask(rbgo, modules)
	if err != nil {
		return &exitError{2, fmt.Sprintf("asking the interpreter: %v", err)}
	}
	for _, m := range modules {
		if len(have[m.konst]) == 0 {
			return &exitError{2, fmt.Sprintf("the interpreter answered no names for %s; "+
				"either the module is gone or the probe no longer reaches it", m.konst)}
		}
	}

	// Read every Markdown file once; a module's section may move between pages.
	type page struct {
		path string
		body []byte
	}
	var pages []page
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
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
		return &exitError{2, fmt.Sprintf("walking %s: %v", root, err)}
	}
	if len(pages) == 0 {
		return &exitError{3, fmt.Sprintf("read no Markdown under %s -- nothing was checked", root)}
	}

	checked, bad, sections := 0, 0, 0
	for _, m := range modules {
		for _, pg := range pages {
			body := sectionFor(pg.body, m.heading)
			if body == nil {
				continue
			}
			sections++
			var unknown []string
			for _, n := range namesIn(body) {
				checked++
				if have[m.konst][n] {
					if verbose {
						fmt.Fprintf(stdout, "%s: %s.%s\n", pg.path, m.konst, n)
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
				fmt.Fprintf(stderr, "%s: the %s section prints %d name(s) the interpreter does not answer: %s\n",
					pg.path, m.konst, len(unknown), strings.Join(unknown, " "))
				bad += len(unknown)
			}
			if want, stated := countIn(body, m.counted); stated && want != singles[m.konst] {
				fmt.Fprintf(stderr, "%s: the %s section states %d, the interpreter answers %d\n",
					pg.path, m.konst, want, singles[m.konst])
				bad++
			}
		}
	}

	// "N of M, never N alone": a walk that found no section reports no
	// disagreement, which is the same output as a correct site.
	fmt.Fprintf(stdout, "%d name(s) checked over %d module section(s) against a real interpreter; %d disagree\n",
		checked, sections, bad)
	if sections == 0 || checked == 0 {
		return &exitError{3, fmt.Sprintf("found no documented module surface under %s; "+
			"either the headings changed or the pages lost their method lists", root)}
	}
	if bad > 0 {
		return &exitError{1, fmt.Sprintf("%d disagreement(s)", bad)}
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
		fmt.Fprintf(os.Stderr, "numericsurface: %s\n", ee.msg)
		osExit(ee.code)
		return
	}
	// A cobra error -- an unknown flag, an argument where none is taken. It is
	// a refusal to run, not a verdict on the pages, so it exits 2 like the
	// other refusals rather than 1.
	fmt.Fprintf(os.Stderr, "numericsurface: %v\n", err)
	osExit(2)
}
