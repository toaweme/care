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

// commits reads the range (from, to], host-enriched when a host is configured
// and reachable, falling back to the git-log backend on any host failure. The
// returned commits are always parsed for their conventional fields.
func (e *Engine) commits(ctx context.Context, from, to string) ([]Commit, error) {
	if e.host != nil {
		if cs, err := e.host.CompareCommits(ctx, from, to); err == nil {
			for i := range cs {
				Parse(&cs[i])
			}
			return cs, nil
		}
	}
	return e.git.CommitsInRange(ctx, from, to)
}

// extras synthesizes the host-only notes additions, returning a zero Extras when
// no host is configured or its calls fail (degrade to the git-log path).
func (e *Engine) extras(ctx context.Context, from, to string) Extras {
	if e.host == nil {
		return Extras{}
	}
	extras := Extras{CompareURL: e.host.CompareURL(from, to)}
	if contributors, err := e.host.NewContributors(ctx, from, to); err == nil {
		extras.NewContributors = contributors
	}
	return extras
}
