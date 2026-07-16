package changelog

import (
	"context"
	"strings"
)

// Engine derives versions and release notes from a git backend, optionally
// enriched by a GitHost, and maintains a CHANGELOG.md. The git backend is the
// host-neutral default; the GitHost enriches when present and degrades to git on
// any failure, so every operation works with zero host involvement.
type Engine struct {
	git      *Git
	host     GitHost
	grouper  *Grouper
	renderer *Renderer
}

// NewEngine builds an engine from a git backend, an optional host (nil for the
// pure git-log path), and the section groups (pass DefaultGroups for the org set).
// plain renders a link-free, attribution-free body for non-git publishing.
func NewEngine(git *Git, host GitHost, groups []Group, plain bool) *Engine {
	return &Engine{git: git, host: host, grouper: NewGrouper(groups), renderer: NewRenderer(host, plain)}
}

// Semver strips a tag's leading v for the version heading: v1.2.3 -> 1.2.3.
func Semver(tag string) string {
	return strings.TrimPrefix(tag, "v")
}

// commits reads the range (from, to] from the local git backend, which is the
// source of truth for what is in range: it sees the working checkout, including
// branch commits not yet pushed to the host. When a host is configured it only
// enriches those commits with author handles, a best-effort pass that never
// changes which commits are returned. If the local walk fails (a shallow CI
// checkout missing the range's history), it falls back to the host's own
// enumeration, which can resolve the range server-side. The returned commits are
// always parsed for their conventional fields.
func (e *Engine) commits(ctx context.Context, from, to string) ([]Commit, error) {
	local, err := e.git.CommitsInRange(ctx, from, to)
	if err != nil {
		// the local history can't cover the range (typically a shallow clone), so
		// let the host enumerate it instead of failing outright.
		if e.host != nil {
			if cs, hostErr := e.host.CompareCommits(ctx, from, e.hostRef(ctx, to)); hostErr == nil {
				for i := range cs {
					Parse(&cs[i])
				}
				return cs, nil
			}
		}
		return nil, err
	}
	if e.host != nil {
		e.enrichHandles(ctx, local, from, to)
	}
	return local, nil
}

// hostRef maps a local ref to one the host can resolve. HEAD is meaningful only
// locally: a host resolves it to its own default branch, so a feature-branch
// checkout would be compared against main instead of itself. Naming the current
// branch asks the host for the range the caller actually meant, which resolves
// once the branch is pushed. A detached HEAD has no branch to name and degrades
// to the ref as given.
func (e *Engine) hostRef(ctx context.Context, ref string) string {
	if ref != "HEAD" {
		return ref
	}
	branch, err := e.git.BranchName(ctx, ref)
	if err != nil || branch == "" {
		return ref
	}
	return branch
}

// enrichHandles fills in host author handles on the local commits, matched by
// commit hash. It is best-effort: the host compare only sees commits pushed to
// it, so un-pushed commits keep their git author name, and any host error leaves
// every handle blank. It never adds, drops, or reorders commits.
func (e *Engine) enrichHandles(ctx context.Context, commits []Commit, from, to string) {
	hosted, err := e.host.CompareCommits(ctx, from, e.hostRef(ctx, to))
	if err != nil {
		return
	}
	handles := make(map[string]string, len(hosted))
	for _, c := range hosted {
		if c.Handle != "" {
			handles[c.Hash] = c.Handle
		}
	}
	for i := range commits {
		if h, ok := handles[commits[i].Hash]; ok {
			commits[i].Handle = h
		}
	}
}

// extras synthesizes the host-only notes additions, returning a zero Extras when
// no host is configured or its calls fail (degrade to the git-log path). The range
// end is named for the host, so a branch checkout's compare link and contributor
// list describe that branch rather than the host's default branch.
func (e *Engine) extras(ctx context.Context, from, to string) Extras {
	if e.host == nil {
		return Extras{}
	}
	ref := e.hostRef(ctx, to)
	extras := Extras{CompareURL: e.host.CompareURL(from, ref)}
	if contributors, err := e.host.NewContributors(ctx, from, ref); err == nil {
		extras.NewContributors = contributors
	}
	return extras
}
