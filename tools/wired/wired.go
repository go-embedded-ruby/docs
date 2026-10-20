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

// Unwired returns the commands no workflow mentions.
func Unwired(commands []string, workflows string) []string {
	var out []string
	for _, c := range commands {
		if !strings.Contains(workflows, c) {
			out = append(out, c)
		}
	}
	return out
}
