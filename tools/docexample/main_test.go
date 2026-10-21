package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// ⛔ The extraction is a VOCABULARY, and a pattern nobody tested is a guess: a
// guess that matches nothing looks exactly like a site whose examples all
// agree. These were the tool's `-selftest` cases; they are a real test now, run
// by `go test ./...` in the same `build` job.
func TestExtraction(t *testing.T) {
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
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extract("t.md", []byte(c.md))
			if len(got) != c.wantEx {
				t.Fatalf("found %d example(s), want %d", len(got), c.wantEx)
			}
			if c.wantEx == 0 {
				return
			}
			if strings.Join(got[0].want, "|") != strings.Join(c.wantWant, "|") {
				t.Errorf("claims %v, want %v", got[0].want, c.wantWant)
			}
		})
	}
}

// ⛔ A refusal must not read as a pass: without an interpreter there is nothing
// to compare the examples against, and reporting success would be a pass it did
// not earn.
func TestRefusals(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"no interpreter given", []string{"--root", "testdata"}, 2},
		{"the interpreter is not there", []string{"--rbgo", "/nonexistent/rbgo", "--root", "testdata"}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := RootCmd()
			cmd.SetArgs(c.args)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			err := cmd.Execute()
			var ee *exitError
			if !errors.As(err, &ee) {
				t.Fatalf("error %v, want an exitError", err)
			}
			if ee.code != c.want {
				t.Errorf("status %d, want %d", ee.code, c.want)
			}
		})
	}
}

// A positional argument is a mistake, not an extra root: the flag is --root.
func TestAnArgumentIsRefused(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"content/en"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("a positional argument was accepted")
	}
}
