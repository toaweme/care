package shared

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/toaweme/care"
)

// initGitRepo creates a git repo in a temp dir with one committed file, no remote
// (so it never gets an upstream), mirroring a detached-HEAD CI checkout.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=care", "GIT_AUTHOR_EMAIL=care@example.com",
			"GIT_COMMITTER_NAME=care", "GIT_COMMITTER_EMAIL=care@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "care@example.com")
	run("config", "user.name", "care")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	run("add", "file.txt")
	run("commit", "-q", "-m", "init")
	return dir
}

func Test_VersionControl_NoUpstream_CleanTree_Warns(t *testing.T) {
	dir := initGitRepo(t)
	out := NewVersionControl().Run(context.Background(), dir, care.RunOptions{})
	if out.Status() != care.StatusWarn {
		t.Fatalf("expected StatusWarn for a clean tree with no upstream, got %v", out.Status())
	}
}

func Test_VersionControl_NoUpstream_DirtyTree_Fails(t *testing.T) {
	dir := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	out := NewVersionControl().Run(context.Background(), dir, care.RunOptions{})
	if out.Status() != care.StatusFail {
		t.Fatalf("expected StatusFail for a dirty tree, got %v", out.Status())
	}
}
