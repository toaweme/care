package changelog

import (
	"context"
	"strings"
	"testing"
)

func Test_Engine_ExtractNotes_ExplicitRange(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: one")
	tag(t, dir, "v1.0.0")
	commit(t, dir, "fix: two")
	commit(t, dir, "feat: three")
	tag(t, dir, "v1.1.0")

	engine := NewEngine(NewGit(dir), nil, DefaultGroups, false)
	// the caller owns the range: notes for exactly v1.0.0..v1.1.0.
	notes, err := engine.ExtractNotes(context.Background(), "v1.0.0", "v1.1.0", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(notes, "- Two") || !strings.Contains(notes, "- Three") {
		t.Errorf("range notes missing in-range commits:\n%s", notes)
	}
	if strings.Contains(notes, "- One") {
		t.Errorf("range notes leaked the pre-range commit:\n%s", notes)
	}
}

func Test_Engine_ExtractNotes_FromFileSlice(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: one")
	tag(t, dir, "v1.0.0")
	commit(t, dir, "fix: derived from git")
	tag(t, dir, "v1.1.0")

	git := NewGit(dir)
	host := &fakeHost{git: git, contributors: []string{"alice"}}
	engine := NewEngine(git, host, DefaultGroups, false)

	// the file carries a curated 1.1.0 section; its body wins over git derivation.
	existing := "# Changelog\n\n## [1.1.0] - 2026-06-27\n\nHand-written highlight for 1.1.0.\n\n### Fixes\n\n- curated fix line\n"
	notes, err := engine.ExtractNotes(context.Background(), "v1.0.0", "v1.1.0", existing)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(notes, "Hand-written highlight for 1.1.0.") || !strings.Contains(notes, "- curated fix line") {
		t.Errorf("notes did not use the curated file body:\n%s", notes)
	}
	if strings.Contains(notes, "Derived from git") {
		t.Errorf("notes leaked git-derived commits over the curated body:\n%s", notes)
	}
	// the curated heading itself is dropped (it is the notes body, not a section).
	if strings.Contains(notes, "## [1.1.0]") {
		t.Errorf("notes included the version heading:\n%s", notes)
	}
	// host extras still attach beneath the curated body.
	if !strings.Contains(notes, "## New Contributors") || !strings.Contains(notes, "@alice made their first contribution") {
		t.Errorf("notes missing host extras under curated body:\n%s", notes)
	}

	// no matching section -> fall back to git derivation.
	mismatch := "# Changelog\n\n## [2.0.0]\n\nUnrelated.\n"
	notes2, err := engine.ExtractNotes(context.Background(), "v1.0.0", "v1.1.0", mismatch)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(notes2, "- Derived from git") {
		t.Errorf("notes did not fall back to git when no section matched:\n%s", notes2)
	}
}

func Test_Engine_ExtractNotes_GitHost(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: one")
	tag(t, dir, "v0.1.0")
	commit(t, dir, "feat: two")
	tag(t, dir, "v0.2.0")

	git := NewGit(dir)
	host := &fakeHost{git: git, handles: map[string]string{"feat: two": "alice"}, contributors: []string{"alice"}}
	engine := NewEngine(git, host, DefaultGroups, false)

	notes, err := engine.ExtractNotes(context.Background(), "v0.1.0", "v0.2.0", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(notes, "- Two ") || !strings.Contains(notes, "by [@alice](") {
		t.Errorf("notes missing enriched handle:\n%s", notes)
	}
	if !strings.Contains(notes, "(https://example.test/commit/") {
		t.Errorf("notes missing commit link:\n%s", notes)
	}
	if !strings.Contains(notes, "## New Contributors") || !strings.Contains(notes, "@alice made their first contribution") {
		t.Errorf("notes missing new contributors:\n%s", notes)
	}
	if !strings.Contains(notes, "**Full Changelog**: https://example.test/compare/v0.1.0...v0.2.0") {
		t.Errorf("notes missing compare link:\n%s", notes)
	}
}

func Test_Engine_ExtractNotes_IncludesUnpushedBranchCommits(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: one")
	tag(t, dir, "v0.4.0")
	// only this commit is pushed to the host; the rest live on a local branch the
	// host's compare API can't see.
	commit(t, dir, "ci: shared with main")
	commit(t, dir, "feat: local only feature")
	commit(t, dir, "fix: local only fix")

	git := NewGit(dir)
	host := &fakeHost{
		git:      git,
		handles:  map[string]string{"ci: shared with main": "alice"},
		unpushed: map[string]bool{"feat: local only feature": true, "fix: local only fix": true},
	}
	engine := NewEngine(git, host, DefaultGroups, false)

	// the range end is the local HEAD, which the host can't resolve; the local git
	// walk is the source of truth, so every branch commit must appear.
	notes, err := engine.ExtractNotes(context.Background(), "v0.4.0", "HEAD", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(notes, "- Local only feature") || !strings.Contains(notes, "- Local only fix") {
		t.Errorf("notes dropped un-pushed branch commits (the host-shadowing bug):\n%s", notes)
	}
	if !strings.Contains(notes, "- Shared with main") {
		t.Errorf("notes dropped the shared commit:\n%s", notes)
	}
	// the shared, pushed commit still gets its host handle; the un-pushed ones don't.
	if !strings.Contains(notes, "by [@alice](") {
		t.Errorf("notes lost host handle enrichment for the pushed commit:\n%s", notes)
	}
}

func Test_Engine_ExtractNotes_EnrichesPushedBranchWithoutMerge(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: one")
	tag(t, dir, "v0.4.0")
	run(t, dir, "git", "branch", "-M", "main")
	commit(t, dir, "ci: shared with main")
	run(t, dir, "git", "switch", "-c", "feat/pre-release-cleanup", "-q")
	commit(t, dir, "feat: branch feature")
	commit(t, dir, "fix: branch fix")

	git := NewGit(dir)
	// the host resolves HEAD to main, so enrichment must name the branch instead.
	host := &fakeHost{
		git:           git,
		defaultBranch: "main",
		handles: map[string]string{
			"ci: shared with main": "alice",
			"feat: branch feature": "bob",
			"fix: branch fix":      "bob",
		},
	}
	engine := NewEngine(git, host, DefaultGroups, false)

	notes, err := engine.ExtractNotes(context.Background(), "v0.4.0", "HEAD", "")
	if err != nil {
		t.Fatal(err)
	}
	// the branch is pushed but not merged: its commits still get their handles.
	if !strings.Contains(notes, "- Branch feature by [@bob](") {
		t.Errorf("branch commit lost its handle (HEAD resolved to the host default branch):\n%s", notes)
	}
	if !strings.Contains(notes, "- Branch fix by [@bob](") {
		t.Errorf("branch commit lost its handle (HEAD resolved to the host default branch):\n%s", notes)
	}
	if !strings.Contains(notes, "- Shared with main by [@alice](") {
		t.Errorf("shared commit lost its handle:\n%s", notes)
	}
}

func Test_Engine_ExtractNotes_CompareLinkNamesBranch(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: one")
	tag(t, dir, "v0.4.0")
	run(t, dir, "git", "branch", "-M", "main")
	run(t, dir, "git", "switch", "-c", "feat/pre-release-cleanup", "-q")
	commit(t, dir, "feat: branch feature")

	git := NewGit(dir)
	host := &fakeHost{git: git, defaultBranch: "main"}
	engine := NewEngine(git, host, DefaultGroups, false)

	notes, err := engine.ExtractNotes(context.Background(), "v0.4.0", "HEAD", "")
	if err != nil {
		t.Fatal(err)
	}
	// a bare HEAD would read as the host's default branch, describing the wrong range.
	if !strings.Contains(notes, "**Full Changelog**: https://example.test/compare/v0.4.0...feat/pre-release-cleanup") {
		t.Errorf("compare link did not name the branch:\n%s", notes)
	}
	if strings.Contains(notes, "compare/v0.4.0...HEAD") {
		t.Errorf("compare link still points at the unresolvable HEAD:\n%s", notes)
	}
}

func Test_Engine_ExtractNotes_DegradesWhenGitHostFails(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: one")
	tag(t, dir, "v0.1.0")
	commit(t, dir, "fix: two")
	tag(t, dir, "v0.2.0")

	git := NewGit(dir)
	host := &fakeHost{git: git, fail: true}
	engine := NewEngine(git, host, DefaultGroups, false)

	notes, err := engine.ExtractNotes(context.Background(), "v0.1.0", "v0.2.0", "")
	if err != nil {
		t.Fatal(err)
	}
	// commit enrichment falls back to git-log; the section is still produced.
	if !strings.Contains(notes, "- Two") {
		t.Errorf("degraded notes missing commit:\n%s", notes)
	}
}
