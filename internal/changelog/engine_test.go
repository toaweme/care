package changelog

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeHost is a deterministic in-memory GitHost for exercising the enrich/extras
// paths without a network. handles maps a commit subject to an author handle.
type fakeHost struct {
	git          *Git
	handles      map[string]string
	contributors []string
	fail         bool
}

func (f *fakeHost) CompareCommits(ctx context.Context, from, to string) ([]Commit, error) {
	if f.fail {
		return nil, context.DeadlineExceeded
	}
	commits, err := f.git.CommitsInRange(ctx, from, to)
	if err != nil {
		return nil, err
	}
	for i := range commits {
		commits[i].Handle = f.handles[commits[i].Subject]
	}
	return commits, nil
}

func (f *fakeHost) CompareURL(from, to string) string {
	if from == "" {
		return ""
	}
	return "https://example.test/compare/" + from + "..." + to
}

func (f *fakeHost) TagURL(tag string) string {
	if tag == "" {
		return ""
	}
	return "https://example.test/releases/tag/" + tag
}

func (f *fakeHost) CommitURL(hash string) string {
	if hash == "" {
		return ""
	}
	return "https://example.test/commit/" + hash
}

func (f *fakeHost) PRURL(number string) string {
	if number == "" {
		return ""
	}
	return "https://example.test/pull/" + number
}

func (f *fakeHost) UserURL(handle string) string {
	if handle == "" {
		return ""
	}
	return "https://example.test/" + handle
}

func (f *fakeHost) NewContributors(ctx context.Context, from, to string) ([]string, error) {
	return f.contributors, nil
}

