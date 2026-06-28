// Package changelog derives conventional-commit release notes from git, with an
// optional Keep a Changelog CHANGELOG.md as persisted state. It is the org's
// single source of release notes, replacing goreleaser's built-in changelog so
// CLIs and libraries (which can't all use goreleaser) emit identical output.
//
// Notes are always derivable from git, so CHANGELOG.md is optional: the engine
// runs fileless (derive straight from tags + commits) or file-as-source-of-truth
// (maintain a CHANGELOG.md and slice notes back out of it). Everything
// host-specific sits behind the GitHost interface; the git-log path is the
// host-neutral default and always works.
package changelog

import "context"

// Commit is one conventional-style commit in a version's range. Handle is the
// host author handle (@user) when a GitHost enriched the range; Author is the git
// author name, the host-neutral fallback used when no handle is known. Desc is the
// subject with its conventional prefix and trailing PR reference stripped; PR is
// the pull-request number (digits only) when the subject ended with "(#NNN)".
type Commit struct {
	Hash     string
	Subject  string
	Author   string
	Handle   string
	Type     string
	Scope    string
	Desc     string
	PR       string
	Breaking bool
}

// Section is a titled group of commits within a version, e.g. "Features".
type Section struct {
	Title   string
	Commits []Commit
}

// Version is one release: its tag (possibly namespaced like server/v1.2.3), the
// bare semver, the release date, any human-authored Prose carried over from a
// CHANGELOG.md, and the generated Sections.
type Version struct {
	Tag      string
	Semver   string
	Date     string
	Prose    string
	Sections []Section
}

// Extras are the host-synthesized additions that `gh release create
// --generate-notes` appends but a curated CHANGELOG.md omits: the compare link
// and the first-time contributors in the range. Empty fields are dropped.
type Extras struct {
	CompareURL      string
	NewContributors []string
}

// Group classifies a commit subject into a titled section by regexp, in a fixed
// render order. The default group (Order highest, empty Match) catches the rest.
type Group struct {
	Title string
	Match string
	Order int
}

// Remote identifies a repository's git host: its host name and owner/repo. An
// unknown host still parses (host set, owner/repo filled when derivable) so the
// caller can decide to degrade to the git-log path.
type Remote struct {
	Host  string
	Owner string
	Repo  string
}

// GitHost is the small host-specific provider. The git-log backend is the
// host-neutral default and always available; a GitHost enriches a range with
// author handles and synthesizes the compare link and new-contributor list. An
// unknown host (or a failure) degrades to the git-log path rather than breaking.
type GitHost interface {
	// CompareCommits returns the commits in (from, to] enriched with author
	// handles. from is empty for a repo's first release (no prior tag).
	CompareCommits(ctx context.Context, from, to string) ([]Commit, error)
	// CompareURL returns the host's compare/{from}...{to} link, or "" when one
	// can't be formed (e.g. no prior tag).
	CompareURL(from, to string) string
	// TagURL returns the host's web link to a tag's release page, or "" when one can't be formed.
	// Used for the first release, which has no prior tag and so no range to compare against.
	TagURL(tag string) string
	// CommitURL returns the host's web link to a commit by hash, or "" when one
	// can't be formed.
	CommitURL(hash string) string
	// PRURL returns the host's web link to a pull/merge request by number, or ""
	// when one can't be formed.
	PRURL(number string) string
	// UserURL returns the host's web link to a user's profile by handle, or ""
	// when one can't be formed.
	UserURL(handle string) string
	// NewContributors returns the author handles making their first contribution
	// in (from, to].
	NewContributors(ctx context.Context, from, to string) ([]string, error)
}
