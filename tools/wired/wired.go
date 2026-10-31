// Package wired answers one question about this repository: is every tool
// under tools/ actually invoked by a workflow?
//
// Why it exists: the move from MkDocs to Hugo (#28) rewrote .github/workflows
// and dropped BOTH documentation guards. tools/conformancefigure, which checks
// that the conformance totals the site states agree with the gate, and
// tools/docexample, which runs every Ruby example and compares it with the
// answer written beside it, were still in the tree and called by nothing. The
// pages could have said anything for two releases and no build would have
// disagreed.
//
// Nothing caught it because nothing asserted it. A guard that is not wired is
// indistinguishable from a guard that passes: both are silent.
package wired

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Commands lists the tool directories under root. A directory counts when it
// holds Go source; wired itself is named explicitly rather than matched by
// that rule, because it is a library this test uses and not a tool a workflow
// should call.
func Commands(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "wired" {
			continue
		}
		has, err := hasGo(filepath.Join(root, e.Name()))
		if err != nil {
			return nil, err
		}
		if has {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

func hasGo(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			return true, nil
		}
	}
	return false, nil
}

// WorkflowText concatenates every workflow file, which is what a tool's name
// is looked for in.
func WorkflowText(dir string) (string, error) {
	var b strings.Builder
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if ext := filepath.Ext(p); ext != ".yml" && ext != ".yaml" {
			return nil
		}
		by, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		b.Write(by)
		b.WriteString("\n")
		return nil
	})
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

// Unwired returns the commands no workflow invokes.
//
// ⛔ It looks for `tools/<name>`, the PATH, not for the bare name.
//
// Measured 2026-10-31: `.github/workflows/docs.yml` names `docexample` twice —
// once on the `run:` line that invokes it, and once in a comment on line 108
// explaining what it judges. Against the bare name, deleting the invocation and
// leaving the comment would have kept that tool reported as wired. The one
// defect this package exists to catch would have been invisible for the one
// tool whose step carries prose about it.
//
// This stays deliberately crude — it still does not check for a well-formed
// step, because the failure it guards is not subtle (a tool nobody calls) and a
// crude test that would have failed #28 beats a precise one nobody wrote. But
// "appears in a sentence" and "appears in a command" are not the same claim, and
// `tools/` is what every spelling of the invocation has in common:
//
//	go run ./tools/upstreamlinks --root content/en
//	go run github.com/go-embedded-ruby/docs/tools/conformancefigure@v0.6.0
//
// while a prose comment does not. A workflow that invoked a tool some other way
// — through a variable, a matrix — would now read as unwired, which fails loudly
// rather than quietly, and that is the right direction for this guard to be
// wrong in.
func Unwired(commands []string, workflows string) []string {
	var out []string
	for _, c := range commands {
		if !strings.Contains(workflows, "tools/"+c) {
			out = append(out, c)
		}
	}
	return out
}
