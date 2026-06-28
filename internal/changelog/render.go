package changelog

import (
	"fmt"
	"strings"
)

// defaultHeader tops a generated CHANGELOG.md. It is emitted only when the
// existing file has no header of its own; any header already present is preserved
// verbatim, so a hand-written preamble is never overwritten.
const defaultHeader = `# Changelog

All notable changes to this project are documented here, newest first.

Entries are generated from [Conventional Commits](https://www.conventionalcommits.org)
and grouped by change type. This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
`

// shortHashLen is how many leading hex characters of a commit hash render as the
// linked short SHA, matching git's default abbreviation.
const shortHashLen = 7

// Renderer turns grouped commits into Keep a Changelog markdown. A host supplies
// the commit and pull-request links; plain mode drops every link and author
// attribution, leaving only the cleaned subjects for non-git publishing targets.
type Renderer struct {
	host  GitHost
	plain bool
}

// NewRenderer builds a renderer. host may be nil (no links); plain forces a
// link-free, attribution-free body even when a host is present.
func NewRenderer(host GitHost, plain bool) *Renderer {
	return &Renderer{host: host, plain: plain}
}

// RenderSections renders the grouped commit lists as Keep a Changelog ### blocks.
// It is the shared body used by both the file sections and the extracted notes.
func (r *Renderer) RenderSections(sections []Section) string {
	var b strings.Builder
	for i, section := range sections {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "### %s\n\n", section.Title)
		for _, c := range section.Commits {
			b.WriteString(r.renderCommit(c))
		}
	}
	return b.String()
}

// renderCommit renders one bullet in the prose style: an optional breaking
// marker, an optional bold scope tag, the cleaned and capitalized subject, then
// (unless plain) the author after "by" and the reference after "in":
// "- Subject by @author in #42".
func (r *Renderer) renderCommit(c Commit) string {
	line := "- "
	if c.Breaking {
		line += "**[breaking]** "
	}
	if c.Scope != "" {
		line += "**" + capitalize(c.Scope) + ":** "
	}
	line += r.subject(c)
	if r.plain {
		return strings.TrimRight(line, " ") + "\n"
	}
	if who := r.author(c); who != "" {
		line += " by " + who
	}
	if ref := r.ref(c); ref != "" {
		line += " in " + ref
	}
	return line + ".\n"
}

// author renders the commit author: the handle linked to its host profile when
// both are known (a bare @handle without a host), else the plain git author name.
func (r *Renderer) author(c Commit) string {
	if c.Handle != "" {
		if r.host != nil {
			if url := r.host.UserURL(c.Handle); url != "" {
				return fmt.Sprintf("[@%s](%s)", c.Handle, url)
			}
		}
		return "@" + c.Handle
	}
	return c.Author
}

// subject is the rendered commit description: the cleaned Desc when parsing found
// one, falling back to the raw subject for anything unparsed.
func (r *Renderer) subject(c Commit) string {
	if c.Desc != "" {
		return c.Desc
	}
	return strings.TrimSpace(c.Subject)
}

// ref renders the change's reference for the "in <ref>" suffix: the linked PR
// when known (a bare "#NNN" without a host), else the linked short SHA, else "".
func (r *Renderer) ref(c Commit) string {
	if c.PR != "" {
		if r.host != nil {
			if url := r.host.PRURL(c.PR); url != "" {
				return fmt.Sprintf("[#%s](%s)", c.PR, url)
			}
		}
		return "#" + c.PR
	}
	return r.commitRef(c)
}

// commitRef renders the short-SHA reference as a markdown link, or nothing when
// no host (or no hash) can form one.
func (r *Renderer) commitRef(c Commit) string {
	if c.Hash == "" || r.host == nil {
		return ""
	}
	url := r.host.CommitURL(c.Hash)
	if url == "" {
		return ""
	}
	return fmt.Sprintf("[%s](%s)", shortHash(c.Hash), url)
}

// shortHash abbreviates a commit hash to its leading characters, leaving shorter
// hashes untouched.
func shortHash(hash string) string {
	if len(hash) <= shortHashLen {
		return hash
	}
	return hash[:shortHashLen]
}

// RenderVersion renders a full version section for CHANGELOG.md: the
// `## [semver] - date` heading, any preserved human prose, then the grouped
// sections.
func (r *Renderer) RenderVersion(v Version) string {
	var b strings.Builder
	b.WriteString(versionHeading(v))
	b.WriteString("\n\n")
	if prose := strings.TrimSpace(v.Prose); prose != "" {
		b.WriteString(prose)
		b.WriteString("\n\n")
	}
	b.WriteString(r.RenderSections(v.Sections))
	return b.String()
}

func versionHeading(v Version) string {
	heading := "## [" + v.Semver + "]"
	if v.Date != "" {
		heading += " - " + v.Date
	}
	return heading
}

// RenderNotes renders a single version's release notes for stdout / a notes file:
// the human prose (when present), the grouped sections, then the host extras.
// Used for the fileless path, where the version is derived from git.
func (r *Renderer) RenderNotes(v Version, extras Extras) string {
	var b strings.Builder
	if prose := strings.TrimSpace(v.Prose); prose != "" {
		b.WriteString(prose)
		b.WriteString("\n\n")
	}
	b.WriteString(r.RenderSections(v.Sections))
	return AppendExtras(b.String(), extras)
}

// AppendExtras appends the host extras (New Contributors, Full Changelog link)
// beneath a release-notes body. Extras with empty fields are dropped, so the
// fileless and no-host paths degrade cleanly. The body is whatever notes text
// was sliced from CHANGELOG.md or rendered from git.
func AppendExtras(body string, extras Extras) string {
	b := strings.Builder{}
	b.WriteString(strings.TrimRight(body, "\n"))
	if len(extras.NewContributors) > 0 {
		b.WriteString("\n\n## New Contributors\n\n")
		for _, handle := range extras.NewContributors {
			fmt.Fprintf(&b, "- @%s made their first contribution\n", strings.TrimPrefix(handle, "@"))
		}
	}
	if extras.CompareURL != "" {
		fmt.Fprintf(&b, "\n\n**Full Changelog**: %s\n", extras.CompareURL)
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}
