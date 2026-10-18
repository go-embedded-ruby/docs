// docexample runs the Ruby examples on this site and checks each one against
// the answer written beside it.
//
// Why it exists: the pages teach by example, and an example carries its answer
// in a comment -- `p Special.gamma(5.0)   # => 24.0`. Nothing ran them. A
// binding can be renamed, a default changed or a return shape altered in the
// ruby repository and every example here keeps its old answer, which reads as
// a promise and is the first thing a reader tries. tools/conformancefigure
// already guards the one NUMBER this site states; this guards the dozens of
// OUTPUTS it states.
//
// WHAT IT JUDGES. A fenced ```ruby block is checked exactly when it claims an
// output -- that is, when some line in it ends in `# => something`. A block
// with no such comment claims nothing and is skipped, which is what makes the
// selection self-evident rather than a list to maintain: the examples that read
// a photo off disk or start a server carry no `# =>` and so are left alone, and
// one that gains an answer is picked up without being registered anywhere.
//
// HOW IT COMPARES. The block runs as one program, so its earlier lines can set
// up the later ones, and the lines it PRINTS are compared in order against the
// `# =>` values it claims. A differing count is itself a failure: an example
// that prints two lines while claiming one is as wrong as a wrong number, and
// the common cause -- `p a, b` printing twice -- is exactly the kind of thing
// nobody notices by reading.
//
// Trailing whitespace around a claimed value is ignored, because a doc author
// aligns the comments. Nothing else is normalised: if the answer differs by a
// digit, that is the point.
//
// Exit status: 0 every example agrees; 1 one or more disagree; 2 could not run
// anything (no interpreter given, or it would not build) -- a refusal to
// answer, not a pass; 3 nothing to check was found, which for a site with
// examples means the extraction broke rather than that the site is clean.
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
	"strings"
)

// fence opens and closes a Markdown code fence, capturing its language.
var fence = regexp.MustCompile("^\\s*```(\\w*)\\s*$")

// claim matches a line that states its own answer: anything, then a comment of
// the form `# => value`, where the value must start with a non-space -- a bare
// `# =>` claims nothing, and treating it as a claim of "" would compare the
// printed line against an empty string and report a mismatch nobody wrote.
// The value runs to the end of the line.
var claim = regexp.MustCompile(`#\s*=>\s*(\S.*)$`)

// example is one fenced block that claims at least one answer.
type example struct {
	file string
	line int      // 1-based line of the opening fence
	src  string   // the block's Ruby, verbatim
	want []string // the claimed answers, in order
}

// extract reads the examples out of one Markdown file.
func extract(path string, md []byte) []example {
	var out []example
	lines := strings.Split(string(md), "\n")
	for i := 0; i < len(lines); i++ {
		m := fence.FindStringSubmatch(lines[i])
		if m == nil || m[1] != "ruby" {
			continue
		}
		start := i
		var body []string
		for i++; i < len(lines); i++ {
			if fence.MatchString(lines[i]) {
				break
			}
			body = append(body, lines[i])
		}
		var want []string
		for _, l := range body {
			if c := claim.FindStringSubmatch(l); c != nil {
				want = append(want, strings.TrimSpace(c[1]))
			}
		}
		if len(want) == 0 {
			continue // claims nothing
		}
		out = append(out, example{
			file: path,
			line: start + 1,
			src:  strings.Join(body, "\n") + "\n",
			want: want,
		})
	}
	return out
}

// run evaluates one example and returns the lines it printed.
func run(rbgo string, ex example) ([]string, error) {
	f, err := os.CreateTemp("", "docexample-*.rb")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(ex.src); err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(rbgo, f.Name())
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	got := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(got) == 1 && got[0] == "" {
		got = nil
	}
	return got, nil
}

