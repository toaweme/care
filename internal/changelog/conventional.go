package changelog

import (
	"regexp"
	"strings"
	"unicode"
)

// conventionalRe parses the type, optional scope, and breaking marker off a
// conventional-commit subject. A subject without a recognized prefix yields no
// match and falls to the default group.
var conventionalRe = regexp.MustCompile(`^(\w+)(?:\(([^)]*)\))?(!)?:\s*(.*)$`)

// prRe matches a trailing pull-request reference like " (#42)", as appended by a
// GitHub squash merge. The captured digits are the PR number.
var prRe = regexp.MustCompile(`\s*\(#(\d+)\)\s*$`)

// Parse fills a commit's conventional fields (Type, Scope, Breaking, Desc, PR)
// from its subject. It is idempotent and leaves Type empty for non-conventional
// subjects, in which case Desc is the whole subject (sans any PR reference).
func Parse(c *Commit) {
	desc := c.Subject
	m := conventionalRe.FindStringSubmatch(c.Subject)
	if m != nil {
		c.Type = strings.ToLower(m[1])
		c.Scope = m[2]
		c.Breaking = m[3] == "!"
		desc = m[4]
	}
	c.Breaking = c.Breaking || strings.Contains(c.Subject, "BREAKING CHANGE")
	if pr := prRe.FindStringSubmatch(desc); pr != nil {
		c.PR = pr[1]
		desc = prRe.ReplaceAllString(desc, "")
	}
	c.Desc = capitalize(strings.TrimSpace(desc))
}

// capitalize upper-cases the first rune of s, leaving the rest untouched so
// acronyms and identifiers keep their casing.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
