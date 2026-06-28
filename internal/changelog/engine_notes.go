package changelog

import "context"

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
		if pv, ok := ParseDocument(StripLinkRefs(existing)).Find(Semver(to)); ok {
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
