package wired_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-embedded-ruby/docs/tools/wired"
)

// TestEveryToolIsWiredIntoAWorkflow is the gate.
//
// It reads the tool directories and the workflow files, and fails when a tool
// is named by none of them. That is deliberately a CRUDE test -- it looks for
// the directory's name in the YAML, not for a correctly formed step -- because
// the failure it exists for is not a subtle one: a tool nobody calls. A crude
// test that would have failed #28 beats a precise one nobody wrote.
func TestEveryToolIsWiredIntoAWorkflow(t *testing.T) {
	root := repoRoot(t)

	cmds, err := wired.Commands(filepath.Join(root, "tools"))
	if err != nil {
		t.Fatalf("reading tools/: %v", err)
	}
	if len(cmds) == 0 {
		t.Fatal("found no tools under tools/ -- this test would then pass by having " +
			"nothing to check, which is how the guards went missing in the first place")
	}

	wf, err := wired.WorkflowText(filepath.Join(root, ".github", "workflows"))
	if err != nil {
		t.Fatalf("reading .github/workflows: %v", err)
	}
	if strings.TrimSpace(wf) == "" {
		t.Fatal("read no workflow text -- every tool would then look unwired")
	}

	bad := wired.Unwired(cmds, wf)
	if len(bad) > 0 {
		t.Errorf("no workflow invokes %v (looked for `tools/<name>`, not the bare name:\n"+
			"a tool named in a comment is documented, not called).\n"+
			"A tool nothing calls is indistinguishable from a tool that passes: both are "+
			"silent. Either wire it into .github/workflows or delete it; leaving it is the "+
			"state that let the MkDocs-to-Hugo move (#28) drop both documentation guards "+
			"without a red build.", bad)
		return
	}
	t.Logf("%d tool(s) under tools/, all invoked by a workflow: %v", len(cmds), cmds)
}

// TestUnwiredNamesOnlyWhatIsMissing checks the rule itself against literals, so
// the gate above cannot pass because the comparison is broken.
func TestUnwiredNamesOnlyWhatIsMissing(t *testing.T) {
	const yaml = "  - run: go run ./tools/docexample -root content/en\n"
	got := wired.Unwired([]string{"docexample", "conformancefigure"}, yaml)
	if len(got) != 1 || got[0] != "conformancefigure" {
		t.Errorf("Unwired = %v, want [conformancefigure]", got)
	}
	if n := len(wired.Unwired([]string{"docexample"}, yaml)); n != 0 {
		t.Errorf("a wired tool was reported as unwired (%d)", n)
	}
	if n := len(wired.Unwired([]string{"docexample", "conformancefigure"}, "")); n != 2 {
		t.Errorf("with no workflow text, %d of 2 tools looked wired", 2-n)
	}
}

// ⛔ A NAME IN A SENTENCE IS NOT AN INVOCATION, and this repository already
// contains the sentence: docs.yml:108 reads
//
//	# docexample judges the examples that claim an answer -- 11 of them. The
//
// so against the bare name, deleting the `go run ./tools/docexample` step and
// leaving that comment kept the tool reported as wired — the exact blindness
// this package exists to remove, for the one tool whose step is documented.
//
// The two halves are asserted together: the comment alone must NOT count, and
// the invocation must still count, so a rule tightened into matching nothing
// cannot pass this.
func TestAMentionInProseIsNotAnInvocation(t *testing.T) {
	const comment = "      # docexample judges the examples that claim an answer -- 11 of them. The\n"
	if got := wired.Unwired([]string{"docexample"}, comment); len(got) != 1 {
		t.Errorf("a tool named only in a comment was reported as wired (Unwired = %v)", got)
	}

	// The real spellings of an invocation, each of which must count.
	for _, yaml := range []string{
		"        run: go run ./tools/docexample --rbgo /tmp/rbgo --root content/en\n",
		"        run: go run github.com/go-embedded-ruby/docs/tools/docexample@v0.6.0\n",
		"        run: cd tools/docexample && go run .\n",
	} {
		if got := wired.Unwired([]string{"docexample"}, yaml); len(got) != 0 {
			t.Errorf("an invoked tool was reported as unwired (%v) for:\n%s", got, yaml)
		}
	}

	// And the comment beside a real invocation is the repository's actual state:
	// wired because of the step, not because of the prose.
	both := "        run: go run ./tools/docexample --root content/en\n" + comment
	if got := wired.Unwired([]string{"docexample"}, both); len(got) != 0 {
		t.Errorf("Unwired = %v, want the step to count", got)
	}
}

// repoRoot walks up to the directory holding go.mod, so the test runs from
// wherever `go test ./...` places it.
func repoRoot(t *testing.T) string {
	t.Helper()
	d, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			t.Fatal("no go.mod above the test's directory")
		}
		d = parent
	}
}
