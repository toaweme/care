package changelog

import (
	"context"
	"fmt"
	"strings"
)

// linkRef is one Keep a Changelog reference-link target: the bracket label (the semver) and the
// range it resolves to. from is empty for a first release, which links to its tag page rather than
// a compare range.
type linkRef struct {
	semver string
	from   string
	to     string
}

// linkRefs builds the Keep a Changelog reference-link footer, the block of `[semver]: url` lines
// that makes each `## [semver]` heading a clickable link. Every tagged version maps to its
// compare/{prev}...{tag} URL, except the first release, which has no predecessor and maps to its
// tag page. A non-nil staged version (the not-yet-tagged --release path) is listed first.
//
// It returns the empty string when there is no host, since the git-log path can form no web links,
// or when nothing resolves to a URL.
func (e *Engine) linkRefs(ctx context.Context, staged *linkRef) string {
	if e.host == nil {
		return ""
	}
	var b strings.Builder
	if staged != nil {
		if url := e.versionURL(staged.from, staged.to); url != "" {
			fmt.Fprintf(&b, "[%s]: %s\n", staged.semver, url)
		}
	}
	tags, err := e.git.Tags(ctx)
	if err != nil {
		return strings.TrimRight(b.String(), "\n")
	}
	for _, tag := range tags {
		from, err := e.git.PreviousTag(ctx, tag)
		if err != nil {
			continue
		}
		if url := e.versionURL(from, tag); url != "" {
			fmt.Fprintf(&b, "[%s]: %s\n", Semver(tag), url)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// versionURL is the reference target for one version: its compare link when a prior tag exists,
// otherwise the tag's release page for a first release.
func (e *Engine) versionURL(from, to string) string {
	if from == "" {
		return e.host.TagURL(to)
	}
	return e.host.CompareURL(from, to)
}
