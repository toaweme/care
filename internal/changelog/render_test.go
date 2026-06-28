package changelog

import (
	"strings"
	"testing"
)

// parsed builds a commit with its conventional fields filled, as the engine does
// before rendering.
func parsed(hash, subject, author, handle string) Commit {
	c := Commit{Hash: hash, Subject: subject, Author: author, Handle: handle}
	Parse(&c)
	return c
}

func Test_Renderer_RenderSections(t *testing.T) {
	sections := []Section{
		{Title: "Features", Commits: []Commit{
			parsed("abc1234def", "feat: add the thing (#42)", "", "alice"),
			parsed("deadbeef0", "feat!: drop v1", "", ""),
		}},
		{Title: "Fixes", Commits: []Commit{
			parsed("0123456789", "fix(api): broken call", "Bob", ""),
		}},
	}
	got := NewRenderer(&fakeHost{}, false).RenderSections(sections)
	want := "### Features\n\n" +
		"- Add the thing by [@alice](https://example.test/alice) in [#42](https://example.test/pull/42).\n" +
		"- **[breaking]** Drop v1 in [deadbee](https://example.test/commit/deadbeef0).\n" +
		"\n### Fixes\n\n" +
		"- **Api:** Broken call by Bob in [0123456](https://example.test/commit/0123456789).\n"
	if got != want {
		t.Errorf("RenderSections mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func Test_Renderer_Plain(t *testing.T) {
	sections := []Section{{Title: "Features", Commits: []Commit{
		parsed("abc1234def", "feat: add the thing (#42)", "", "alice"),
		parsed("0123456789", "fix(api): broken call", "Bob", ""),
	}}}
	got := NewRenderer(&fakeHost{}, true).RenderSections(sections)
	want := "### Features\n\n- Add the thing\n- **Api:** Broken call\n"
	if got != want {
		t.Errorf("plain RenderSections mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func Test_Renderer_NoHost(t *testing.T) {
	sections := []Section{{Title: "Features", Commits: []Commit{
		parsed("abc1234def", "feat: add the thing (#42)", "", "alice"),
	}}}
	got := NewRenderer(nil, false).RenderSections(sections)
	// no host: the PR number stays as bare text, no commit link, author still shown.
	want := "### Features\n\n- Add the thing by @alice in #42.\n"
	if got != want {
		t.Errorf("no-host RenderSections mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func Test_Renderer_RenderVersion(t *testing.T) {
	v := Version{
		Semver:   "1.2.3",
		Date:     "2026-06-27",
		Prose:    "Human intro.",
		Sections: []Section{{Title: "Features", Commits: []Commit{parsed("", "feat: a", "", "")}}},
	}
	got := NewRenderer(nil, false).RenderVersion(v)
	if !strings.HasPrefix(got, "## [1.2.3] - 2026-06-27\n\nHuman intro.\n\n### Features\n\n- A.\n") {
		t.Errorf("RenderVersion unexpected:\n%s", got)
	}
}

func Test_AppendExtras(t *testing.T) {
	body := "### Features\n\n- A\n"
	extras := Extras{
		CompareURL:      "https://github.com/o/r/compare/v1...v2",
		NewContributors: []string{"alice", "@bob"},
	}
	got := AppendExtras(body, extras)
	if !strings.Contains(got, "## New Contributors") {
		t.Errorf("missing New Contributors:\n%s", got)
	}
	if !strings.Contains(got, "- @alice made their first contribution") {
		t.Errorf("missing alice line:\n%s", got)
	}
	if !strings.Contains(got, "- @bob made their first contribution") {
		t.Errorf("@bob should be normalized:\n%s", got)
	}
	if !strings.HasSuffix(strings.TrimRight(got, "\n"), "**Full Changelog**: https://github.com/o/r/compare/v1...v2") {
		t.Errorf("compare link should be last:\n%s", got)
	}
}

func Test_AppendExtras_Empty(t *testing.T) {
	body := "### Features\n\n- A\n"
	if got := AppendExtras(body, Extras{}); got != body {
		t.Errorf("empty extras should leave body unchanged:\n%q", got)
	}
}
