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
		t.Errorf("no workflow mentions %v.\n"+
			"A tool nothing calls is indistinguishable from a tool that passes: both are "+
			"silent. Either wire it into .github/workflows or delete it; leaving it is the "+
			"state that let the MkDocs-to-Hugo move (#28) drop both documentation guards "+
			"without a red build.", bad)
		return
	}
	t.Logf("%d tool(s) under tools/, all named by a workflow: %v", len(cmds), cmds)
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
