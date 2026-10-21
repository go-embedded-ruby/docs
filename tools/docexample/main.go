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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
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

// exitError carries the status the command means, so the documented 0/1/2/3 are
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
		Use:   "docexample",
		Short: "Run the Ruby examples on this site and check each against its stated answer",
		Long: "docexample finds every fenced ruby block that claims an output in a `# =>`\n" +
			"comment, runs the block as one program, and compares the lines it prints,\n" +
			"in order, with the answers it claims. A block that claims nothing is\n" +
			"skipped, which is what makes the selection self-evident.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return check(cmd.OutOrStdout(), rbgo, root, verbose)
		},
	}
	cmd.Flags().StringVar(&rbgo, "rbgo", "", "path to an rbgo binary (required)")
	cmd.Flags().StringVar(&root, "root", "content/en", "directory of Markdown files to read")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "also list the examples that agree")
	return cmd
}

// check is the whole decision. Findings go to stdout, as they did before: they
// are the tool's answer, not its diagnostics.
func check(stdout io.Writer, rbgo, root string, verbose bool) error {
	if rbgo == "" {
		return &exitError{2, "--rbgo is required: without an interpreter this cannot check anything,\n" +
			"and reporting success would be a pass it did not earn."}
	}
	if _, err := os.Stat(rbgo); err != nil {
		return &exitError{2, fmt.Sprintf("--rbgo %q: %v", rbgo, err)}
	}

	var examples []example
	walked := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
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
		return &exitError{2, fmt.Sprintf("walking %s: %v", root, err)}
	}
	if walked == 0 {
		return &exitError{3, fmt.Sprintf("read no Markdown under %s -- nothing was checked", root)}
	}
	if len(examples) == 0 {
		return &exitError{3, fmt.Sprintf("%d Markdown file(s) and no example claiming an answer.\n"+
			"For a site that teaches by example that means the extraction broke, not that it is clean.", walked)}
	}

	bad := 0
	claims := 0
	for _, ex := range examples {
		claims += len(ex.want)
		got, err := run(rbgo, ex)
		if err != nil {
			fmt.Fprintf(stdout, "%s:%d: the example claims %d answer(s) but will not run: %v\n",
				ex.file, ex.line, len(ex.want), err)
			bad++
			continue
		}
		if len(got) != len(ex.want) {
			fmt.Fprintf(stdout, "%s:%d: printed %d line(s), claims %d\n  printed: %v\n  claims:  %v\n",
				ex.file, ex.line, len(got), len(ex.want), got, ex.want)
			bad++
			continue
		}
		ok := true
		for i := range got {
			if strings.TrimSpace(got[i]) != ex.want[i] {
				fmt.Fprintf(stdout, "%s:%d: answer %d is %q, the page says %q\n",
					ex.file, ex.line, i+1, strings.TrimSpace(got[i]), ex.want[i])
				ok = false
			}
		}
		if !ok {
			bad++
		} else if verbose {
			fmt.Fprintf(stdout, "%s:%d: %d answer(s) agree\n", ex.file, ex.line, len(ex.want))
		}
	}

	fmt.Fprintf(stdout, "%d example(s) with %d claimed answer(s) over %d file(s); %d example(s) disagree\n",
		len(examples), claims, walked, bad)
	if bad > 0 {
		return &exitError{1, fmt.Sprintf("%d example(s) disagree", bad)}
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
		fmt.Fprintf(os.Stderr, "docexample: %s\n", ee.msg)
		osExit(ee.code)
		return
	}
	// A cobra error -- an unknown flag, an argument where none is taken. It is
	// a refusal to run, not a verdict on the pages, so it exits 2.
	fmt.Fprintf(os.Stderr, "docexample: %v\n", err)
	osExit(2)
}
