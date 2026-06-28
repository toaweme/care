package changelog

import (
	"regexp"
	"sort"
)

// DefaultGroups is the org's section set and order, mirroring care's
// .goreleaser.yml groups exactly (same titles, same prefix regexes). Nothing is
// excluded: every commit type carries useful info, so unmatched commits fall to
// the default "Chores & Other" group rather than being dropped.
var DefaultGroups = []Group{
	{Title: "Features", Match: `^.*?feat(\(.+\))??!?:.+$`, Order: 0},
	{Title: "Fixes", Match: `^.*?fix(\(.+\))??!?:.+$`, Order: 1},
	{Title: "Documentation", Match: `^.*?docs(\(.+\))??!?:.+$`, Order: 2},
	{Title: "Refactors", Match: `^.*?(refactor|ref)(\(.+\))??!?:.+$`, Order: 3},
	{Title: "Tests", Match: `^.*?test(\(.+\))??!?:.+$`, Order: 4},
	{Title: "CI & Build", Match: `^.*?(ci|build)(\(.+\))??!?:.+$`, Order: 5},
	{Title: "Chores & Other", Match: "", Order: 999},
}

// Grouper assigns commits to sections using the configured groups, with each
// group's match regexp compiled once at construction and reused per commit.
type Grouper struct {
	groups []Group
	res    []*regexp.Regexp
}

// NewGrouper compiles the groups into a reusable grouper. The default (empty
// Match) group catches everything unmatched; its regexp slot is nil.
func NewGrouper(groups []Group) *Grouper {
	ordered := append([]Group(nil), groups...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Order < ordered[j].Order })
	res := make([]*regexp.Regexp, len(ordered))
	for i, g := range ordered {
		if g.Match != "" {
			res[i] = regexp.MustCompile(g.Match)
		}
	}
	return &Grouper{groups: ordered, res: res}
}

// Group sorts commits into sections in group order, dropping empty sections.
// Each commit lands in the first group whose regexp matches its subject, or the
// default group when none do.
func (g *Grouper) Group(commits []Commit) []Section {
	buckets := make([][]Commit, len(g.groups))
	defaultIdx := -1
	for i, re := range g.res {
		if re == nil {
			defaultIdx = i
		}
	}
	for _, c := range commits {
		idx := defaultIdx
		for i, re := range g.res {
			if re != nil && re.MatchString(c.Subject) {
				idx = i
				break
			}
		}
		if idx < 0 {
			// no default group configured: skip unmatched commits.
			continue
		}
		buckets[idx] = append(buckets[idx], c)
	}
	var sections []Section
	for i, bucket := range buckets {
		if len(bucket) == 0 {
			continue
		}
		sections = append(sections, Section{Title: g.groups[i].Title, Commits: bucket})
	}
	return sections
}
