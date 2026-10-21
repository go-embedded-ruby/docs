// upstreamlinks checks that every library a module section links to is one
// rbgo still depends on.
//
// Why it exists: on 2026-10-20, 3 of the 12 upstream links on the "Scientific
// stack" page were wrong, and each named a library rbgo had STOPPED using --
// `BigDecimal` said go-composites/bigfloat when the binding had moved to
// go-ruby-bigdecimal/bigdecimal, `Date` said go-composites/date when it had
// moved to go-ruby-date/date, and `Set` said go-composites/set when it binds
// nothing at all and is implemented in rbgo over its own Hash.
//
// An upstream swap is INVISIBLE from this repository. The commit that makes it
// has no reason to touch another repository's prose, and nothing fails when it
// does not. Both stale links still resolved to real repositories, so a reader
// followed one and landed somewhere plausible and wrong.
//
// WHAT IT JUDGES, and why go.mod rather than a table here. A table of "which
// library each module binds" would be maintained by hand in this repository,
// which only moves the problem: a table repeating the page's mistake agrees
// with it. rbgo's go.mod is the tie-breaker precisely because a library that
// gets swapped out DISAPPEARS from it -- which is what makes all three of the
// real cases detectable without anybody writing the right answer down.
//
// It is a necessary condition, not a sufficient one: a page could name a
// library that rbgo requires for an unrelated reason, and this would accept it.
// Catching that needs the importing package, not the module graph. Said here so
// the limit is known rather than discovered.
//
// Exit status: 0 every link is a current dependency; 1 one or more are not;
// 2 could not read rbgo's go.mod -- a refusal to answer, not a pass; 3 no link
// was found at all, which for a page that attributes every module means the
// extraction broke rather than that the page is clean.
package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"

	"github.com/go-embedded-ruby/docs/internal/surface"
)

// goModURL is rbgo's go.mod on its default branch. The dependency set is read
// from the file the build uses, not from a release note or a badge.
const goModURL = "https://raw.githubusercontent.com/go-embedded-ruby/ruby/main/go.mod"

// exitError carries the status the command means, so the documented 1/2/3 are
// not flattened into cobra's single "an error happened" exit of 1.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

// deps is what rbgo's go.mod says: its own module path, and the modules it
// requires directly.
type deps struct {
	main   string
	direct map[string]bool
}

// parse reads a go.mod with the module-file parser rather than by looking for
// lines. A require block, a single-line require and a `// indirect` marker are
// all different shapes, and a hand-rolled reader gets the last one wrong in the
// direction that matters here -- silently accepting an indirect dependency as
// evidence that a binding exists.
func parse(name string, b []byte) (deps, error) {
	f, err := modfile.Parse(name, b, nil)
	if err != nil {
		return deps{}, err
	}
	d := deps{direct: map[string]bool{}}
	if f.Module != nil {
		d.main = f.Module.Mod.Path
	}
	for _, r := range f.Require {
		if !r.Indirect {
			d.direct[r.Mod.Path] = true
		}
	}
	if len(d.direct) == 0 {
		// A go.mod with no direct requirement would make every link look
		// wrong, which is a worse answer than refusing.
		return deps{}, errors.New("no direct requirement found; this is not rbgo's go.mod")
	}
	return d, nil
}

// fetch reads rbgo's go.mod, from a path when one is given and over HTTP
// otherwise.
func fetch(path string, timeout time.Duration) (deps, error) {
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return deps{}, err
		}
		return parse(path, b)
	}
	cl := &http.Client{Timeout: timeout}
	resp, err := cl.Get(goModURL)
	if err != nil {
		return deps{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// ⛔ An error page is still a body: without this the parser would read
		// GitHub's 404 HTML, fail, and the message would blame the format.
		return deps{}, fmt.Errorf("%s: HTTP %d", goModURL, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return deps{}, err
	}
	return parse(goModURL, b)
}

func RootCmd() *cobra.Command {
	var (
		root    string
		gomod   string
		timeout time.Duration
	)
	cmd := &cobra.Command{
		Use:   "upstreamlinks",
		Short: "Check that every library a module section links to is one rbgo still depends on",
		Long: "upstreamlinks reads the module sections of this site, collects the GitHub\n" +
			"repositories they link to, and checks each one against rbgo's go.mod. A\n" +
			"library that was swapped out disappears from go.mod, which is what makes a\n" +
			"stale attribution detectable without anybody writing the right answer down.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return check(cmd.OutOrStdout(), cmd.ErrOrStderr(), root, gomod, timeout)
		},
	}
	cmd.Flags().StringVar(&root, "root", "content/en", "directory of Markdown files to read")
	cmd.Flags().StringVar(&gomod, "gomod", "", "read rbgo's go.mod from this path instead of fetching it")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "how long to wait for the fetch")
	return cmd
}

func check(stdout, stderr io.Writer, root, gomod string, timeout time.Duration) error {
	d, err := fetch(gomod, timeout)
	if err != nil {
		return &exitError{2, fmt.Sprintf("cannot read rbgo's dependencies: %v", err)}
	}

	type page struct {
		path string
		body []byte
	}
	var pages []page
	err = filepath.WalkDir(root, func(p string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if de.IsDir() || filepath.Ext(p) != ".md" {
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
	for _, m := range surface.Modules {
		for _, pg := range pages {
			body := surface.Section(pg.body, m.Heading)
			if body == nil {
				continue
			}
			sections++
			for _, repo := range surface.Repos(body) {
				path := "github.com/" + repo
				checked++
				switch {
				case path == d.main:
					// rbgo itself, not a claim about an upstream library.
				case d.direct[path]:
				case m.Links[repo] != "":
				default:
					fmt.Fprintf(stderr, "%s: the %s section links to %s, which rbgo does not depend on\n",
						pg.path, m.Name(), path)
					bad++
				}
			}
		}
	}

	// "N of M, never N alone": a walk that matched no link reports no
	// disagreement, which is the same output as a correct page.
	fmt.Fprintf(stdout, "%d upstream link(s) checked over %d module section(s) against rbgo's go.mod; %d disagree\n",
		checked, sections, bad)
	if sections == 0 || checked == 0 {
		return &exitError{3, fmt.Sprintf("found no upstream link under %s; "+
			"either the headings changed or the pages lost their attributions", root)}
	}
	if bad > 0 {
		return &exitError{1, fmt.Sprintf("%d stale attribution(s)", bad)}
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
		fmt.Fprintf(os.Stderr, "upstreamlinks: %s\n", ee.msg)
		osExit(ee.code)
		return
	}
	// A cobra error -- an unknown flag, an argument where none is taken. It is
	// a refusal to run, not a verdict on the pages, so it exits 2.
	fmt.Fprintf(os.Stderr, "upstreamlinks: %v\n", err)
	osExit(2)
}
