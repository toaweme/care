package changelog

import (
	"context"
	"fmt"
	"strings"
)

// unreleasedSemver is the bracket label for the staging section that gathers
// commits made since the latest tag, matching the Keep a Changelog [Unreleased]
// convention. It carries no date and always sorts above the tagged releases.
const unreleasedSemver = "Unreleased"

// unreleasedRef is the range end for the [Unreleased] section: the working tree's
// HEAD, so every commit past the latest tag is staged.
const unreleasedRef = "HEAD"

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
	doc := ParseDocument(StripLinkRefs(existing))
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
	// the [Unreleased] block is regenerated from commits past the latest tag (below),
	// so drop the old one here; any other untagged block (a hand-written entry) is
	// kept verbatim at the top.
	tagged := taggedSemvers(tags)
	var orphans []string
	for _, pv := range doc.Versions {
		if tagged[pv.Semver] || strings.EqualFold(pv.Semver, unreleasedSemver) {
			continue
		}
		orphans = append(orphans, pv.Raw)
	}
	// from is the latest tag (tags are newest-first), or "" before the first release
	// so the Unreleased range spans all history.
	from := ""
	if len(tags) > 0 {
		from = tags[0]
	}
	unreleased, err := e.buildUnreleased(ctx, from, doc)
	if err != nil {
		return "", err
	}
	header := doc.Header
	if strings.TrimSpace(header) == "" {
		header = defaultHeader
	}
	var all []string
	if unreleased != "" {
		all = append(all, unreleased)
	}
	all = append(all, orphans...)
	all = append(all, blocks...)
	// only link [Unreleased] when a prior tag gives it a compare base.
	var staged *linkRef
	if unreleased != "" && from != "" {
		staged = &linkRef{semver: unreleasedSemver, from: from, to: unreleasedRef}
	}
	if refs := e.linkRefs(ctx, staged); refs != "" {
		all = append(all, refs)
	}
	return assemble(header, all), nil
}

// buildUnreleased renders the [Unreleased] section from the commits in
// (from, HEAD], or "" when there are none. from is the latest tag, or empty before
// the first release so the range spans all history. Prose under an existing
// [Unreleased] block is carried over; only its groups are regenerated. Author
// handles stay blank for unpushed commits (the host compare can't see them), but
// the commit links still resolve from the local hashes.
func (e *Engine) buildUnreleased(ctx context.Context, from string, doc Document) (string, error) {
	commits, err := e.commits(ctx, from, unreleasedRef)
	if err != nil {
		return "", err
	}
	if len(commits) == 0 {
		return "", nil
	}
	v := Version{Semver: unreleasedSemver, Sections: e.grouper.Group(commits)}
	if pv, ok := doc.Find(unreleasedSemver); ok {
		v.Prose = pv.Prose
	}
	return e.renderer.RenderVersion(v), nil
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
	doc := ParseDocument(StripLinkRefs(existing))
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
		// the staged version replaces its own prior block, and absorbs the
		// [Unreleased] commits, so drop both rather than keeping them verbatim.
		if pv.Semver == semver || strings.EqualFold(pv.Semver, unreleasedSemver) {
			continue
		}
		blocks = append(blocks, pv.Raw)
	}
	header := doc.Header
	if strings.TrimSpace(header) == "" {
		header = defaultHeader
	}
	staged := &linkRef{semver: semver, from: from, to: version}
	if refs := e.linkRefs(ctx, staged); refs != "" {
		blocks = append(blocks, refs)
	}
	return assemble(header, blocks), nil
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
