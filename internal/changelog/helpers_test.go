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
// paths without a network. handles maps a commit subject to an author handle;
// unpushed lists subjects the host can't see (commits not yet pushed to it), which
// the compare API drops just as GitHub does for un-pushed branch commits.
// defaultBranch, when set, makes the host resolve HEAD to that branch the way a
// real host does, since HEAD names the server's own default branch and never the
// caller's checkout.
// canned, when set, is returned verbatim instead of walking git, standing in for
// a host that can serve a range the local checkout cannot (a shallow clone).
// contribTo records the range end NewContributors was asked for, so a test can
// assert which ref reached the host.
type fakeHost struct {
	git           *Git
	handles       map[string]string
	contributors  []string
	unpushed      map[string]bool
	defaultBranch string
	canned        []Commit
	contribTo     string
	fail          bool
}

func (f *fakeHost) CompareCommits(ctx context.Context, from, to string) ([]Commit, error) {
	if f.fail {
		return nil, context.DeadlineExceeded
	}
	if to == "HEAD" && f.defaultBranch != "" {
		to = f.defaultBranch
	}
	if f.canned != nil {
		return f.canned, nil
	}
	commits, err := f.git.CommitsInRange(ctx, from, to)
	if err != nil {
		return nil, err
	}
	visible := commits[:0]
	for _, c := range commits {
		if f.unpushed[c.Subject] {
			continue
		}
		c.Handle = f.handles[c.Subject]
		visible = append(visible, c)
	}
	return visible, nil
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
	f.contribTo = to
	return f.contributors, nil
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

// revParse resolves a ref to its full hash, for tests needing a concrete commit
// to detach onto.
func revParse(t *testing.T, dir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse %s failed: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
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
