package changelog

import (
	"regexp"
	"strings"
)

// versionHeadingRe matches a Keep a Changelog version heading: `## [1.2.3]` with
// an optional ` - 2026-06-27` date. The bracketed version is the slice key.
var versionHeadingRe = regexp.MustCompile(`^##\s+\[([^\]]+)\](?:\s*-\s*(\S+))?`)

// Document is a parsed CHANGELOG.md: the header preamble (everything before the
// first version heading) and the version sections in file order (newest first by
// convention, but parsing does not reorder).
type Document struct {
	Header   string
	Versions []ParsedVersion
}

// ParsedVersion is one `## [version]` block, retaining its Raw text so human
// edits survive an update untouched. Prose is the text between the heading and
// the first `###` group; Body is the whole block minus the heading (curated
// prose plus any groups), used as the curated release notes in notes mode;
// HasGroups reports whether any `###` group is present.
type ParsedVersion struct {
	Semver    string
	Date      string
	Raw       string
	Prose     string
	Body      string
	HasGroups bool
}

// ParseDocument splits a CHANGELOG.md into its header and version blocks. It never
// errors: malformed input simply yields whatever blocks it can recognize, so a
// hand-edited file degrades to fileless rather than breaking.
func ParseDocument(content string) Document {
	lines := strings.Split(content, "\n")
	var doc Document
	var headerEnd int
	for i, line := range lines {
		if versionHeadingRe.MatchString(line) {
			break
		}
		headerEnd = i + 1
	}
	doc.Header = strings.TrimRight(strings.Join(lines[:headerEnd], "\n"), "\n")

	var current *blockAccumulator
	flush := func() {
		if current != nil {
			doc.Versions = append(doc.Versions, current.build())
			current = nil
		}
	}
	for _, line := range lines[headerEnd:] {
		if m := versionHeadingRe.FindStringSubmatch(line); m != nil {
			flush()
			current = &blockAccumulator{semver: m[1], date: m[2], heading: line}
			continue
		}
		if current != nil {
			current.lines = append(current.lines, line)
		}
	}
	flush()
	return doc
}

// Find returns the parsed version with the given semver, if present.
func (d Document) Find(semver string) (ParsedVersion, bool) {
	for _, v := range d.Versions {
		if v.Semver == semver {
			return v, true
		}
	}
	return ParsedVersion{}, false
}

// Has reports whether a version with the given semver is already in the document.
func (d Document) Has(semver string) bool {
	_, ok := d.Find(semver)
	return ok
}

type blockAccumulator struct {
	semver  string
	date    string
	heading string
	lines   []string
}

var groupHeadingRe = regexp.MustCompile(`^###\s+`)

func (b *blockAccumulator) build() ParsedVersion {
	body := strings.Trim(strings.Join(b.lines, "\n"), "\n")
	hasGroups := false
	var prose []string
	for _, line := range strings.Split(body, "\n") {
		if groupHeadingRe.MatchString(line) {
			hasGroups = true
			break
		}
		prose = append(prose, line)
	}
	// keep the blank line after the heading so a raw passthrough matches what
	// RenderVersion emits, making the update idempotent.
	raw := b.heading
	if body != "" {
		raw += "\n\n" + body
	}
	return ParsedVersion{
		Semver:    b.semver,
		Date:      b.date,
		Raw:       raw,
		Prose:     strings.Trim(strings.Join(prose, "\n"), "\n"),
		Body:      body,
		HasGroups: hasGroups,
	}
}
