package surface

import (
	"strings"
	"testing"
)

// ⛔ The extraction is a VOCABULARY, and a vocabulary nobody tested is a guess.
// A guess that matches nothing looks exactly like a clean site, which is the
// one way these guards could go quiet about something that matters.
func TestNames(t *testing.T) {
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
			// ⛔ The page also says "63 of the 67 carry such a pairing". A
			// looser pattern would read that as the count and report a
			// disagreement nobody wrote.
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
			// A Windows checkout serves CRLF; a `$`-anchored reading of it
			// silently matches nothing, which has bitten this fleet before.
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
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := Section([]byte(c.md), c.want)
			got := Names(body)
			if strings.Join(got, " ") != strings.Join(c.names, " ") {
				t.Errorf("names %v, want %v", got, c.names)
			}
			n, ok := Count(body, c.patt)
			switch {
			case c.count < 0 && ok:
				t.Errorf("read a count of %d, want none", n)
			case c.count >= 0 && (!ok || n != c.count):
				t.Errorf("count %d (found=%v), want %d", n, ok, c.count)
			}
		})
	}
}

// ⛔ The three stale attributions of 2026-10-20 were all ordinary Markdown
// links, so the reader has to find them in that shape -- and must NOT turn a
// link into a tree or a file into a different repository, or a reference to one
// file of a dependency would read as a second, unknown dependency.
func TestRepos(t *testing.T) {
	cases := []struct {
		name    string
		md      string
		heading string
		want    []string
	}{
		{
			name:    "a Markdown link names its repository",
			md:      "## BigDecimal\n\nbinding\n[go-ruby-bigdecimal/bigdecimal](https://github.com/go-ruby-bigdecimal/bigdecimal).\n",
			heading: "BigDecimal",
			want:    []string{"go-ruby-bigdecimal/bigdecimal"},
		},
		{
			name:    "two links in one section, in order, deduplicated",
			md:      "## Time\n\n[a](https://github.com/go-composites/time) and [b](https://github.com/go-ruby-date/date) and [a](https://github.com/go-composites/time)\n",
			heading: "Time",
			want:    []string{"go-composites/time", "go-ruby-date/date"},
		},
		{
			name:    "a link into a tree still names the repository",
			md:      "## FFT\n\nsee [web/](https://github.com/go-embedded-ruby/ruby/tree/main/web)\n",
			heading: "FFT",
			want:    []string{"go-embedded-ruby/ruby"},
		},
		{
			name:    "a section with no link has none",
			md:      "## Set\n\nIt binds no outside library.\n",
			heading: "Set",
			want:    nil,
		},
		{
			name:    "a non-GitHub link is not a repository",
			md:      "## Special\n\nbuilt with [Hugo](https://gohugo.io/) and [x](https://example.com/a/b)\n",
			heading: "Special",
			want:    nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Repos(Section([]byte(c.md), c.heading))
			if strings.Join(got, " ") != strings.Join(c.want, " ") {
				t.Errorf("repos %v, want %v", got, c.want)
			}
		})
	}
}

// Every exemption carries a reason, because a bare list becomes a way of
// silencing a guard rather than a record of a judgement.
func TestEveryExemptionGivesAReason(t *testing.T) {
	for _, m := range Modules {
		for word, why := range m.Prose {
			if strings.TrimSpace(why) == "" {
				t.Errorf("%s exempts the word %q with no reason", m.Name(), word)
			}
		}
		for repo, why := range m.Links {
			if strings.TrimSpace(why) == "" {
				t.Errorf("%s exempts the link %q with no reason", m.Name(), repo)
			}
		}
	}
}

// A module needs at least one constant, or Name() is empty and every message
// about it says nothing.
func TestEveryModuleIsNamed(t *testing.T) {
	for i, m := range Modules {
		if len(m.Konsts) == 0 || m.Name() == "" {
			t.Errorf("module %d has no constant", i)
		}
		if m.Heading == "" {
			t.Errorf("%s has no heading to find it by", m.Name())
		}
	}
}
