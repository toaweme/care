package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Test_Changelog_Write_FromTags covers the default --write path: the maintained
// file is rebuilt from the repo's tags.
func Test_Changelog_Write_FromTags(t *testing.T) {
	dir := newGitRepo(t)
	gitCommit(t, dir, "feat: first feature")
	gitTag(t, dir, "v0.1.0")

	cmd := NewChangelogCommand()
	cfg := ChangelogConfig{File: "./CHANGELOG.md"}
	cmd.Inputs = &cfg

	out, err := cmd.write(context.Background(), dir, cfg, detectEngine(context.Background(), dir, cfg))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## [0.1.0]") || !strings.Contains(out, "- First feature") {
		t.Errorf("write did not build the tagged section:\n%s", out)
	}
}

// Test_Changelog_Write_Release covers the --release path: an untagged version is
// staged from the range past the latest tag, dated today, ahead of tagging.
func Test_Changelog_Write_Release(t *testing.T) {
	dir := newGitRepo(t)
	gitCommit(t, dir, "feat: released feature")
	gitTag(t, dir, "v0.1.0")
	gitCommit(t, dir, "feat: upcoming feature")
	gitCommit(t, dir, "fix: upcoming fix")

	cmd := NewChangelogCommand()
	cfg := ChangelogConfig{File: "./CHANGELOG.md", Write: true, Release: "v0.2.0"}
	cmd.Inputs = &cfg

	out, err := cmd.write(context.Background(), dir, cfg, detectEngine(context.Background(), dir, cfg))
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().Format("2006-01-02")
	if !strings.Contains(out, "## [0.2.0] - "+today) {
		t.Errorf("staged section missing today's date %q:\n%s", today, out)
	}
	if !strings.Contains(out, "- Upcoming feature") || !strings.Contains(out, "- Upcoming fix") {
		t.Errorf("staged section missing untagged commits:\n%s", out)
	}
	// the released commit before v0.1.0 must not appear in the staged section.
	if strings.Contains(out, "- Released feature") {
		t.Errorf("staged section leaked the already-released commit:\n%s", out)
	}
}

func newGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "config", "user.email", "test@care.test")
	gitRun(t, dir, "config", "user.name", "Care Test")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	gitRun(t, dir, "config", "tag.gpgSign", "false")
	return dir
}

func gitCommit(t *testing.T, dir, subject string) {
	t.Helper()
	gitRun(t, dir, "commit", "--allow-empty", "-q", "-m", subject)
}

func gitTag(t *testing.T, dir, name string) {
	t.Helper()
	gitRun(t, dir, "tag", name)
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2026-06-01T00:00:00",
		"GIT_COMMITTER_DATE=2026-06-01T00:00:00",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
}
