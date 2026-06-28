package changelog

import (
	"context"
	"fmt"
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

// BuildVersion derives one tag's section from git (host-enriched when available):
// resolves the previous tag in the tag's history, collects the range commits, and
// groups them. Used to build CHANGELOG.md sections.
func (e *Engine) BuildVersion(ctx context.Context, tag string) (Version, error) {
	from, err := e.git.PreviousTag(ctx, tag)
	if err != nil {
		return Version{}, fmt.Errorf("failed to resolve previous tag for %q: %w", tag, err)
	}
	commits, err := e.commits(ctx, from, tag)
	if err != nil {
		return Version{}, err
	}
	return Version{
		Tag:      tag,
		Semver:   Semver(tag),
		Date:     e.git.TagDate(ctx, tag),
		Sections: e.grouper.Group(commits),
	}, nil
}

// Update brings CHANGELOG.md up to date without destroying anything: it adds a
// section for every tag missing from the file, injects generated commit groups
// beneath any version that has hand-written prose but no groups yet, and leaves
// fully-formed sections (prose and all human edits) untouched. Versions not
// backed by a tag (an Unreleased block, a hand-written entry) stay at the top.
// Idempotent: a re-run reproduces its own output.
func (e *Engine) Update(ctx context.Context, existing string) (string, error) {
	doc := ParseDocument(existing)
	tags, err := e.git.Tags(ctx)
	if err != nil {
		return "", err
	}
	// tags come newest-first; render the whole list in that order.
	var blocks []string
	for _, tag := range tags {
		pv, present := doc.Find(Semver(tag))
		if present && pv.HasGroups {
			// fully formed: keep verbatim so human edits survive.
			blocks = append(blocks, pv.Raw)
			continue
		}
		v, err := e.BuildVersion(ctx, tag)
		if err != nil {
			return "", err
		}
		if present {
			// present but prose-only: add groups, preserve the prose and date.
			v.Prose = pv.Prose
			v.Date = pv.Date
		}
		blocks = append(blocks, e.renderer.RenderVersion(v))
	}
	// keep present versions with no matching tag (e.g. an Unreleased block) at the top.
	tagged := taggedSemvers(tags)
	var orphans []string
	for _, pv := range doc.Versions {
		if !tagged[pv.Semver] {
			orphans = append(orphans, pv.Raw)
		}
	}
	header := doc.Header
	if strings.TrimSpace(header) == "" {
		header = keepAChangelogHeader
	}
	return assemble(header, append(orphans, blocks...)), nil
}

// InsertVersion stages an as-yet-untagged release in CHANGELOG.md: it builds a
// section for the range (from, to] labeled version with date, then merges it into
// the existing document ahead of the tagged versions, replacing any block already
// carrying that version (its human prose is preserved, its groups regenerated).
// This is the pre-tag path, so the changelog can be committed and the tag placed
// on that same commit. Re-running before tagging refreshes the section; once the
// tag exists, a plain Update keeps it verbatim.
func (e *Engine) InsertVersion(ctx context.Context, from, to, version, date, existing string) (string, error) {
	// host-enriched so the staged section carries author handles, same as the
	// tagged sections. The compare API only sees pushed commits, so the branch
	// must be pushed before staging or the range will be incomplete.
	commits, err := e.commits(ctx, from, to)
	if err != nil {
		return "", err
	}
	semver := Semver(version)
	doc := ParseDocument(existing)
	v := Version{
		Tag:      version,
		Semver:   semver,
		Date:     date,
		Sections: e.grouper.Group(commits),
	}
	if pv, ok := doc.Find(semver); ok {
		v.Prose = pv.Prose
	}
	// the staged version is the newest release, so it leads; every other block
	// (the matching one replaced above) keeps its file order beneath it.
	blocks := []string{e.renderer.RenderVersion(v)}
	for _, pv := range doc.Versions {
		if pv.Semver == semver {
			continue
		}
		blocks = append(blocks, pv.Raw)
	}
	header := doc.Header
	if strings.TrimSpace(header) == "" {
		header = keepAChangelogHeader
	}
	return assemble(header, blocks), nil
}

// ExtractNotes produces the release notes for the range (from, to]. When
// existing holds a CHANGELOG.md with a curated `## [Semver(to)]` section, that
// section's body (prose plus groups) is used verbatim and only the host extras
// are appended, so hand-written notes reach the release. Otherwise the notes are
// derived from git and enriched with the host extras. from and to are any git
// refs the caller chose; from is "" to start from the first commit (a first
// release or an explicit full range). Callers pass existing only for the natural
// range; an explicit --since/--full must pass "" so the file never overrides it.
func (e *Engine) ExtractNotes(ctx context.Context, from, to, existing string) (string, error) {
	extras := e.extras(ctx, from, to)
	if existing != "" {
		if pv, ok := ParseDocument(existing).Find(Semver(to)); ok {
			return AppendExtras(pv.Body, extras), nil
		}
	}
	commits, err := e.commits(ctx, from, to)
	if err != nil {
		return "", err
	}
	v := Version{Semver: Semver(to), Sections: e.grouper.Group(commits)}
	return e.renderer.RenderNotes(v, extras), nil
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

// assemble joins the header and version blocks with blank-line separators and a
// single trailing newline.
func assemble(header string, blocks []string) string {
	parts := make([]string, 0, len(blocks)+1)
	if h := strings.TrimRight(header, "\n"); h != "" {
		parts = append(parts, h)
	}
	for _, block := range blocks {
		if b := strings.Trim(block, "\n"); b != "" {
			parts = append(parts, b)
		}
	}
	return strings.Join(parts, "\n\n") + "\n"
}

func taggedSemvers(tags []string) map[string]bool {
	set := make(map[string]bool, len(tags))
	for _, tag := range tags {
		set[Semver(tag)] = true
	}
	return set
}