func main() {
	rbgo := flag.String("rbgo", "", "path to an rbgo binary (required)")
	root := flag.String("root", "docs", "directory of Markdown files to read")
	verbose := flag.Bool("v", false, "also list the examples that agree")
	selftest := flag.Bool("selftest", false, "check the extraction against known cases and exit")
	flag.Parse()

	if *selftest {
		os.Exit(runSelfTest())
	}
	if *rbgo == "" {
		fmt.Fprintln(os.Stderr, "docexample: -rbgo is required: without an interpreter this cannot check anything,")
		fmt.Fprintln(os.Stderr, "and reporting success would be a pass it did not earn.")
		os.Exit(2)
	}
	if _, err := os.Stat(*rbgo); err != nil {
		fmt.Fprintf(os.Stderr, "docexample: -rbgo %q: %v\n", *rbgo, err)
		os.Exit(2)
	}

	var examples []example
	walked := 0
	err := filepath.WalkDir(*root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(p) != ".md" {
			return nil
		}
		walked++
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		examples = append(examples, extract(p, b)...)
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "docexample: walking %s: %v\n", *root, err)
		os.Exit(2)
	}
	if walked == 0 {
		fmt.Fprintf(os.Stderr, "docexample: read no Markdown under %s -- nothing was checked\n", *root)
		os.Exit(3)
	}
	if len(examples) == 0 {
		fmt.Fprintf(os.Stderr, "docexample: %d Markdown file(s) and no example claiming an answer.\n", walked)
		fmt.Fprintln(os.Stderr, "For a site that teaches by example that means the extraction broke, not that it is clean.")
		os.Exit(3)
	}

	bad := 0
	claims := 0
	for _, ex := range examples {
		claims += len(ex.want)
		got, err := run(*rbgo, ex)
		if err != nil {
			fmt.Printf("%s:%d: the example claims %d answer(s) but will not run: %v\n",
				ex.file, ex.line, len(ex.want), err)
			bad++
			continue
		}
		if len(got) != len(ex.want) {
			fmt.Printf("%s:%d: printed %d line(s), claims %d\n  printed: %v\n  claims:  %v\n",
				ex.file, ex.line, len(got), len(ex.want), got, ex.want)
			bad++
			continue
		}
		ok := true
		for i := range got {
			if strings.TrimSpace(got[i]) != ex.want[i] {
				fmt.Printf("%s:%d: answer %d is %q, the page says %q\n",
					ex.file, ex.line, i+1, strings.TrimSpace(got[i]), ex.want[i])
				ok = false
			}
		}
		if !ok {
			bad++
		} else if *verbose {
			fmt.Printf("%s:%d: %d answer(s) agree\n", ex.file, ex.line, len(ex.want))
		}
	}

	fmt.Printf("%d example(s) with %d claimed answer(s) over %d file(s); %d example(s) disagree\n",
		len(examples), claims, walked, bad)
	if bad > 0 {
		os.Exit(1)
	}
}

// runSelfTest checks the extraction on cases whose answer is known, because a
// pattern nobody tested is a guess and a guess that matches nothing looks
// exactly like a clean site. Returns the process exit status.
func runSelfTest() int {
	cases := []struct {
		name     string
		md       string
		wantEx   int      // examples found
		wantWant []string // the claims of the first one
	}{
		{
			name:     "a ruby block claiming one answer",
			md:       "text\n\n```ruby\np 1 + 1   # => 2\n```\n",
			wantEx:   1,
			wantWant: []string{"2"},
		},
		{
			name:     "set-up lines are kept and only claims are collected",
			md:       "```ruby\na = 1\np a       # => 1\np a + 1   # => 2\n```\n",
			wantEx:   1,
			wantWant: []string{"1", "2"},
		},
		{
			name:   "a block claiming nothing is skipped",
			md:     "```ruby\nImage.load(\"photo.png\").save(\"out.png\")\n```\n",
			wantEx: 0,
		},
		{
			name:   "a comment that is not an answer is not a claim",
			md:     "```ruby\np BigDecimal(\"0.1\")   # exact, no 0.30000000000000004\n```\n",
			wantEx: 0,
		},
		{
			name:   "a shell block is not Ruby",
			md:     "```sh\necho hi   # => hi\n```\n",
			wantEx: 0,
		},
		{
			name:     "a value containing a # is kept whole",
			md:       "```ruby\np \"a#b\"   # => \"a#b\"\n```\n",
			wantEx:   1,
			wantWant: []string{`"a#b"`},
		},
		{
			name:   "a bare # => claims nothing",
			md:     "```ruby\np 1   # =>\n```\n",
			wantEx: 0,
		},
		{
			name:     "a value is kept even with no space after the arrow",
			md:       "```ruby\np 1   #=>2\n```\n",
			wantEx:   1,
			wantWant: []string{"2"},
		},
	}
	bad := 0
	for _, c := range cases {
		got := extract("t.md", []byte(c.md))
		if len(got) != c.wantEx {
			fmt.Printf("selftest %q: found %d example(s), want %d\n", c.name, len(got), c.wantEx)
			bad++
			continue
		}
		if c.wantEx == 0 {
			continue
		}
		if strings.Join(got[0].want, "|") != strings.Join(c.wantWant, "|") {
			fmt.Printf("selftest %q: claims %v, want %v\n", c.name, got[0].want, c.wantWant)
			bad++
		}
	}
	fmt.Printf("selftest: %d case(s), %d failure(s)\n", len(cases), bad)
	if bad > 0 {
		return 1
	}
	return 0
}
