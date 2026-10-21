// Package surface is what the site SAYS about rbgo's module surface, and how
// to read it out of the pages.
//
// It exists because two guards ask different questions of the same sections:
// tools/numericsurface asks a real interpreter whether every method name the
// prose prints is one it answers, and tools/upstreamlinks asks rbgo's go.mod
// whether every library a section links to is one rbgo still depends on. A
// second copy of the section reader would drift from the first, and the module
// table would end up listing different sections to each guard -- so both live
// here, once.
package surface

import (
	"regexp"
	"strconv"
	"strings"
)

// Module is one `## ` section of the site and what is known about it.
type Module struct {
	// Konsts are the Ruby constants this section documents. It is a LIST
	// because a section may cover more than one -- "Time & Date" teaches both,
	// and a reader meets `leap?` and `Time.at` in the same prose. Checking
	// each constant against its own section would report every one of the
	// other's names as unknown, which is a defect in the table, not the page.
	Konsts  []string
	Heading string // what its `## ` heading contains

	// Instances are Ruby expressions whose INSTANCE methods also count as
	// names of this module's surface. Stats documents `pdf`/`cdf`/`ppf` in its
	// own section although they belong to the distribution object it answers,
	// and a reader meets them there; so the check has to know about them.
	Instances []string

	// Counted is a phrase on the page stating how many names the module has,
	// e.g. "66 Ruby names". Empty when the page states no count. When it is
	// there, the figure must equal the number of SINGLETON names the section's
	// constants answer between them.
	Counted string

	// Prose exempts a backticked word that is not a method of this module,
	// with the reason it appears. A bare list would rot into a way of silencing
	// the guard; a reason makes each one reviewable.
	Prose map[string]string

	// Links exempts a GitHub link in this section that is not a claim about
	// which library the module binds -- a reference to MRI's source, say.
	// Same discipline: each carries its reason.
	Links map[string]string
}

// Name is how a section is called in a message.
func (m Module) Name() string { return strings.Join(m.Konsts, "/") }

// Modules is every section of the site that documents a Ruby module or class.
//
// ⛔ "Numeric core" (Complex, Rational, Math) is deliberately absent: its
// heading names no constant, so the section reader cannot find it, and three
// table rows pointing at one heading would each report the other two's names as
// unknown. Covering it needs a different key than a heading substring, which is
// a change to this file rather than a row in this list.
var Modules = []Module{
	{
		Konsts:  []string{"Special"},
		Heading: "Special",
		Counted: `(\d+) Ruby names`,
	},
	{
		Konsts:    []string{"Stats"},
		Heading:   "Stats",
		Instances: []string{"Stats.norm", "Stats.poisson(3.0)"},
		Counted:   `(\d+) univariate distributions`,
		Prose: map[string]string{
			"loc":   "scipy's ARGUMENT name, in the sentence giving the argument order",
			"scale": "scipy's ARGUMENT name, in the sentence giving the argument order",
		},
	},
	{
		Konsts:  []string{"Integrate"},
		Heading: "Integrate",
		Prose: map[string]string{
			"round": "Float#round, which the FMA paragraph says the two quadrature examples carry",
		},
	},
	{
		Konsts:  []string{"Optimize"},
		Heading: "Optimize",
	},
	{
		Konsts:    []string{"NDArray"},
		Heading:   "NDArray",
		Instances: []string{"NDArray.arange(0, 6)"},
	},
	{
		Konsts:  []string{"FFT"},
		Heading: "FFT",
	},
	{
		Konsts:    []string{"Image"},
		Heading:   "Image",
		Instances: []string{"Image.new(2, 2)"},
	},
	{
		Konsts:    []string{"Set"},
		Heading:   "Set",
		Instances: []string{"Set.new([1, 2])"},
	},
	{
		Konsts:    []string{"Bag"},
		Heading:   "Bag",
		Instances: []string{"Bag.new"},
	},
	{
		// One section, two constants: see the comment on Konsts.
		Konsts:    []string{"Time", "Date"},
		Heading:   "Time",
		Instances: []string{"Time.now", "Date.new(2026, 6, 21)"},
	},
	{
		Konsts:    []string{"BigDecimal"},
		Heading:   "BigDecimal",
		Instances: []string{"BigDecimal(\"1.5\")"},
	},
}

// candidate is a backticked token that claims to be a method name: one bare
// lower-case word, optionally ending in ? or !. Anything with a dot, slash,
// colon, space, bracket, capital or digit-leading shape is not one.
var candidate = regexp.MustCompile("`([a-z][a-z0-9_]*[?!]?)`")

// heading matches a level-2 ATX heading and captures its text.
var heading = regexp.MustCompile(`^##\s+(.*)$`)

// repoLink matches a GitHub repository URL and captures owner/name. The
// trailing group stops at the repository, so a link into a tree or a file
// still names the repository it belongs to.
var repoLink = regexp.MustCompile(`https://github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)`)

// Section returns the lines of the `## ` section whose heading contains want.
//
// ⛔ CRLF is folded first. A Windows checkout serves it, and a `$`-anchored
// reading of such a file silently matches nothing -- which has cost this fleet
// a red lane before.
func Section(md []byte, want string) []string {
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

// Names collects the candidate method names of a section, deduplicated and in
// the order a reader meets them.
func Names(body []string) []string {
	return dedup(body, candidate, func(m []string) string { return m[1] })
}

// Repos collects the GitHub repositories a section links to, as "owner/name",
// deduplicated and in the order a reader meets them.
func Repos(body []string) []string {
	return dedup(body, repoLink, func(m []string) string { return m[1] + "/" + m[2] })
}

func dedup(body []string, re *regexp.Regexp, pick func([]string) string) []string {
	seen := map[string]bool{}
	var out []string
	for _, l := range body {
		for _, m := range re.FindAllStringSubmatch(l, -1) {
			v := pick(m)
			if !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
	}
	return out
}

// Count reads the figure a section states about itself, if it states one.
func Count(body []string, pattern string) (int, bool) {
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