func Test_Engine_Update_FromScratch(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: initial feature")
	tag(t, dir, "v0.1.0")
	commit(t, dir, "fix: a bug")
	commit(t, dir, "docs: update readme")
	tag(t, dir, "v0.2.0")

	git := NewGit(dir)
	engine := NewEngine(git, nil, DefaultGroups, false)
	// an empty existing file builds the whole changelog from every tag.
	out, err := engine.Update(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	// newest first: v0.2.0 before v0.1.0.
	i02 := strings.Index(out, "## [0.2.0]")
	i01 := strings.Index(out, "## [0.1.0]")
	if i02 < 0 || i01 < 0 || i02 > i01 {
		t.Fatalf("version order wrong:\n%s", out)
	}
	if !strings.Contains(out, "### Fixes") || !strings.Contains(out, "- A bug") {
		t.Errorf("missing fixes section:\n%s", out)
	}
	if !strings.Contains(out, "### Features") || !strings.Contains(out, "- Initial feature") {
		t.Errorf("missing features:\n%s", out)
	}
}

func Test_Engine_Update_GeneratesUnreleased(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: shipped")
	tag(t, dir, "v0.1.0")
	commit(t, dir, "feat: pending feature")
	commit(t, dir, "fix: pending fix")

	git := NewGit(dir)
	host := &fakeHost{git: git}
	engine := NewEngine(git, host, DefaultGroups, false)

	out, err := engine.Update(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	// the staging section leads, undated, above the tagged release.
	if !strings.Contains(out, "## [Unreleased]") {
		t.Fatalf("missing unreleased section:\n%s", out)
	}
	if iu, i01 := strings.Index(out, "## [Unreleased]"), strings.Index(out, "## [0.1.0]"); iu < 0 || i01 < 0 || iu > i01 {
		t.Errorf("unreleased should lead, before 0.1.0:\n%s", out)
	}
	// the commits past the latest tag land in it, and not the released one.
	if !strings.Contains(out, "- Pending feature") || !strings.Contains(out, "- Pending fix") {
		t.Errorf("unreleased missing pending commits:\n%s", out)
	}
	if strings.Count(out, "Shipped") != 1 {
		t.Errorf("released commit leaked into unreleased:\n%s", out)
	}
	// it links to the compare range from the latest tag to HEAD.
	if !strings.Contains(out, "[Unreleased]: https://example.test/compare/v0.1.0...HEAD") {
		t.Errorf("missing unreleased compare reference:\n%s", out)
	}
	// idempotent and prose-preserving: a hand-written highlight survives a re-run.
	withProse := strings.Replace(out, "## [Unreleased]\n\n", "## [Unreleased]\n\nHand-written highlight.\n\n", 1)
	again, err := engine.Update(context.Background(), withProse)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(again, "Hand-written highlight.") {
		t.Errorf("re-run lost unreleased prose:\n%s", again)
	}
	if strings.Count(again, "## [Unreleased]") != 1 {
		t.Errorf("unreleased block duplicated:\n%s", again)
	}
}

func Test_Engine_Update_NoUnreleasedWhenAtTag(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: shipped")
	tag(t, dir, "v0.1.0")

	engine := NewEngine(NewGit(dir), nil, DefaultGroups, false)
	out, err := engine.Update(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	// HEAD is the latest tag, so there is nothing to stage.
	if strings.Contains(out, "## [Unreleased]") {
		t.Errorf("unexpected unreleased section with no commits past the tag:\n%s", out)
	}
}

func Test_Engine_InsertVersion_PromotesUnreleased(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: shipped")
	tag(t, dir, "v0.1.0")
	commit(t, dir, "feat: pending feature")

	git := NewGit(dir)
	engine := NewEngine(git, nil, DefaultGroups, false)

	// a changelog already carrying an [Unreleased] staging block.
	staged, err := engine.Update(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(staged, "## [Unreleased]") {
		t.Fatalf("setup missing unreleased block:\n%s", staged)
	}

	from, err := git.PreviousTag(context.Background(), "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	out, err := engine.InsertVersion(context.Background(), from, "HEAD", "v0.2.0", "2026-06-28", staged)
	if err != nil {
		t.Fatal(err)
	}
	// promoting to a tagged version replaces the staging block, not duplicates it.
	if strings.Contains(out, "## [Unreleased]") {
		t.Errorf("unreleased block survived promotion:\n%s", out)
	}
	if !strings.Contains(out, "## [0.2.0] - 2026-06-28") || !strings.Contains(out, "- Pending feature") {
		t.Errorf("promotion missing staged version with its commits:\n%s", out)
	}
}

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

func Test_Engine_Update_AddsMissingAndPreservesEdits(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: old feature")
	tag(t, dir, "v0.1.0")
	commit(t, dir, "feat: new feature")
	tag(t, dir, "v0.2.0")

	existing := "# Changelog\n\n## [0.1.0] - 2026-01-01\n\nHuman authored note for 0.1.0.\n\n### Features\n\n- old feature, hand edited (@someone)\n"

	engine := NewEngine(NewGit(dir), nil, DefaultGroups, false)
	out, err := engine.Update(context.Background(), existing)
	if err != nil {
		t.Fatal(err)
	}
	// the missing 0.2.0 tag was added, newest-first.
	if !strings.Contains(out, "## [0.2.0]") || !strings.Contains(out, "- New feature") {
		t.Errorf("update did not add 0.2.0:\n%s", out)
	}
	if strings.Index(out, "## [0.2.0]") > strings.Index(out, "## [0.1.0]") {
		t.Errorf("update order wrong:\n%s", out)
	}
	// the fully-formed, hand-edited 0.1.0 section survived verbatim.
	if !strings.Contains(out, "Human authored note for 0.1.0.") || !strings.Contains(out, "old feature, hand edited") {
		t.Errorf("update clobbered human edits:\n%s", out)
	}
}

func Test_Engine_Update_AppendsLinkReferences(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: old feature")
	tag(t, dir, "v0.1.0")
	commit(t, dir, "feat: new feature")
	tag(t, dir, "v0.2.0")

	git := NewGit(dir)
	host := &fakeHost{git: git}
	engine := NewEngine(git, host, DefaultGroups, false)

	out, err := engine.Update(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	// the newest version links to its compare range.
	if !strings.Contains(out, "\n[0.2.0]: https://example.test/compare/v0.1.0...v0.2.0\n") {
		t.Errorf("missing compare reference for 0.2.0:\n%s", out)
	}
	// the first release has no range, so it links to its tag page, and the
	// reference footer is the tail of the file.
	if !strings.HasSuffix(strings.TrimRight(out, "\n"), "[0.1.0]: https://example.test/releases/tag/v0.1.0") {
		t.Errorf("first release should link to its tag page, last in file:\n%s", out)
	}
	// re-running regenerates the footer rather than stacking a second copy.
	again, err := engine.Update(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if again != out {
		t.Errorf("update not idempotent with link refs:\nfirst:\n%s\nsecond:\n%s", out, again)
	}
	if n := strings.Count(again, "[0.2.0]: "); n != 1 {
		t.Errorf("expected one 0.2.0 reference, got %d:\n%s", n, again)
	}
}

func Test_Engine_Update_NoHostOmitsLinkReferences(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: only feature")
	tag(t, dir, "v1.0.0")

	engine := NewEngine(NewGit(dir), nil, DefaultGroups, false)
	out, err := engine.Update(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	// without a host no web links can be formed, so the footer is omitted.
	if strings.Contains(out, "[1.0.0]: ") {
		t.Errorf("git-log path should not emit reference links:\n%s", out)
	}
}

func Test_Engine_Update_FillsProseOnlySections(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: the feature")
	commit(t, dir, "fix: the fix")
	tag(t, dir, "v1.0.0")

	existing := "# Changelog\n\n## [1.0.0] - 2026-06-01\n\nHand-written summary, groups to be filled.\n"

	engine := NewEngine(NewGit(dir), nil, DefaultGroups, false)
	out, err := engine.Update(context.Background(), existing)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Hand-written summary, groups to be filled.") {
		t.Errorf("update lost prose:\n%s", out)
	}
	if !strings.Contains(out, "### Features") || !strings.Contains(out, "### Fixes") {
		t.Errorf("update did not inject groups under prose:\n%s", out)
	}

	// idempotent: a second run (now that groups exist) changes nothing.
	out2, err := engine.Update(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if out2 != out {
		t.Errorf("update not idempotent:\nfirst:\n%s\nsecond:\n%s", out, out2)
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

func Test_Engine_InsertVersion_StagesUntaggedRange(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: released")
	tag(t, dir, "v0.1.0")
	commit(t, dir, "feat: staged feature")
	commit(t, dir, "fix: staged fix")

	git := NewGit(dir)
	engine := NewEngine(git, nil, DefaultGroups, false)
	existing := "# Changelog\n\n## [0.1.0] - 2026-06-01\n\n### Features\n\n- Released ([abc1234](x)) (Care Test)\n"

	from, err := git.PreviousTag(context.Background(), "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	out, err := engine.InsertVersion(context.Background(), from, "HEAD", "v0.2.0", "2026-06-27", existing)
	if err != nil {
		t.Fatal(err)
	}
	// the staged 0.2.0 section leads, dated, built from the untagged range.
	if !strings.Contains(out, "## [0.2.0] - 2026-06-27") {
		t.Errorf("missing staged version heading:\n%s", out)
	}
	if !strings.Contains(out, "- Staged feature") || !strings.Contains(out, "- Staged fix") {
		t.Errorf("staged section missing range commits:\n%s", out)
	}
	// the existing tagged section is preserved beneath it, newest-first.
	if i02, i01 := strings.Index(out, "## [0.2.0]"), strings.Index(out, "## [0.1.0]"); i02 < 0 || i01 < 0 || i02 > i01 {
		t.Errorf("version order wrong:\n%s", out)
	}
	// the pre-range, already-released commit must not leak into the staged section.
	if strings.Contains(out, "- Released ([") && strings.Count(out, "Released") != 1 {
		t.Errorf("released commit leaked into staged section:\n%s", out)
	}
}

func Test_Engine_InsertVersion_RefreshesAndKeepsProse(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: base")
	tag(t, dir, "v0.1.0")
	commit(t, dir, "feat: first staged")

	git := NewGit(dir)
	engine := NewEngine(git, nil, DefaultGroups, false)
	from, err := git.PreviousTag(context.Background(), "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	// first run stages the section; a human then adds prose.
	out1, err := engine.InsertVersion(context.Background(), from, "HEAD", "v0.2.0", "2026-06-27", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out1, "- First staged") {
		t.Fatalf("first stage missing commit:\n%s", out1)
	}
	withProse := strings.Replace(out1, "## [0.2.0] - 2026-06-27\n\n", "## [0.2.0] - 2026-06-27\n\nHand-written highlight.\n\n", 1)

	// a new commit lands; re-running refreshes the groups but preserves the prose
	// and does not duplicate the version block.
	commit(t, dir, "fix: second staged")
	out2, err := engine.InsertVersion(context.Background(), from, "HEAD", "v0.2.0", "2026-06-28", withProse)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out2, "## [0.2.0]") != 1 {
		t.Errorf("staged version block duplicated:\n%s", out2)
	}
	if !strings.Contains(out2, "Hand-written highlight.") {
		t.Errorf("re-run lost human prose:\n%s", out2)
	}
	if !strings.Contains(out2, "- First staged") || !strings.Contains(out2, "- Second staged") {
		t.Errorf("re-run did not refresh groups with the new commit:\n%s", out2)
	}
	if !strings.Contains(out2, "## [0.2.0] - 2026-06-28") {
		t.Errorf("re-run did not update the date:\n%s", out2)
	}
}

func Test_Engine_InsertVersion_ThenTaggedUpdateKeepsItVerbatim(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: base")
	tag(t, dir, "v0.1.0")
	commit(t, dir, "feat: staged feature")

	git := NewGit(dir)
	engine := NewEngine(git, nil, DefaultGroups, false)

	// the released baseline already carries 0.1.0.
	baseline, err := engine.Update(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	from, err := git.PreviousTag(context.Background(), "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	staged, err := engine.InsertVersion(context.Background(), from, "HEAD", "v0.2.0", "2026-06-27", baseline)
	if err != nil {
		t.Fatal(err)
	}

	// the staged section is committed and the tag placed on that commit; a plain
	// Update must keep both blocks verbatim rather than re-deriving them.
	tag(t, dir, "v0.2.0")
	out, err := engine.Update(context.Background(), staged)
	if err != nil {
		t.Fatal(err)
	}
	if out != staged {
		t.Errorf("tagged Update changed the staged section:\nstaged:\n%s\ngot:\n%s", staged, out)
	}
}

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init", "-q")
	run(t, dir, "git", "config", "user.email", "test@care.test")
	run(t, dir, "git", "config", "user.name", "Care Test")
	run(t, dir, "git", "config", "commit.gpgsign", "false")
	run(t, dir, "git", "config", "tag.gpgSign", "false")
	run(t, dir, "git", "config", "tag.forceSignAnnotated", "false")
	return dir
}

func commit(t *testing.T, dir, subject string) {
	t.Helper()
	run(t, dir, "git", "commit", "--allow-empty", "-q", "-m", subject)
}

func tag(t *testing.T, dir, name string) {
	t.Helper()
	run(t, dir, "git", "tag", name)
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2026-06-01T00:00:00",
		"GIT_COMMITTER_DATE=2026-06-01T00:00:00",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, filepath.Base(strings.Join(args, " ")), err, out)
	}
}
