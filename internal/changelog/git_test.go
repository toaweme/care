package changelog

import (
	"context"
	"testing"
)

func Test_Semver(t *testing.T) {
	if got := Semver("v1.2.3"); got != "1.2.3" {
		t.Errorf("Semver = %q, want 1.2.3", got)
	}
	if got := Semver("0.1.0"); got != "0.1.0" {
		t.Errorf("Semver = %q, want 0.1.0", got)
	}
}

func Test_Git_PreviousTag(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: one")
	tag(t, dir, "v0.1.0")
	commit(t, dir, "fix: two")
	tag(t, dir, "v0.2.0")

	git := NewGit(dir)
	prev, err := git.PreviousTag(context.Background(), "v0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if prev != "v0.1.0" {
		t.Errorf("PreviousTag(v0.2.0) = %q, want v0.1.0", prev)
	}

	// the first tag has no predecessor: empty, not an error.
	first, err := git.PreviousTag(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if first != "" {
		t.Errorf("PreviousTag(v0.1.0) = %q, want empty", first)
	}
}

func Test_Git_LatestTag(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: one")
	tag(t, dir, "v0.1.0")
	commit(t, dir, "feat: two")
	tag(t, dir, "v0.10.0")

	latest, err := NewGit(dir).LatestTag(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// version sort, not lexical: v0.10.0 outranks v0.1.0.
	if latest != "v0.10.0" {
		t.Errorf("LatestTag = %q, want v0.10.0", latest)
	}
}

func Test_Git_BranchName(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: one")
	run(t, dir, "git", "branch", "-M", "main")
	commit(t, dir, "feat: two")
	run(t, dir, "git", "switch", "-c", "feat/work", "-q")
	commit(t, dir, "feat: three")
	head := revParse(t, dir, "HEAD")

	tests := []struct {
		name  string
		setup func()
		ref   string
		want  string
	}{
		{name: "branch checkout names the branch", ref: "HEAD", want: "feat/work"},
		{name: "explicit branch ref", ref: "main", want: "main"},
		{
			name:  "detached HEAD has no branch to name",
			setup: func() { run(t, dir, "git", "checkout", "-q", "--detach", head) },
			ref:   "HEAD",
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			got, err := NewGit(dir).BranchName(context.Background(), tt.ref)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("BranchName(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}
